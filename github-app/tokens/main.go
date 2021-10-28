package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-app/tokens/invocation"
	"github.com/rclark/aws-basics/utils"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("%+v", errors.Wrap(err, "could not acquire AWS credentials"))
	}

	handler := &invocation.Handler{
		Secrets:   secretsmanager.NewFromConfig(cfg),
		Logger:    utils.Logger{},
		Requester: http.DefaultClient,
	}

	lambda.Start(handler.Run)
}
