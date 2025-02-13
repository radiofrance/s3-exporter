# S3 Exporter

__Situation:__ Collecting metrics from S3 Buckets in Prometheus.

__Solution:__ An exporter that can report for any needed metrics from S3 Buckets such as the last modified date for objects in a bucket that match a given prefix for instance.

__Export Variable:__ To make it works successfully, you have to export these environment variable before :
- AWS_REGION=eu-west-1
- AWS_ACCESS_KEY_ID= ...
- AWS_SECRET_ACCESS_KEY= ...

see the AWS SDK FOR GO DOC FILE : https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html
