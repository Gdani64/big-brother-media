package qbt

import (
	"fmt"
	"io"
	"net/http"
)

func (c *Client) Version() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", c.BaseUrl, "/app/version"), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("version request failed: %s", string(body))
	}

	if err != nil {
		return "", err
	}

	return string(body), nil
}
