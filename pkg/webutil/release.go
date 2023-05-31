package webutil

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type actualVersion struct {
	TagName string `json:"tag_name"`
}

func GetLatestRelease(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 200 {
		return "", errors.New("Failed to connect release server")
	}

	data, _ := io.ReadAll(resp.Body)
	release := new(actualVersion)
	json.Unmarshal(data, &release)

	return release.TagName, nil
}
