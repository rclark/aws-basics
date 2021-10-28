# github-app

A private GitHub app to setup and install in your GitHub account. This app is configured to send webhooks to your AWS account through the [github-events component](../github-events), and also provides your AWS account with permissions to interact with your GitHub account.

## Component consists of

- Included is a small tool to help you get started building your GitHub app. However, finalizing the app setup still requires you to do some manual work in GitHub's UI. More details coming soon.

- The app's credentials are stored in AWS Secrets Manager.

- A Lambda function is launched which runs once every 10 minutes. This function generates an API token representing your GitHub App, and stores it in AWS Secrets Manager. That token can be used throughout your AWS account in order to access content on GitHub.
