package create

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang/mock/gomock"
	"github.com/rclark/aws-basics/github-app/create/mock"
	"github.com/rclark/aws-basics/github-app/secrets"
	"github.com/stretchr/testify/require"
)

func TestCreateAppSuccess(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := mock.NewMockSecretCreator(ctrl)
	writer := mock.NewMockResponseWriter(ctrl)
	requester := mock.NewMockRequester(ctrl)

	server := &LocalhostServer{
		Server:    http.Server{Addr: ":6060"},
		Secrets:   sm,
		requester: requester,
		done:      make(chan bool),
		errors:    make(chan error),
	}

	// In the test, instead of the browser being opened, we simulate the redirect
	// back to localhost:6060 that would be triggered by the user's interaction
	// with GitHub to create the app.
	server.open = func(s string) error {
		require.Equal(t, "http://localhost:6060", s, "opens user's browser to localhost url")

		u, _ := url.Parse("http://localhost:6060/redirect?code=the-code")

		go func() {
			server.accept(writer, &http.Request{
				Method: "GET",
				URL:    u,
			})
		}()

		return nil
	}

	// We expect a POST to be made to GitHub to get the app's credentials.
	req, _ := http.NewRequest("POST", "https://api.github.com/app-manifests/the-code/conversions", nil)
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	res := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{
			"id": 101,
			"slug": "aws-basics",
			"name": "aws-basics",
			"client_id": "client-id",
			"client_secret": "client-secret",
			"webhook_secret": "webhook-secret",
			"pem": "pem"
		}`)),
	}
	requester.EXPECT().Do(req).Return(res, nil)

	// We expect secrets to be saved
	sm.EXPECT().CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secrets.AppID),
		Description:  aws.String("The app's id"),
		SecretString: aws.String("101"),
	})

	sm.EXPECT().CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secrets.ClientID),
		Description:  aws.String("The app's client id"),
		SecretString: aws.String("client-id"),
	})

	sm.EXPECT().CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secrets.ClientSecret),
		Description:  aws.String("The app's client secret"),
		SecretString: aws.String("client-secret"),
	})

	sm.EXPECT().CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secrets.WebhookSecret),
		Description:  aws.String("The app's webhook secret"),
		SecretString: aws.String("webhook-secret"),
	})

	sm.EXPECT().CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secrets.PEM),
		Description:  aws.String("The app's pem"),
		SecretString: aws.String("pem"),
	})

	sm.EXPECT().CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secrets.Token),
		Description:  aws.String("The app's token"),
		SecretString: aws.String("null"),
	})

	// We expect a success message to be shown in the browser.
	writer.EXPECT().Write([]byte("Success! You can close this browser window."))

	err := server.CreateApp(ctx)
	require.NoError(t, err, "should not error")
}
