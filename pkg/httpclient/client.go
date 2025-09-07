package httpclient

import (
	"io"
	"net/http"
	"time"
)

type HttpClient struct {
	client *http.Client
}

func NewHttpClient(timeout time.Duration) *HttpClient {
	return &HttpClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (h *HttpClient) Get(url string) (*http.Response, error) {
	return h.client.Get(url)
}

func (h *HttpClient) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	return h.client.Post(url, contentType, body)
}
