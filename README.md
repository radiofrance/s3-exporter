# S3 Exporter

__Situation:__ Collecting metrics from S3 Buckets in Prometheus.

__Solution:__ An exporter that can report for any needed metrics from S3 Buckets such as the last modified date for objects in a bucket that match a given prefix for instance.

## AWS credentials

Each bucket references a `profile` from the configuration file:

- if the profile defines both `access_key_id` and `secret_access_key`, these static keys are used;
- otherwise, the [AWS SDK for Go v2 default credentials chain](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-credentials)
  is used: `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` environment variables, shared config files,
  IAM Roles for Service Accounts (IRSA), EKS Pod Identity, EC2 instance role...

The region is taken from the bucket's `region`, then the profile's `region`, then the default chain
(`AWS_REGION`...).

The only S3 call made is a paginated `ListObjectsV2`: the `s3:ListBucket` permission on each
monitored bucket is enough.

## Configuration

By default, the exporter reads its configuration from `/etc/config/s3-exporter/s3config.json` (this path
can be changed via `S3_EXPORTER_CONFIG_PATH` environment variable).

There is an example of this configuration file in `fixtures/sample.json`.

### Using an other provider than Amazon S3

If one of your tenants is not hosted on Amazon S3 (example if you use Garage, MinIO, etc),
you can specify a `endpoint` url in the configuration for your tenant.

You can also tell the exporter to use path-style if you want.
