package invocation

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt"
	"github.com/golang/mock/gomock"
	"github.com/rclark/aws-basics/github-app/secrets"
	"github.com/rclark/aws-basics/github-app/tokens/invocation/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := mock.NewMockSecretsReadWrite(ctrl)
	requester := mock.NewMockRequester(ctrl)
	logger := mock.NewMockLogger(ctrl)

	pem, err := os.ReadFile("test-key.pem")
	require.NoError(t, err, "failed to read test pem file")
	pub, err := os.ReadFile("test-pub.pem")
	public, err := jwt.ParseRSAPublicKeyFromPEM(pub)
	require.NoError(t, err, "failed to parse public key from test pem file")

	// We expect the logger to be cleared, and printed
	logger.EXPECT().Clear()
	logger.EXPECT().Print()

	// We expect credentials to be looked up in AWS SecretsManager
	sm.EXPECT().
		GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secrets.AppID),
		}).
		Return(&secretsmanager.GetSecretValueOutput{
			SecretString: aws.String("app-id"),
		}, nil)

	sm.EXPECT().
		GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secrets.InstallationID),
		}).
		Return(&secretsmanager.GetSecretValueOutput{
			SecretString: aws.String("installation-id"),
		}, nil)

	sm.EXPECT().
		GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secrets.PEM),
		}).
		Return(&secretsmanager.GetSecretValueOutput{
			SecretString: aws.String(string(pem)),
		}, nil)

	// We expect a POST request to be sent to GitHub.
	req, _ := http.NewRequest("POST", "", nil)
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Authorization", "")
	requester.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "POST", req.Method, "post request")
			assert.Equal(t, "api.github.com", req.URL.Host, "request to GitHub api")
			assert.Equal(t, "/app/installations/installation-id/access_tokens", req.URL.Path, "request for app installation token")
			assert.Equal(t, "application/vnd.github.v3+json", req.Header.Get("Accept"), "accept header")

			// We expect the JWT provided in the request to be created properly.
			token, err := jwt.Parse(req.Header.Get("Authorization"), func(token *jwt.Token) (interface{}, error) {
				sign, ok := token.Method.(*jwt.SigningMethodRSA)
				require.True(t, ok, "jwt should be RSA")
				require.Equal(t, "RS256", sign.Name, "jwt should use sha256")

				return public, nil
			})
			require.NoError(t, err, "failed to parse jwt")

			claims, ok := token.Claims.(jwt.MapClaims)
			require.True(t, ok, "contains claims")
			assert.Equal(t, "app-id", claims["iss"])

			now := time.Now().UTC()
			issued := time.Unix(int64(claims["iat"].(float64)), 0)
			expires := time.Unix(int64(claims["exp"].(float64)), 0)

			issuedRange := issued.Before(now.Add(-1*time.Minute)) && issued.After(now.Add(-2*time.Minute))
			assert.True(t, issuedRange, "issued claim in expected timeframe")
			expiresRange := expires.Before(now.Add(10*time.Minute)) && expires.After(now.Add(9*time.Minute))
			assert.True(t, expiresRange, "expires claim in expected timeframe")

			return &http.Response{
				Body: io.NopCloser(strings.NewReader(`{"token":"api-token"}`)),
			}, nil
		})

	// We expect the token to be stored in AWS SecretsManager.
	sm.EXPECT().PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secrets.Token),
		SecretString: aws.String("api-token"),
	})

	handler := &Handler{
		Secrets:   sm,
		Logger:    logger,
		Requester: requester,
	}

	err = handler.Run(ctx)
	require.NoError(t, err, "should not error")
}
