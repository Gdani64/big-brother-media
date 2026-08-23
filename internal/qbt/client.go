package qbt

import (
	"fmt"
	"net/http"
	"time"
)

const (
	BaseUrlV2 = "http://192.168.68.68:8095/api/v2"
)

type Client struct {
	BaseUrl    string
	apiKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string) Client {
	return Client{
		BaseUrl: BaseUrlV2,
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

type successMessage struct {
	Code int `json:"code"`
	Data any `json:"data"`
}

func (c *Client) sendRequest(req *http.Request, v any) error {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
	// 	errRes := errorResponse{
	// 		Code: res.StatusCode,
	// 		Message: res.Body,
	// 	}
	// }

	return nil
}
