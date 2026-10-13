package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"
)

type bucketStats struct {
	objectsCount int
	objectsSize  int64
	lastModified time.Time
}

// newAWSConfig builds the AWS configuration used to scrape the given exporter's bucket.
//
// When the profile defines both static keys, they are used. Otherwise, the AWS SDK default
// credentials chain is kept (environment variables, IRSA web identity, EKS Pod Identity,
// instance metadata...), so the exporter can run without any long-lived secret.
func newAWSConfig(ctx context.Context, profile Profile, exporter Exporter) (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	if profile.AwsAccessKeyID != "" && profile.AwsSecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentialsProvider(
			profile.AwsAccessKeyID,
			profile.AwsSecretAccessKey,
			"",
		)
	}

	if len(profile.AwsEndpoint) > 0 {
		awsConfig.BaseEndpoint = aws.String(profile.AwsEndpoint)
	}

	if len(exporter.AwsRegion) > 0 {
		awsConfig.Region = exporter.AwsRegion
	} else if len(profile.AwsRegion) > 0 {
		awsConfig.Region = profile.AwsRegion
	}

	return awsConfig, nil
}

// listBucket walks every object of the exporter's bucket/prefix and aggregates its stats.
//
// It stops at the first failed page: the paginator does not advance on error, so retrying
// would request the same page forever and block the collection of every following bucket.
func listBucket(ctx context.Context, client s3.ListObjectsV2APIClient, exporter Exporter) (bucketStats, error) {
	var stats bucketStats

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: &exporter.Bucket,
		Prefix: &exporter.Prefix,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)

		s3TotalRequestCount.With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).Inc()

		if err != nil {
			return bucketStats{}, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, item := range page.Contents {
			stats.objectsCount++

			if item.Size == nil {
				continue
			}

			stats.objectsSize += *item.Size
			if item.LastModified == nil {
				continue
			}

			if item.LastModified.After(stats.lastModified) {
				stats.lastModified = *item.LastModified
			}
		}
	}

	return stats, nil
}

func (s3Cfg S3Config) Collect() {
	slog.Info("Collecting S3 Metrics in Progress")

	ctx := context.Background()

	for _, exporter := range s3Cfg.Exporters {
		profile := s3Cfg.Profiles[exporter.Profile]

		awsConfig, err := newAWSConfig(ctx, profile, exporter)
		if err != nil {
			errorf("Failed to build AWS configuration on exporter %s/%s: %v",
				exporter.Bucket, exporter.Prefix, err)

			continue
		}

		infof("S3 Bucket is %s/%s", exporter.Bucket, exporter.Prefix)

		svc := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
			o.UsePathStyle = profile.UsePathStyle
		})

		// On failure, keep the previously exported values instead of publishing partial stats:
		// the backup age alerts then keep firing on stale data rather than resetting.
		stats, err := listBucket(ctx, svc, exporter)
		if err != nil {
			errorf("Failed to collect metrics for %s/%s: %v", exporter.Bucket, exporter.Prefix, err)

			continue
		}

		if len(exporter.MetricsToCollect) == 0 {
			exporter.MetricsToCollect = allMetricsToCollect
		}

		for _, metric := range exporter.MetricsToCollect {
			switch metric {
			case OldestFileDate.String():
				if stats.objectsCount > 0 {
					lastModifiedMetrics := float64(stats.lastModified.UnixNano() / int64(time.Second))
					s3LatestPosixTimestampMetrics.
						With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).
						Set(lastModifiedMetrics)
					infof(
						"Final lastModified for %s/%s is : %v -> (%v seconds since January 1, 1970 UTC)",
						exporter.Bucket, exporter.Prefix, stats.lastModified, lastModifiedMetrics)
				}
			case BucketCount.String():
				bucketObjectsCountTotalMetrics := float64(stats.objectsCount)
				s3BucketObjectsMetrics.
					With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).
					Set(bucketObjectsCountTotalMetrics)
				infof("S3 Bucket Total Count is %v for %s/%s",
					bucketObjectsCountTotalMetrics, exporter.Bucket, exporter.Prefix)
			case BucketSize.String():
				bucketObjectsSizeTotalMetrics := float64(stats.objectsSize) / 1e9
				s3BucketObjectsSizeMetrics.
					With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).
					Set(bucketObjectsSizeTotalMetrics)
				infof("S3 Bucket Total Size is %v giga octets for %s/%s",
					bucketObjectsSizeTotalMetrics, exporter.Bucket, exporter.Prefix)
			default:
				warnf("metrics %s isn't recognized", metric)
			}
		}
	}

	slog.Info("End of Collection")
}
