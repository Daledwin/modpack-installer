// Package httpx provides tiny HTTP GET helpers (redirects handled by net/http).
package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxBytes = 256 << 20 // 256 MB

var client = &http.Client{Timeout: 120 * time.Second}

// Bytes fetches a URL and returns its body, erroring on non-200 / oversize.
func Bytes(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "modpack-installer/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBytes {
		return nil, fmt.Errorf("GET %s: response exceeds %d bytes", url, maxBytes)
	}
	return b, nil
}

// JSON fetches a URL and decodes the JSON body into v.
func JSON(url string, v any) error {
	b, err := Bytes(url)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
