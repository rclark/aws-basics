package invocation

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-app/secrets"
	"golang.org/x/sync/errgroup"
)

//go:generate mockgen -source ./handler.go -package mock -destination ./mock/handler.go

// Requester implements the http.DefaultClient's method to run a request.
type Requester interface {
	Do(*http.Request) (*http.Response, error)
}

// Logger is used for testing that the function produces expected log outputs.
type Logger interface {
	Clear()
	Set(string, string)
	Print()
}

// SecretsReadWrite are the AWS SecretsManager methods for reading and updating
// secrets.
type SecretsReadWrite interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(context.Context, *secretsmanager.PutSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
}

// Handler is used to manage configurations for each Lambda invocation.
type Handler struct {
	Secrets   SecretsReadWrite
	Logger    Logger
	Requester Requester
}

// AppInfo represents the data about a GitHub app that's required in order to
// generate an API token representing the app.
type AppInfo struct {
	ID             string
	InstallationID string
	PEM            string
}

// Fetch gets the AppInfo data from AWS SecretsManager.
func (a *AppInfo) Fetch(ctx context.Context, sm SecretsReadWrite) error {
	g := new(errgroup.Group)
	g.Go(func() error {
		res, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secrets.AppID),
		})
		if err != nil {
			return errors.Wrap(err, "failed to retrieve app id")
		}
		a.ID = *res.SecretString
		return nil
	})

	g.Go(func() error {
		res, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secrets.InstallationID),
		})
		if err != nil {
			return errors.Wrap(err, "failed to retrieve app id")
		}
		a.InstallationID = *res.SecretString
		return nil
	})

	g.Go(func() error {
		res, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secrets.PEM),
		})
		if err != nil {
			return errors.Wrap(err, "failed to retrieve app id")
		}
		a.PEM = *res.SecretString
		return nil
	})

	return g.Wait()
}

// JWT uses AppInfo data to generate a JWT. This token can be exchanged with
// GitHub in order to recieve an API token.
func (a *AppInfo) JWT() (string, error) {
	now := time.Now().UTC()

	claims := jwt.StandardClaims{
		ExpiresAt: now.Add(10 * time.Minute).Unix(),
		IssuedAt:  now.Add(-1 * time.Minute).Unix(),
		Issuer:    a.ID,
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.PEM))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse app pem")
	}

	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
}

type response struct {
	Token string `json:"token"`
}

// Run is what each Lambda invocation does. The function fetches credentials for
// the GitHub app from AWS SecretsManager. It uses those credentials to generate
// a JWT according to GitHub's specifications (). It then provides that JWT to
// GitHub in a request for an API access token. Finally, it updates the app's
// token in AWS SecretsManager where other systems can access it.
//
// This Lambda function is intended to run every 10 minutes. The tokens
// it generates expire after 60 minutes. As a result, any application that
// accesses the app's token in AWS SecretsManager can expect to receive a token
// that will be valid for at least 50 minutes.
//
// If the Lambda function fails for any reason, it will be retried up to 2 more
// times by AWS. Logs for the Lambda function will only include information
// about errors that were encountered.
func (h *Handler) Run(ctx context.Context) (err error) {
	h.Logger.Clear()

	defer func() {
		if err != nil {
			h.Logger.Set("Error", fmt.Sprintf("%+v", err))
		}

		h.Logger.Print()
	}()

	info := new(AppInfo)
	if err := info.Fetch(ctx, h.Secrets); err != nil {
		return errors.Wrap(err, "failed to lookup app information in secrets manager")
	}

	jwt, err := info.JWT()
	if err != nil {
		return errors.Wrap(err, "failed to create jwt")
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", info.InstallationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Add("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", jwt))

	res, err := h.Requester.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed POST request for app token")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if res.StatusCode != 201 {
		h.Logger.Set("StatusCode", res.Status)
		h.Logger.Set("Response", string(body))
		return errors.New("unexpected api response")
	}

	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return errors.Wrap(err, "failed to parse response body")
	}

	_, err = h.Secrets.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secrets.Token),
		SecretString: &r.Token,
	})

	return errors.Wrap(err, "failed to update token in secrets manager")
}
