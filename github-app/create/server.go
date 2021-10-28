package create

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/rclark/aws-basics/github-app/secrets"
)

//go:generate mockgen -source ./server.go -package mock -destination ./mock/server.go
//go:generate mockgen -package mock -destination ./mock/http.go net/http ResponseWriter

// Requester implements the http.DefaultClient's method to run a request.
type Requester interface {
	Do(*http.Request) (*http.Response, error)
}

// SecretCreator implements a method for saving secrets in AWS SecretsManager.
type SecretCreator interface {
	CreateSecret(context.Context, *secretsmanager.CreateSecretInput, ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
}

// LocalhostServer runs a localhost website that helps automate the creation of
// a new GitHub App.
type LocalhostServer struct {
	http.Server
	Secrets   SecretCreator
	requester Requester
	open      func(string) error
	done      chan bool
	errors    chan error
}

// NewLocalhostServer sets up the localhost website.
func NewLocalhostServer(sm SecretCreator) *LocalhostServer {
	l := &LocalhostServer{
		Secrets:   sm,
		requester: http.DefaultClient,
		open:      browser.OpenURL,
		done:      make(chan bool),
		errors:    make(chan error),
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/", l.prompt)
	handler.HandleFunc("/redirect/", l.accept)

	l.Server = http.Server{
		Addr:    ":6060",
		Handler: handler,
	}

	return l
}

// CreateApp launches the localhost website, and opens it in the user's default
// web browser. The user is expected to submit the form, which will redirect to
// GitHub in order to create the aws-basics GitHub App. After the user has
// finished, the system receives the new GitHub App's credentials. The site
// prompts the user to record those credentials in their config.tfvars file.
func (l *LocalhostServer) CreateApp(ctx context.Context) (err error) {
	go l.listen()
	time.Sleep(5 * time.Millisecond)

	os.Setenv("LD_LIBRARY_PATH", "")
	if err := l.open("http://localhost:6060"); err != nil {
		return err
	}

	select {
	case <-l.done:
		err = nil
	case <-ctx.Done():
		err = errors.New("timed out attempting to create GitHub app")
	case lisErr := <-l.errors:
		err = lisErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l.Shutdown(ctx)

	return err
}

func (l *LocalhostServer) prompt(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `
	<html>
		<head>
			<link rel="stylesheet" href="https://unpkg.com/purecss@2.0.6/build/pure-min.css" integrity="sha384-Uu6IeWbM+gzNVXJcM9XV3SohHtmWE+3VGi496jvgX1jyvDTXfdK+rfZc8C1Aehk5" crossorigin="anonymous">
		</head>

		<div class="pure-g">
			<div class="pure-u-1-4"></div>
			<div class="pure-u-1-2">
				<br>
				<form class="pure-form pure-form-aligned">
					<legend>Create the aws-basics GitHub App</legend>
					<div class="pure-control-group">
						<label for="webhook">Your webhook URL</label>
						<input class="pure-input-2-3" type="text" id="webhook" placeholder="https://xxxxxxxxxx.execute-api.us-west-2.amazonaws.com">
					</div>
				</form>
				<form class="pure-form" action="https://github.com/settings/apps/new" method="post">
					<input type="text" name="manifest" id="manifest" hidden><br>
					<input class="pure-button pure-button-primary" type="submit" value="Go">
				</form>
			</div>
			<div class="pure-u-1-4"></div>
		</div>

		<script>
			const input = document.getElementById("manifest")
			const webhook = document.getElementById("webhook")

			webhook.addEventListener('input', (e) => {
				input.value = JSON.stringify({
					"name": "aws-basics",
					"url": "https://github.com/rclark/aws-basics",
					"redirect_url": "http://localhost:6060/redirect/",
					"hook_attributes": {
						"url": e.target.value
					},
					"public": false,
					"default_permissions": {
						"contents": "read"
					},
					"default_events": [
						"push"
					]
				})
			})
		</script>
	</html>
	`)
}

type response struct {
	ID            int    `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
	PEM           string `json:"pem"`
}

func (r response) Save(ctx context.Context, sm SecretCreator) error {
	g := new(errgroup.Group)
	g.Go(func() error {
		_, err := sm.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secrets.AppID),
			Description:  aws.String("The app's id"),
			SecretString: aws.String(fmt.Sprint(r.ID)),
		})
		return errors.Wrap(err, "failed writing app id to secrets manager")
	})

	g.Go(func() error {
		_, err := sm.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secrets.ClientID),
			Description:  aws.String("The app's client id"),
			SecretString: aws.String(r.ClientID),
		})
		return errors.Wrap(err, "failed writing client id to secrets manager")
	})

	g.Go(func() error {
		_, err := sm.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secrets.ClientSecret),
			Description:  aws.String("The app's client secret"),
			SecretString: aws.String(r.ClientSecret),
		})
		return errors.Wrap(err, "failed writing client secret to secrets manager")
	})

	g.Go(func() error {
		_, err := sm.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secrets.WebhookSecret),
			Description:  aws.String("The app's webhook secret"),
			SecretString: aws.String(r.WebhookSecret),
		})
		return errors.Wrap(err, "failed writing webhook secret to secrets manager")
	})

	g.Go(func() error {
		_, err := sm.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secrets.PEM),
			Description:  aws.String("The app's pem"),
			SecretString: aws.String(r.PEM),
		})
		return errors.Wrap(err, "failed writing pem to secrets manager")
	})

	g.Go(func() error {
		_, err := sm.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secrets.Token),
			Description:  aws.String("The app's token"),
			SecretString: aws.String("null"),
		})
		return errors.Wrap(err, "failed to create token in secrets manager")
	})

	return g.Wait()
}

func (l *LocalhostServer) accept(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	code := values.Get("code")
	if code == "" {
		l.errors <- errors.New("no code in response")
		fmt.Fprintf(w, "Failed! For more details, see your terminal. You can close this browser window.")
		return
	}

	url := fmt.Sprintf("https://api.github.com/app-manifests/%s/conversions", code)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		fmt.Fprintf(w, "Failed! For more details, see your terminal. You can close this browser window.")
		l.errors <- errors.Wrap(err, "failed to create POST request")
		return
	}
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	res, err := l.requester.Do(req)
	if err != nil {
		fmt.Fprintf(w, "Failed! For more details, see your terminal. You can close this browser window.")
		l.errors <- errors.Wrap(err, "failed to send POST request")
		return
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Fprintf(w, "Failed! For more details, see your terminal. You can close this browser window.")
		l.errors <- errors.Wrap(err, "failed to read response")
		return
	}

	var info response
	if err := json.Unmarshal(body, &info); err != nil {
		fmt.Fprintf(w, "Failed! For more details, see your terminal. You can close this browser window.")
		l.errors <- errors.Wrap(err, "failed to parse response")
		return
	}

	if err := info.Save(r.Context(), l.Secrets); err != nil {
		fmt.Fprintf(w, "Failed! For more details, see your terminal. You can close this browser window.")
		l.errors <- errors.Wrap(err, "failed to save app credentials")
		return
	}

	fmt.Fprintf(w, "Success! You can close this browser window.")

	l.done <- true
}

func (l *LocalhostServer) listen() {
	if err := l.ListenAndServe(); err != http.ErrServerClosed {
		l.errors <- err
	}
}
