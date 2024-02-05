package webutil

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type actualVersion struct {
	TagName string `json:"tag_name"`
}

func GetLatestRelease(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release server returned non-OK status: %d", resp.StatusCode)
	}

	release := &actualVersion{}
	err = json.NewDecoder(resp.Body).Decode(release)
	if err != nil {
		return "", fmt.Errorf("failed to decode release data: %w", err)
	}

	return release.TagName, nil
}
