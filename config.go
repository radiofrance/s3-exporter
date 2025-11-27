package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kelseyhightower/envconfig"
)

type S3Params struct {
	ExportersJSONFilePath string `default:"/etc/config/s3-exporter/s3config.json" envconfig:"S3_EXPORTER_CONFIG_PATH"`
}

type S3Config struct {
	Exporters []Exporter         `json:"buckets"`
	Profiles  map[string]Profile `json:"profiles"`
	Settings  Settings           `json:"settings"`
}

type Settings struct {
	CheckInterval int64 `json:"check_interval" required:"true"`
}

type Profile struct {
	AwsAccessKeyID     string `json:"access_key_id"     required:"true"`
	AwsSecretAccessKey string `json:"secret_access_key" required:"true"`
	AwsRegion          string `json:"region"            required:"true"`
}

type Exporter struct {
	Bucket           string   `json:"name"    required:"true"`
	Prefix           string   `json:"prefix"`
	MetricsToCollect []string `json:"metrics"`
	Profile          string   `json:"profile" required:"true"`
	AwsRegion        string   `json:"region"`
}

func initS3Exporter() (*S3Config, error) {
	var s3Config S3Params

	err := envconfig.Process("", &s3Config)
	if err != nil {
		return nil, fmt.Errorf("unable to process s3 envconfig : %w", err)
	}

	jsonFile, err := os.Open(s3Config.ExportersJSONFilePath)

	defer func() {
		err := jsonFile.Close()
		if err != nil {
			errorf("can't close %s: %v", s3Config.ExportersJSONFilePath, err)
		}
	}()

	if err != nil {
		return nil, fmt.Errorf("failed to open json file: %w", err)
	}

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("error reading the body: %w", err)
	}

	var s3w S3Config

	err = json.Unmarshal(byteValue, &s3w)
	if err != nil {
		return nil, fmt.Errorf("error Unmarshalling the bytes to struct: %w", err)
	}

	return &s3w, nil
}
