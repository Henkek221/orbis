package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
)

// Get fetches the contents of a URL
func Get(url string) (data []byte, err error) {
	res, err := http.Get(url)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("http status %d", res.StatusCode)
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	data, err = maybeGunzip(body)
	return
}

// GetWithCheck fetches the contents of a URL if the response passes a check
func GetWithCheck(url string, check func(*http.Response) error) (data []byte, err error) {
	res, err := http.Get(url)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("http status %d", res.StatusCode)
		return
	}
	err = check(res)
	if err != nil {
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	data, err = maybeGunzip(body)
	return
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
