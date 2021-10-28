# artifacts-bucket

An S3 bucket to be used for storing deployment artifacts.

## Component consists of

S3 buckets named `artifacts-${AWS::AccountId}-${AWS::Region}`. Buckets are named this way to avoid conflicts with other AWS Accounts. Buckets exist in each region primarily to support AWS Lambda, which requires deployment bundles to be located on S3 in the same region where the Lambda function runs.
