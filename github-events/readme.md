# github-events

Creates EventBridge events in your AWS account representing developer activity in your GitHub repositories. Because EventBridge acts as a trigger for so many different actions in an AWS account, this is a critical integration point for any custom gitops-style workflows.

## Depends on

- [artifacts-bucket](../artifacts-bucket) in order to store Lambda bundles.
- [system-permissions](../system-permissions) provides an IAM role to use.
- [github-app](../github-app) which forwards events from GitHub to AWS.

## Component consists of

- API Gateway w/ POST endpoint
- Lambda function to verify signature and call PutEvents

This system need only be run once per AWS account, in a single region.

## Limitations

- Ideally we would use an API Gateway AWS integration to translate webhooks directly into EventBridge events. However, API Gateway's authorization Lambda functions do not receive the message body. This makes it impossible to verify the signatures that GitHub produces and provides with each request. As a result, the API Gateway invokes a Lambda function which both verifies the request's signature and forwards the GitHub event to EventBridge.

- GitHub event payloads can range in size up to 25MB. However, AWS API Gateway limits POST body size to 10MB, and AWS Lambda further limits invocation payload to 6MB. This means any event produced on GitHub that is larger than 6MB in size will not arrive in EventBridge. If the system were architected as an always-on container listening for POST request, it would incur a much higher cost on AWS. This limitation is considered acceptable in exchange for lower AWS bills!
