package ingest

import (
	"fmt"
	"os"

	"github.com/Gdani64/big-brother-media/internal/classify"
	"github.com/Gdani64/big-brother-media/internal/parse"
	"github.com/Gdani64/big-brother-media/internal/qbt"
)

type mediaClassifier interface {
	Query(input classify.Input) (classify.MediaType, error)
}

func NewFileJob(baseDiskPath string, torrentPath string, qbtClient qbt.Client, mc mediaClassifier) error {
	ti, err := parse.Bencode(torrentPath)
	if err != nil {
		return fmt.Errorf("bencode parsing error: %v", err)
	}

	files := make([]string, 0, len(ti.Info.Files))
	for _, file := range ti.Info.Files {
		files = append(files, file.Path...)
	}

	classifierInput := classify.Input{
		Name:      ti.Info.Name,
		FilePaths: files,
	}
	mediaType, err := mc.Query(classifierInput)
	if err != nil {
		return fmt.Errorf("gemini classifier query error: %v", err)
	}
	fmt.Printf("classified as: %s\n", mediaType)

	result, err := qbtClient.AddTorrent(torrentPath, qbt.WithSavePath(baseDiskPath+classify.MediaTypeToPath(mediaType)))
	if err != nil {
		return fmt.Errorf("torrent api error: %v", err)
	}
	fmt.Printf("add torrent result: %+v\n", result)

	err = os.Remove(torrentPath)
	if err != nil {
		return err
	}

	return nil
}
