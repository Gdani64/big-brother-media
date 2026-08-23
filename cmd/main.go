package main

import (
	"fmt"
	"os"

	"github.com/Gdani64/big-brother-media/internal/qbt"
)

func main() {
	apiKey, exists := os.LookupEnv("QB_API_KEY")
	if !exists {
		fmt.Println("QB_API_KEY env variable not set")
		os.Exit(0)
	}

	qbtClient := qbt.NewClient(apiKey)
	apiVersion, err := qbtClient.Version()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Printf("qbt API version: %s", apiVersion)
	os.Exit(0)
}
