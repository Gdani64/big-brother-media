package qbt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	BaseUrlEnv = "SYNO_IP"
	BasePathV2 = "/api/v2"
)

type Client struct {
	BaseUrl    string
	apiKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string) Client {
	baseUrl, exists := os.LookupEnv(BaseUrlEnv)
	if !exists {
		panic("SYNO_IP env variable not set")
	}

	return Client{
		BaseUrl: baseUrl + BasePathV2,
		apiKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// type successMessage struct {
// 	Code int `json:"code"`
// 	Data any `json:"data"`
// }

func (c *Client) sendRequest(req *http.Request, v any) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Accept", "application/json; charset=utf-8")

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)

		var errRes errorResponse
		if err := json.Unmarshal(body, &errRes); err == nil {
			return fmt.Errorf("qbt error (status %d): %s", res.StatusCode, errRes.Message)
		}

		return fmt.Errorf("qbt error (status %d): %s", res.StatusCode, string(body))
	}

	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		return err
	}

	return nil
}
