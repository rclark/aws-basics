package configuration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-app/secrets"
	"github.com/rclark/aws-basics/toolkit/src/command"
	"golang.org/x/sync/errgroup"
)

type SecretReader interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type IdentityGetter interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type Builder struct {
	sm               SecretReader
	sts              IdentityGetter
	run              func(context.Context, *command.Process) error
	pipe             func(context.Context, *command.Process, *command.Process) error
	githubToken      string
	awsCreds         aws.Credentials
	awsAccountID     string
	PrimaryAWSRegion string
}

func NewBuilder(ctx context.Context) (*Builder, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS configuration")
	}

	return &Builder{
		sm:               secretsmanager.NewFromConfig(cfg),
		sts:              sts.NewFromConfig(cfg),
		run:              command.Run,
		pipe:             command.Pipe,
		PrimaryAWSRegion: cfg.Region,
	}, nil
}

type BuildIdentification struct {
	Repository string // TODO: is this owner/repo or just repo?
	Commit     string
	Directory  string
}

func (b *Builder) DockerImage(ctx context.Context, id BuildIdentification, config *DockerImageConfig) error {
	if err := b.loadCreds(ctx); err != nil {
		return errors.Wrap(err, "failed to load external credentials")
	}

	if err := b.ecrLogin(ctx); err != nil {
		return errors.Wrap(err, "")
	}

	tag := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", b.awsAccountID, b.PrimaryAWSRegion, id.Repository)

	if err := b.run(ctx, &command.Process{
		WorkingDirectory: id.Directory,
		EnvironmentVariables: []string{
			fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", b.awsCreds.AccessKeyID),
			fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", b.awsCreds.SecretAccessKey),
			fmt.Sprintf("AWS_SESSION_TOKEN=%s", b.awsCreds.SessionToken),
			fmt.Sprintf("GITHUB_ACCESS_TOKEN=%s", b.githubToken),
		},
		Command: "docker",
		Arguments: []string{
			"build",
			"--build-arg", "AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}",
			"--build-arg", "AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}",
			"--build-arg", "AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN}",
			"--build-arg", "GITHUB_ACCESS_TOKEN=${GITHUB_ACCESS_TOKEN}",
			"--tag", tag,
			filepath.Join(id.Directory, config.DockerfilePath),
			filepath.Join(id.Directory, config.Context),
		},
	}); err != nil {
		return errors.Wrap(err, "docker build failed")
	}

	if err := b.run(ctx, &command.Process{
		WorkingDirectory: id.Directory,
		Command:          "docker",
		Arguments:        []string{"push", tag},
	}); err != nil {
		return errors.Wrap(err, "docker push failed")
	}

	return nil
}

func (b *Builder) LambdaBundle(ctx context.Context, id BuildIdentification, config *LambdaBundleConfig) error {
	if err := b.loadCreds(ctx); err != nil {
		return errors.Wrap(err, "failed to load external credentials")
	}

	split := strings.Split(config.BuildCommand, " ")
	p := &command.Process{
		WorkingDirectory: id.Directory,
		EnvironmentVariables: []string{
			fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", b.awsCreds.AccessKeyID),
			fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", b.awsCreds.SecretAccessKey),
			fmt.Sprintf("AWS_SESSION_TOKEN=%s", b.awsCreds.SessionToken),
			fmt.Sprintf("GITHUB_ACCESS_TOKEN=%s", b.githubToken),
		},
		Command:   split[0],
		Arguments: split[1:],
	}

	if err := b.run(ctx, p); err != nil {
		return errors.Wrapf(err, `failed to run "%s"`, config.BuildCommand)
	}

	zipfile := fmt.Sprintf("%s.zip", id.Commit)

	switch config.Runtime {
	case "go1.x":
		if err := b.run(ctx, &command.Process{
			WorkingDirectory: "dist",
			Command:          "zip",
			Arguments:        []string{fmt.Sprintf("../%s", zipfile), "*"},
		}); err != nil {
			return errors.Wrap(err, "failed to create zip archive")
		}

	case "nodejs14.x":
		args := []string{zipfile, "*"}

		if config.IncludePaths != nil {
			args = append(args, "-i")
			args = append(args, config.IncludePaths...)
			args = append(args, "node_modules")
		}

		if config.ExcludePaths != nil {
			args = append(args, "-x")
			args = append(args, config.ExcludePaths...)
		}

		if err := b.run(ctx, &command.Process{
			Command:   "zip",
			Arguments: args,
		}); err != nil {
			return errors.Wrap(err, "failed to create zip archive")
		}

	default:
		return errors.New(fmt.Sprintf("unknown runtime %s", config.Runtime))
	}

	dst := fmt.Sprintf(
		"s3://artifacts-%s-%s/%s/%s",
		b.awsAccountID,
		b.PrimaryAWSRegion,
		id.Repository,
		zipfile,
	)

	upload := &command.Process{
		Command:   "aws",
		Arguments: []string{"s3", "cp", zipfile, dst},
	}

	return errors.Wrap(b.run(ctx, upload), "failed upload to S3")
}

func (b *Builder) BuildAll(ctx context.Context, id BuildIdentification, builds *Builds) error {
	for i, config := range builds.LambdaBundles {
		if err := b.LambdaBundle(ctx, id, config); err != nil {
			return errors.Wrapf(err, "build failed for lambda bundle %v", i)
		}
	}

	for i, config := range builds.DockerImages {
		if err := b.DockerImage(ctx, id, config); err != nil {
			return errors.Wrapf(err, "build failed for lambda bundle %v", i)
		}
	}

	return nil
}

func (b *Builder) loadCreds(ctx context.Context) error {
	g := new(errgroup.Group)

	g.Go(func() error {
		return b.loadGitHubCreds(ctx)
	})

	g.Go(func() error {
		return b.loadAWSCreds(ctx)
	})

	g.Go(func() error {
		return b.lookupAWSCaller(ctx)
	})

	return g.Wait()
}

func (b *Builder) ecrLogin(ctx context.Context) error {
	src := &command.Process{
		Command:   "aws",
		Arguments: []string{"ecr", "get-login-password"},
	}

	dst := &command.Process{
		Command: "docker",
		Arguments: []string{
			"login",
			"--username", "AWS",
			"--password-stdin",
			fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", b.awsAccountID, b.PrimaryAWSRegion),
		},
	}

	return errors.Wrap(b.pipe(ctx, src, dst), "failed to log into ecr")
}

func (b *Builder) loadGitHubCreds(ctx context.Context) error {
	res, err := b.sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secrets.Token),
	})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve token from AWS Secrets Manager")
	}

	b.githubToken = *res.SecretString
	return nil
}

func (b *Builder) loadAWSCreds(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to load AWS configuration")
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to acquire AWS credentials")
	}

	b.awsCreds = creds
	return nil
}

func (b *Builder) lookupAWSCaller(ctx context.Context) error {
	res, err := b.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return errors.Wrap(err, "failed to get AWS identity")
	}

	b.awsAccountID = *res.Account
	return nil
}
