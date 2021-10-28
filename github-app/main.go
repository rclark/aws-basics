package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-app/create"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(errors.Wrap(err, "could not acquire AWS credentials"))
	}
	sm := secretsmanager.NewFromConfig(cfg)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := create.NewLocalhostServer(sm).CreateApp(ctx); err != nil {
		log.Fatal(err)
	}
}
