package main

import (
	"fmt"
	"os"

	"github.com/Gdani64/big-brother-media/internal/classify"
	"github.com/Gdani64/big-brother-media/internal/parse"
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
	fmt.Printf("qbt API version: %s\n", apiVersion)

	// torrentsInfo, err := qbtClient.TorrentsInfo()
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	os.Exit(1)
	// }

	// fmt.Printf("torrents info: %v\n", torrentsInfo)

	torrentPath := "/home/danielguglea/DEV/torrents/35David_Sedaris___Me_Talk_Pretty_One_Day.torrent"

	// result, err := qbtClient.AddTorrent(torrentPath)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	os.Exit(1)
	// }
	// fmt.Printf("add torrent result: %+v\n", result)

	ti, err := parse.Bencode(torrentPath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	files := make([]string, 0, len(ti.Info.Files))
	for _, file := range ti.Info.Files {
		files = append(files, file.Path...)
	}

	geminiClassifier, err := classify.NewGemini()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	mediaType, err := geminiClassifier.Query(ti.Info.Name, files)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Printf("classified as: %s\n", mediaType)

	os.Exit(0)
}
