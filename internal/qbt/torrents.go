package qbt

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Torrent struct {
	Comment string `json:"comment"`
}

type TorrentsListResult []Torrent

type AddTorrentResult struct {
	AddedTorrentIDs []string `json:"added_torrent_ids"`
	FailureCount    int      `json:"failure_count"`
	PendingCount    int      `json:"pending_count"`
	SuccessCount    int      `json:"success_count"`
}

func (c *Client) TorrentsInfo() (*TorrentsListResult, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/torrents/info", c.BaseUrl), nil)
	if err != nil {
		return nil, err
	}

	var res TorrentsListResult
	err = c.sendRequest(req, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

type addTorrentParams struct {
	savePath string
}

type AddTorrentOption func(*addTorrentParams)

func WithSavePath(path string) AddTorrentOption {
	return func(p *addTorrentParams) {
		p.savePath = path
	}
}

func (c *Client) AddTorrent(torrentPath string, opts ...AddTorrentOption) (*AddTorrentResult, error) {
	params := &addTorrentParams{}
	for _, opt := range opts {
		opt(params)
	}

	file, err := os.Open(torrentPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("torrents", filepath.Base(torrentPath))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	if params.savePath != "" {
		if err := w.WriteField("savepath", params.savePath); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/torrents/add", c.BaseUrl), &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	var res AddTorrentResult
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}
