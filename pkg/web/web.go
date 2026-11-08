package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Get fetches the contents of a URL
func Get(url string) (data []byte, err error) {
	return GetWithCheck(url, func(*http.Response) error { return nil })
}

// GetWithCheck fetches the contents of a URL if the response passes a check.
// Network errors and 5xx/429 responses are retried a few times with backoff;
// a long scan otherwise fails on one dropped TLS handshake.
func GetWithCheck(url string, check func(*http.Response) error) (data []byte, err error) {
	delay := 400 * time.Millisecond
	for attempt := 1; ; attempt++ {
		var retry bool
		data, retry, err = getOnce(url, check)
		if err == nil || !retry || attempt == 4 {
			return data, err
		}
		time.Sleep(delay)
		delay *= 2
	}
}

var client = &http.Client{Timeout: 90 * time.Second}

func getOnce(url string, check func(*http.Response) error) (data []byte, retry bool, err error) {
	res, err := client.Get(url)
	if err != nil {
		return nil, true, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		retry = res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests
		return nil, retry, fmt.Errorf("http status %d", res.StatusCode)
	}
	if err = check(res); err != nil {
		return nil, false, err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, true, err
	}
	data, err = maybeGunzip(body)
	return data, false, err
}

func maybeGunzip(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
