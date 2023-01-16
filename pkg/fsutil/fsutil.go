package fsutil

import (
	"io"
	"net/http"
	"os"
)

// Download is download file
func Download(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// MustReadFile read file contents, will panic on error
func MustReadFile(filePath string) []byte {
	bs, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	return bs
}
