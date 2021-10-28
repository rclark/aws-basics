package invocation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents/types"
	"github.com/pkg/errors"
)

//go:generate mockgen -source ./handler.go -package mock -destination ./mock/handler.go

// Logger is used for testing that the function produces expected log outputs.
type Logger interface {
	Clear()
	Set(string, string)
	Print()
}

// CanPutEvents represents the CloudWatch Events PutEvents API method.
type CanPutEvents interface {
	PutEvents(ctx context.Context, params *cloudwatchevents.PutEventsInput, optFns ...func(*cloudwatchevents.Options)) (*cloudwatchevents.PutEventsOutput, error)
}

// Handler stores configuration that is reusable across Lambda function
// invocations.
type Handler struct {
	Secret string
	Bus    string
	Events CanPutEvents
	Logger Logger
}

// Run is the code to execute on each Lambda function invocation. The function
// receives an event representing a request to API gateway. It validates that
// the request came from GitHub, by verifying the signature provided in the
// X-Hub-Signature-256 header was produced using the shared secret that is
// configured for the system's GitHub App. See https://docs.github.com/en/developers/webhooks-and-events/webhooks/securing-your-webhooks#validating-payloads-from-github
// for more information about signature verification.
//
// If the signature is valid, the function produces a single CloudWatch Event
// representing the payload it received from GitHub. The function may result in
// the following HTTP response status codes:
//
// • 401: Invalid requests or signature mismatch.
//
// • 500: Failed to make the PutEvents API call.
//
// • 201: Success.
//
// Each time the Lambda invokes, a single, JSON-structured log entry is
// produced. The log entry will contain the following data about the request and
// its handling, unless the data is missing from the request:
//
// • Delivery: A GUID representing this event, which can be correlated to event
// logs in the GitHub App's UI.
//
// • SignatureExpected: The signature calculated by the Lambda invocation.
//
// • SignatureFound: The signature provided by the request's
// X-Hub-Signature-256 header.
//
// • EventType: The lower-cased name of the type of GitHub event this request
// represents, as provided in the request's X-GitHub-Event header.
//
// • Error: If there was a 401 or 500 response, this will provide a description
// of the failure that was encountered, and a stack trace in case debugging is
// neccessary.
func (h *Handler) Run(ctx context.Context, event events.APIGatewayV2HTTPRequest) (response events.APIGatewayV2HTTPResponse, err error) {
	h.Logger.Clear()

	defer func() {
		h.Logger.Print()
	}()

	response = events.APIGatewayV2HTTPResponse{StatusCode: 401}

	delivery, ok := event.Headers["x-github-delivery"]
	if !ok {
		h.Logger.Set("Error", fmt.Sprintf("%+v", errors.New("missing delivery header")))
		return response, nil
	}
	h.Logger.Set("Delivery", delivery)

	body := []byte(event.Body)
	if event.IsBase64Encoded {
		b, err := base64.RawStdEncoding.DecodeString(event.Body)
		if err != nil {
			h.Logger.Set("Error", fmt.Sprintf("%+v", errors.Wrap(err, "failed to decode request body")))
			return response, nil
		}
		body = b
	}

	hash := hmac.New(sha256.New, []byte(h.Secret))
	hash.Write(body)
	expected := fmt.Sprintf("sha256=%x", hash.Sum(nil))
	h.Logger.Set("SignatureExpected", expected)

	signature, ok := event.Headers["x-hub-signature-256"]
	if !ok {
		h.Logger.Set("Error", fmt.Sprintf("%+v", errors.New("no signature header")))
		return response, nil
	}
	h.Logger.Set("SignatureFound", signature)

	if signature != expected {
		h.Logger.Set("Error", fmt.Sprintf("%+v", errors.New("signature mismatch")))
		return response, nil
	}

	eventType, ok := event.Headers["x-github-event"]
	if !ok {
		h.Logger.Set("Error", fmt.Sprintf("%+v", errors.New("missing event type header")))
		return response, nil
	}
	eventType = strings.ToLower(eventType)
	h.Logger.Set("EventType", eventType)

	_, err = h.Events.PutEvents(ctx, &cloudwatchevents.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{{
			Detail:       aws.String(string(body)),
			DetailType:   aws.String(eventType),
			EventBusName: aws.String(h.Bus),
			Source:       aws.String("github"),
		}},
	})
	if err != nil {
		h.Logger.Set("Error", fmt.Sprintf("%+v", errors.Wrap(err, "failed PutEvents API call")))
		response.StatusCode = 500
		return response, nil
	}

	response.StatusCode = 201
	return response, nil
}
