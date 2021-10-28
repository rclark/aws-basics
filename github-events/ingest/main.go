package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-events/ingest/invocation"
	"github.com/rclark/aws-basics/utils"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("%+v", errors.Wrap(err, "could not acquire AWS credentials"))
	}

	handler := &invocation.Handler{
		Secret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		Bus:    os.Getenv("GITHUB_EVENT_BUS_NAME"),
		Events: cloudwatchevents.NewFromConfig(cfg),
		Logger: utils.Logger{},
	}

	lambda.Start(handler.Run)
}
