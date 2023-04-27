package internal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/werbot/werbot/pkg/strutil"
)

// LoadConfig reads .env file configurations into ENV
func LoadConfig(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := godotenv.Load(path); err != nil {
		return err
	}

	return nil
}

func lookup(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// GetString is ...
func GetString(key, fallback string) string {
	return lookup(key, fallback)
}

// GetSliceString is ...
func GetSliceString(key, fallback string) []string {
	value := lookup(key, fallback)
	return strutil.ToSlice(value)
}

// GetInt is ...
func GetInt(key string, fallback int) int {
	value := lookup(key, "")
	if value, err := strconv.Atoi(value); err == nil {
		return value
	}
	return fallback
}

// GetInt32 is ...
func GetInt32(key string, fallback int32) int32 {
	value := lookup(key, "")
	if value, err := strconv.ParseInt(value, 10, 32); err == nil {
		return int32(value)
	}
	return fallback
}

// GetBool is ...
func GetBool(key string, fallback bool) bool {
	value := lookup(key, "")
	if value, err := strconv.ParseBool(value); err == nil {
		return value
	}
	return fallback
}

// GetDuration is ...
func GetDuration(key, fallback string) time.Duration {
	value := lookup(key, fallback)
	duration, _ := time.ParseDuration(value)
	return duration
}

// GetByteFromFile is ...
func GetByteFromFile(key, fallback string) ([]byte, error) {
	var data []byte
	value := lookup(key, fallback)

	// Simplify the if condition by handling the error directly from 'readFile'
	var err error
	if data, err = readFile(value); err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", value, err)
	}

	return data, nil
}

func readFile(file string) ([]byte, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", file, err)
	}
	return data, nil
}
