package parse

import (
	"os"

	"github.com/zeebo/bencode"
)

type TorrentInfo struct {
	Info struct {
		Name  string `bencode:"name"`
		Files []struct {
			Path   []string `bencode:"path"`
			Length int64    `bencode:"length"`
		} `bencode:"files"`
	} `bencode:"info"`
}

func Bencode(torrentPath string) (*TorrentInfo, error) {
	f, err := os.Open(torrentPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var t TorrentInfo
	if err := bencode.NewDecoder(f).Decode(&t); err != nil {
		return nil, err
	}

	return &t, nil
}
