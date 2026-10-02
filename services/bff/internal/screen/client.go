package screen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DownstreamError struct {
	Service string
	Status  int
	Err     error
}

func (e *DownstreamError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Service, e.Err)
	}
	return fmt.Sprintf("%s: ответил %d", e.Service, e.Status)
}

func (e *DownstreamError) Unwrap() error { return e.Err }

type client struct {
	name    string
	baseURL string
	http    *http.Client
}

func newClient(name, baseURL string) *client {
	return &client{name: name, baseURL: baseURL, http: &http.Client{Timeout: 2 * time.Second}}
}

func (c *client) getJSON(ctx context.Context, path, authorization string, dst any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, &DownstreamError{Service: c.name, Err: err}
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, &DownstreamError{Service: c.name, Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, &DownstreamError{Service: c.name, Status: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return resp.StatusCode, &DownstreamError{Service: c.name, Err: err}
	}
	return resp.StatusCode, nil
}
