package github

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-app/secrets"
	"github.com/rclark/aws-basics/toolkit/src/command"
)

type SecretReader interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type Client struct {
	sm    SecretReader
	cmd   func(context.Context, *command.Process) error
	token string
}

func NewClient() (*Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "could not acquire AWS credentials")
	}

	return &Client{
		sm:  secretsmanager.NewFromConfig(cfg),
		cmd: command.Run,
	}, nil
}

func (c *Client) getToken(ctx context.Context) error {
	res, err := c.sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secrets.Token),
	})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve token from AWS Secrets Manager")
	}

	c.token = *res.SecretString
	return nil
}

func (c *Client) Clone(ctx context.Context, repo string, commit string) (string, error) {
	if c.token == "" {
		if err := c.getToken(ctx); err != nil {
			return "", errors.Wrap(err, "could not acquire github access token")
		}
	}

	dir, err := ioutil.TempDir("", strings.Replace(repo, "/", "-", -1))
	if err != nil {
		return "", errors.Wrap(err, "could not create temporary directory")
	}

	if err := c.cmd(ctx, &command.Process{
		WorkingDirectory: dir,
		Command:          "git",
		Arguments:        []string{"init"},
	}); err != nil {
		return "", errors.Wrap(err, "exec failure")
	}

	origin := fmt.Sprintf("https://x-access-token:${GITHUB_ACCESS_TOKEN}@github.com/%s", repo)
	if err := c.cmd(ctx, &command.Process{
		WorkingDirectory:     dir,
		EnvironmentVariables: []string{fmt.Sprintf("GITHUB_ACCESS_TOKEN=%s", c.token)},
		Command:              "git",
		Arguments:            []string{"remote", "add", "origin", origin},
	}); err != nil {
		return "", errors.Wrap(err, "exec failure")
	}

	if err := c.cmd(ctx, &command.Process{
		WorkingDirectory: dir,
		Command:          "git",
		Arguments:        []string{"fetch", "origin", commit, "--depth=1"},
	}); err != nil {
		return "", errors.Wrap(err, "exec failure")
	}

	if err := c.cmd(ctx, &command.Process{
		WorkingDirectory: dir,
		Command:          "git",
		Arguments:        []string{"reset", "--hard", "FETCH_HEAD"},
	}); err != nil {
		return "", errors.Wrap(err, "exec failure")
	}

	return dir, nil
}
