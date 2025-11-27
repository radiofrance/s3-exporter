package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"
)

func (s3Cfg S3Config) Collect() {
	slog.Info("Collecting S3 Metrics in Progress")

	for _, exporter := range s3Cfg.Exporters {
		creds := credentials.NewStaticCredentialsProvider(
			s3Cfg.Profiles[exporter.Profile].AwsAccessKeyID,
			s3Cfg.Profiles[exporter.Profile].AwsSecretAccessKey,
			"",
		)

		awsConfig, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			errorf("Failed to load AWS configuration on exporter %s/%s: %v",
				exporter.Bucket, exporter.Prefix, err)

			continue
		}

		awsConfig.Credentials = creds

		if len(exporter.AwsRegion) > 0 {
			awsConfig.Region = exporter.AwsRegion
		} else {
			awsConfig.Region = s3Cfg.Profiles[exporter.Profile].AwsRegion
		}

		infof("S3 Bucket is %s/%s", exporter.Bucket, exporter.Prefix)

		var (
			bucketObjectsCountTotal int
			bucketObjectsSizeTotal  int64
			lastModified            time.Time
		)

		svc := s3.NewFromConfig(awsConfig)
		query := &s3.ListObjectsV2Input{
			Bucket: &exporter.Bucket,
			Prefix: &exporter.Prefix,
		}

		paginateReq := s3.NewListObjectsV2Paginator(svc, query)
		for paginateReq.HasMorePages() {
			page, err := paginateReq.NextPage(context.Background())

			s3TotalRequestCount.With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).Inc()

			if err != nil {
				errorf(
					"Failed to call paginated request on s3 list Objects for %s/%s: %v",
					exporter.Bucket, exporter.Prefix, err)

				continue
			}

			for _, item := range page.Contents {
				bucketObjectsCountTotal++

				if item.Size == nil {
					continue
				}

				bucketObjectsSizeTotal += *item.Size
				if item.LastModified == nil {
					continue
				}

				if item.LastModified.After(lastModified) {
					lastModified = *item.LastModified
				}
			}
		}

		if len(exporter.MetricsToCollect) == 0 {
			exporter.MetricsToCollect = allMetricsToCollect
		}

		for _, metric := range exporter.MetricsToCollect {
			switch metric {
			case OldestFileDate.String():
				if bucketObjectsCountTotal > 0 {
					lastModifiedMetrics := float64(lastModified.UnixNano() / int64(time.Second))
					s3LatestPosixTimestampMetrics.
						With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).
						Set(lastModifiedMetrics)
					infof(
						"Final lastModified for %s/%s is : %v -> (%v seconds since January 1, 1970 UTC)",
						exporter.Bucket, exporter.Prefix, lastModified, lastModifiedMetrics)
				}
			case BucketCount.String():
				bucketObjectsCountTotalMetrics := float64(bucketObjectsCountTotal)
				s3BucketObjectsMetrics.
					With(prometheus.Labels{"bucket": exporter.Bucket, "prefix": exporter.Prefix}).
					Set(bucketObjectsCountTotalMetrics)
				infof("S3 Bucket Total Count is %v for %s/%s",
					bucketObjectsCountTotalMetrics, exporter.Bucket, exporter.Prefix)
			case BucketSize.String():
				bucketObjectsSizeTotalMetrics := float64(bucketObjectsSizeTotal) / 1e9
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
