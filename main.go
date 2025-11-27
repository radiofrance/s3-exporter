package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	rMetrics := mux.NewRouter()
	rMetrics.Handle("/metrics", promhttp.Handler())

	srvMetrics := &http.Server{
		Addr:         ":9145",
		Handler:      http.TimeoutHandler(rMetrics, 10*time.Second, "Server Timeout"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		defer waitGroup.Done()

		err := srvMetrics.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				slog.Info("Metrics server closed")
			} else {
				errorf("Failed to start metrics server: %v", err)
			}
		}
	}()

	// Collect S3 Metrics in a goroutine
	s3wCollectCtx, s3wCollectCancel := context.WithCancel(context.Background())

	go func() {
		defer waitGroup.Done()

		s3w, err := initS3Exporter()
		if err != nil {
			errorf("Failed to get exporters from json file: %v", err)
			return
		}

		tck := time.NewTicker(time.Duration(s3w.Settings.CheckInterval) * time.Second)
		defer tck.Stop()

		s3w.Collect() //nolint:contextcheck

		for {
			select {
			case <-tck.C:
				s3w.Collect() //nolint:contextcheck
			case <-s3wCollectCtx.Done():
				return
			}
		}
	}()

	// Graceful shutdown, inspired by https://github.com/gorilla/mux#graceful-shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	<-c
	s3wCollectCancel()

	_ = srvMetrics.Shutdown(context.Background())

	slog.Info("Shutting down")
	waitGroup.Wait()
	os.Exit(0)
}
