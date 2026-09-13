package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Gdani64/big-brother-media/internal/classify"
	"github.com/Gdani64/big-brother-media/internal/dirwatcher"
	"github.com/Gdani64/big-brother-media/internal/ingest"
	"github.com/Gdani64/big-brother-media/internal/qbt"
)

func main() {
	ctx := context.Background()
	apiKey := "private_do_not_share_asdasdasdas12321123"
	//apiKey, exists := os.LookupEnv("QB_API_KEY")
	//if !exists {
	//	panic("QB_API_KEY env variable not set")
	//}

	qbtClient := qbt.NewClient(apiKey)

	geminiClassifier, err := classify.NewGemini()
	if err != nil {
		panic(fmt.Errorf("gemini classifier constructor error: %v", err))
	}

	paths, exists := os.LookupEnv("WATCHED_DIR_PATHS")
	if !exists {
		panic("WATCHED_DIR_PATHS env variable not set")
	}

	splitPaths := strings.Split(paths, ",")

	newFileEvents, err := dirwatcher.WatchForNewFiles(ctx, splitPaths...)
	if err != nil {
		panic(fmt.Errorf("error from dir watcher: %v", err))
	}

	// pre go 1.22 version, all goroutines would receive the same f unless shadowed and reassigned with f := f
	for f := range newFileEvents {
		fmt.Printf("new file %s\n", f.Name)
		go func() {
			err := ingest.NewFileJob(f.Name, qbtClient, geminiClassifier)
			if err != nil {
				fmt.Println(err.Error())
			}
		}()
	}

	os.Exit(0)
}
