package fsutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Download downloads file from url and saves it to filepath
func Download(filepath string, url string) (err error) {
	// Make HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check HTTP response status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := fmt.Sprintf("bad status: %s", resp.Status)
		return errors.New(errMsg)
	}

	// Open file to write
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() {
		_err := file.Close()
		if err == nil {
			err = _err
		}
	}()

	// Copy file content from response body
	_, err = io.Copy(file, resp.Body)
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

// RemoveByteLineBreak is ...
func RemoveByteLineBreak(content []byte) string {
	return string(bytes.ReplaceAll(content, []byte("\n"), []byte("")))
}

// PathExists reports whether the named file or directory exists.
func PathExists(path string) bool {
	if path == "" {
		return false
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// FileExists reports whether the named file or directory exists.
func FileExists(path string) bool {
	if path == "" {
		return false
	}

	if fi, err := os.Stat(path); err == nil {
		return !fi.IsDir()
	}
	return false
}
