package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

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

	// TESTING start
	// Flush buffered events before the program terminates.
	defer sentry.Flush(2 * time.Second)
	sentry.CaptureMessage("It works!")
	// TESTING end

	ctx := context.Background()

	baseDiskPath, exists := os.LookupEnv("DOWNLOAD_BASE_DISK_PATH")
	if !exists {
		log.Fatal("DOWNLOAD_BASE_DISK_PATH env variable not set")
	}

	apiKey, exists := os.LookupEnv("QB_API_KEY")
	if !exists {
		log.Fatal("QB_API_KEY env variable not set")
	}

	qbtClient := qbt.NewClient(apiKey)

	geminiClassifier, err := classify.NewGemini()
	if err != nil {
		log.Fatalf("gemini classifier constructor error: %v", err)
	}

	paths, exists := os.LookupEnv("WATCHED_DIR_PATHS")
	if !exists {
		log.Fatal("WATCHED_DIR_PATHS env variable not set")
	}

	splitPaths := strings.Split(paths, ",")

	newFileEvents, err := dirwatcher.WatchForNewFiles(ctx, splitPaths...)
	if err != nil {
		log.Fatalf("error from dir watcher: %v", err)
	}

	// pre go 1.22 version, all goroutines would receive the same f unless shadowed and reassigned with f := f
	for f := range newFileEvents {
		log.Printf("new file %s\n", f.Name)
		go func() {
			err := ingest.NewFileJob(baseDiskPath, f.Name, qbtClient, geminiClassifier)
			if err != nil {
				log.Println(err.Error())
			}
		}()
	}

	os.Exit(0)
}
