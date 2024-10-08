package webutil

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// actualVersion represents the structure of the release version.
type actualVersion struct {
	TagName string `json:"tag_name"`
}

// GetLatestRelease fetches the latest release tag name from the provided URL.
func GetLatestRelease(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release server returned non-OK status: %d", resp.StatusCode)
	}

	var release actualVersion
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode release data: %w", err)
	}

	return release.TagName, nil
}
