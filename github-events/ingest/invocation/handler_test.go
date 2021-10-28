package invocation

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents/types"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rclark/aws-basics/github-events/ingest/invocation/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMissingDeliveryHeader(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: false,
		Body:            `{"not":"encoded"}`,
	}

	var logged string
	log.EXPECT().Clear()
	log.EXPECT().Set("Error", gomock.Any()).DoAndReturn(func(key string, val string) {
		logged = val
	})
	log.EXPECT().Print()

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 401, res.StatusCode, "should return 401")
	assert.True(t, strings.Contains(logged, "missing delivery header"), "expected log message")
}

func TestInvalidEventBody(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: true,
		Body:            `{"not":"encoded"}`,
		Headers:         map[string]string{"x-github-delivery": "1324d090-1319-4fe5-8a9f-32dd44b238fd"},
	}

	var logged string
	log.EXPECT().Clear()
	log.EXPECT().Set("Delivery", "1324d090-1319-4fe5-8a9f-32dd44b238fd")
	log.EXPECT().Set("Error", gomock.Any()).DoAndReturn(func(key string, val string) {
		logged = val
	})
	log.EXPECT().Print()

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 401, res.StatusCode, "should return 401")
	assert.True(t, strings.Contains(logged, "failed to decode request body"), "expected log message")
}

func TestMissingSignature(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: true,
		Body:            base64.RawStdEncoding.EncodeToString([]byte(`{"now":"encoded"}`)),
		Headers:         map[string]string{"x-github-delivery": "1324d090-1319-4fe5-8a9f-32dd44b238fd"},
	}

	var logged string
	log.EXPECT().Clear()
	log.EXPECT().Set("Delivery", "1324d090-1319-4fe5-8a9f-32dd44b238fd")
	log.EXPECT().Set("Error", gomock.Any()).DoAndReturn(func(key string, val string) {
		logged = val
	})
	log.EXPECT().Set("SignatureExpected", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Print()

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 401, res.StatusCode, "should return 401")
	assert.True(t, strings.Contains(logged, "no signature header"), "expected log message")
}

func TestMismatchedSignature(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: true,
		Body:            base64.RawStdEncoding.EncodeToString([]byte(`{"now":"encoded"}`)),
		Headers: map[string]string{
			"x-hub-signature-256": "sha256=from-github",
			"x-github-delivery":   "1324d090-1319-4fe5-8a9f-32dd44b238fd",
		},
	}

	var logged string
	log.EXPECT().Clear()
	log.EXPECT().Set("Delivery", "1324d090-1319-4fe5-8a9f-32dd44b238fd")
	log.EXPECT().Set("Error", gomock.Any()).DoAndReturn(func(key string, val string) {
		logged = val
	})
	log.EXPECT().Set("SignatureExpected", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Set("SignatureFound", "sha256=from-github")
	log.EXPECT().Print()

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 401, res.StatusCode, "should return 401")
	assert.True(t, strings.Contains(logged, "signature mismatch"), "expected log message")
}

func TestMissingEventTypeHeader(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: true,
		Body:            base64.RawStdEncoding.EncodeToString([]byte(`{"now":"encoded"}`)),
		Headers: map[string]string{
			"x-hub-signature-256": "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb",
			"x-github-delivery":   "1324d090-1319-4fe5-8a9f-32dd44b238fd",
		},
	}

	var logged string
	log.EXPECT().Clear()
	log.EXPECT().Set("Delivery", "1324d090-1319-4fe5-8a9f-32dd44b238fd")
	log.EXPECT().Set("Error", gomock.Any()).DoAndReturn(func(key string, val string) {
		logged = val
	})
	log.EXPECT().Set("SignatureExpected", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Set("SignatureFound", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Print()

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 401, res.StatusCode, "should return 401")
	assert.True(t, strings.Contains(logged, "missing event type header"), "expected log message")
}

func TestFailedPutEvents(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	body := `{"now":"encoded"}`
	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: true,
		Body:            base64.RawStdEncoding.EncodeToString([]byte(body)),
		Headers: map[string]string{
			"x-hub-signature-256": "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb",
			"x-github-event":      "Push",
			"x-github-delivery":   "1324d090-1319-4fe5-8a9f-32dd44b238fd",
		},
	}

	var logged string
	log.EXPECT().Clear()
	log.EXPECT().Set("Delivery", "1324d090-1319-4fe5-8a9f-32dd44b238fd")
	log.EXPECT().Set("Error", gomock.Any()).DoAndReturn(func(key string, val string) {
		logged = val
	})
	log.EXPECT().Set("SignatureExpected", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Set("SignatureFound", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Set("EventType", "push")
	log.EXPECT().Print()

	cw.EXPECT().PutEvents(ctx, &cloudwatchevents.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{{
			Detail:       aws.String(body),
			DetailType:   aws.String("push"),
			EventBusName: aws.String("github-events"),
			Source:       aws.String("github"),
		}},
	}).
		DoAndReturn(func(context.Context, *cloudwatchevents.PutEventsInput, ...func(*cloudwatchevents.Options)) (*cloudwatchevents.PutEventsOutput, error) {
			return nil, errors.New("api call failed")
		})

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 500, res.StatusCode, "should return 500")
	assert.True(t, strings.Contains(logged, "failed PutEvents API call"), "expected log message")
	assert.True(t, strings.Contains(logged, "api call failed"), "logs underlying API failure")
}

func TestSuccess(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cw := mock.NewMockCanPutEvents(ctrl)
	log := mock.NewMockLogger(ctrl)

	handler := Handler{
		Secret: "secret",
		Bus:    "github-events",
		Events: cw,
		Logger: log,
	}

	body := `{"now":"encoded"}`
	event := events.APIGatewayV2HTTPRequest{
		IsBase64Encoded: true,
		Body:            base64.RawStdEncoding.EncodeToString([]byte(body)),
		Headers: map[string]string{
			"x-hub-signature-256": "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb",
			"x-github-event":      "Push",
			"x-github-delivery":   "1324d090-1319-4fe5-8a9f-32dd44b238fd",
		},
	}

	log.EXPECT().Clear()
	log.EXPECT().Set("Delivery", "1324d090-1319-4fe5-8a9f-32dd44b238fd")
	log.EXPECT().Set("SignatureExpected", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Set("SignatureFound", "sha256=b4d09a57d222aeefc11428e84e7be1eb8868852805ceded48eb9749f5fd8b1bb")
	log.EXPECT().Set("EventType", "push")
	log.EXPECT().Print()

	cw.EXPECT().PutEvents(ctx, &cloudwatchevents.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{{
			Detail:       aws.String(body),
			DetailType:   aws.String("push"),
			EventBusName: aws.String("github-events"),
			Source:       aws.String("github"),
		}},
	})

	res, err := handler.Run(ctx, event)
	require.NoError(t, err, "should not error")
	assert.Equal(t, 201, res.StatusCode, "should return 201")
}
