package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/Gdani64/big-brother-media/internal/classify"
	"github.com/Gdani64/big-brother-media/internal/dirwatcher"
	"github.com/Gdani64/big-brother-media/internal/ingest"
	"github.com/Gdani64/big-brother-media/internal/qbt"
	"github.com/getsentry/sentry-go"
)

func main() {
	sentryDSN, exists := os.LookupEnv("SENTRY_DSN")
	if !exists {
		log.Fatal("SENTRY_DSN env variable not set")
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:   sentryDSN,
		Debug: true,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}

	ctx := context.Background()
	logger := sentry.NewLogger(ctx)

	baseDiskPath, exists := os.LookupEnv("DOWNLOAD_BASE_DISK_PATH")
	if !exists {
		logger.Fatal().Emit("DOWNLOAD_BASE_DISK_PATH env variable not set")
	}

	apiKey, exists := os.LookupEnv("QB_API_KEY")
	if !exists {
		logger.Fatal().Emit("QB_API_KEY env variable not set")
	}

	qbtClient := qbt.NewClient(apiKey)

	geminiClassifier, err := classify.NewGemini()
	if err != nil {
		logger.Fatal().String("error", err.Error()).Emit("gemini classifier constructor error")
	}

	paths, exists := os.LookupEnv("WATCHED_DIR_PATHS")
	if !exists {
		logger.Fatal().Emit("WATCHED_DIR_PATHS env variable not set")
	}

	splitPaths := strings.Split(paths, ",")

	newFileEvents, err := dirwatcher.WatchForNewFiles(ctx, splitPaths...)
	if err != nil {
		logger.Fatal().String("error", err.Error()).Emit("error from dir watcher")
	}

	// pre go 1.22 version, all goroutines would receive the same f unless shadowed and reassigned with f := f
	for f := range newFileEvents {
		logger.Info().String("file", f.Name).Emit("new file")
		go func() {
			err := ingest.NewFileJob(baseDiskPath, f.Name, qbtClient, geminiClassifier)
			if err != nil {
				logger.Error().Emit(err.Error())
			}
		}()
	}

	os.Exit(0)
}
