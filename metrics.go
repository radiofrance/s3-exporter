package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	allMetricsToCollect = []string{"latest_file_posix_timestamp", "bucket_objects", "bucket_objects_total_size_go"}
	metricsLabels       = []string{"bucket", "prefix"}
	metricsNamespace    = "s3"
	metricsSubsystem    = "exporter"
)

type MetricsType int

const (
	OldestFileDate MetricsType = iota
	BucketCount
	BucketSize
)

func (m MetricsType) String() string {
	return allMetricsToCollect[m]
}

var (
	s3LatestPosixTimestampMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      OldestFileDate.String(),
		Help:      "Time in seconds since January 1, 1970 UTC",
	}, metricsLabels)

	s3BucketObjectsMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      BucketCount.String(),
		Help:      "Total Number of Bucket for bucket/prefix",
	}, metricsLabels)

	s3BucketObjectsSizeMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      BucketSize.String(),
		Help:      "Total Size of Bucket for bucket/prefix",
	}, metricsLabels)

	s3TotalRequestCount = promauto.NewCounterVec(prometheus.CounterOpts{ //nolint:promlinter
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "request_count",
		Help:      "Total number of request to S3",
	}, metricsLabels)
)
