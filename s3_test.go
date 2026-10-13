package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	profileKeyID = "PROFILEKEY"
	envKeyID     = "ENVKEY"
	euWest1      = "eu-west-1"
	euWest3      = "eu-west-3"
	usEast1      = "us-east-1"
)

// isolateAWSEnv makes the AWS SDK default chain only see the env vars set by the test.
func isolateAWSEnv(t *testing.T) {
	t.Helper()

	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("AWS_CONFIG_FILE", missing)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", missing)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_ROLE_ARN", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
}

func TestNewAWSConfigCredentials(t *testing.T) {
	tests := []struct {
		name      string
		profile   Profile
		envKeyID  string
		wantKeyID string
	}{
		{
			name:      "static keys from profile take precedence",
			profile:   Profile{AwsAccessKeyID: profileKeyID, AwsSecretAccessKey: "profile-secret"},
			envKeyID:  envKeyID,
			wantKeyID: profileKeyID,
		},
		{
			name:      "no keys in profile falls back to default chain",
			profile:   Profile{},
			envKeyID:  envKeyID,
			wantKeyID: envKeyID,
		},
		{
			name:      "partial keys in profile falls back to default chain",
			profile:   Profile{AwsAccessKeyID: profileKeyID},
			envKeyID:  envKeyID,
			wantKeyID: envKeyID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_ACCESS_KEY_ID", tt.envKeyID)
			t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")

			cfg, err := newAWSConfig(context.Background(), tt.profile, Exporter{})
			if err != nil {
				t.Fatalf("newAWSConfig() error = %v", err)
			}

			creds, err := cfg.Credentials.Retrieve(context.Background())
			if err != nil {
				t.Fatalf("Credentials.Retrieve() error = %v", err)
			}

			if creds.AccessKeyID != tt.wantKeyID {
				t.Errorf("AccessKeyID = %q, want %q", creds.AccessKeyID, tt.wantKeyID)
			}
		})
	}
}

func TestNewAWSConfigRegion(t *testing.T) {
	tests := []struct {
		name       string
		profile    Profile
		exporter   Exporter
		envRegion  string
		wantRegion string
	}{
		{"bucket region wins", Profile{AwsRegion: euWest1}, Exporter{AwsRegion: euWest3}, usEast1, euWest3},
		{"profile region next", Profile{AwsRegion: euWest1}, Exporter{}, usEast1, euWest1},
		{"default chain region last", Profile{}, Exporter{}, usEast1, usEast1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_REGION", tt.envRegion)

			cfg, err := newAWSConfig(context.Background(), tt.profile, tt.exporter)
			if err != nil {
				t.Fatalf("newAWSConfig() error = %v", err)
			}

			if cfg.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", cfg.Region, tt.wantRegion)
			}
		})
	}
}

type fakeListClient struct {
	pages []*s3.ListObjectsV2Output
	err   error
	calls int
}

func (f *fakeListClient) ListObjectsV2(
	_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}

	return f.pages[f.calls-1], nil
}

func TestListBucket(t *testing.T) {
	t.Parallel()

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)

	client := &fakeListClient{pages: []*s3.ListObjectsV2Output{
		{
			Contents:              []types.Object{{Size: aws.Int64(10), LastModified: &older}},
			IsTruncated:           aws.Bool(true),
			NextContinuationToken: aws.String("next"),
		},
		{
			Contents: []types.Object{{Size: aws.Int64(5), LastModified: &newer}, {Size: aws.Int64(1)}},
		},
	}}

	stats, err := listBucket(context.Background(), client, Exporter{Bucket: "bucket"})
	if err != nil {
		t.Fatalf("listBucket() error = %v", err)
	}

	want := bucketStats{objectsCount: 3, objectsSize: 16, lastModified: newer}
	if stats != want {
		t.Errorf("listBucket() = %+v, want %+v", stats, want)
	}
}

func TestListBucketStopsOnError(t *testing.T) {
	t.Parallel()

	errDenied := errors.New("access denied")
	client := &fakeListClient{err: errDenied}

	done := make(chan error, 1)

	go func() {
		_, err := listBucket(context.Background(), client, Exporter{Bucket: "bucket"})
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, errDenied) {
			t.Errorf("listBucket() error = %v, want %v", err, errDenied)
		}

		if client.calls != 1 {
			t.Errorf("ListObjectsV2 called %d times, want 1", client.calls)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listBucket() did not return on a persistent error")
	}
}
