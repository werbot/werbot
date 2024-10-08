package internal

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/werbot/werbot/pkg/utils/strutil"
)

func lookup(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// GetString returns the environment variable value as a string or the fallback if not set.
func GetString(key, fallback string) string {
	return lookup(key, fallback)
}

// GetSliceString returns the environment variable value as a slice of strings or the fallback if not set.
func GetSliceString(key, fallback string) []string {
	return strutil.ToSlice(lookup(key, fallback))
}

// GetInt returns the environment variable value as an int or the fallback if not set.
func GetInt(key string, fallback int) int {
	if value, err := strconv.Atoi(lookup(key, "")); err == nil {
		return value
	}
	return fallback
}

// GetInt32 returns the environment variable value as an int32 or the fallback if not set.
func GetInt32(key string, fallback int32) int32 {
	if value, err := strconv.ParseInt(lookup(key, ""), 10, 32); err == nil {
		return int32(value)
	}
	return fallback
}

// GetBool returns the environment variable value as a bool or the fallback if not set.
func GetBool(key string, fallback bool) bool {
	if value, err := strconv.ParseBool(lookup(key, "")); err == nil {
		return value
	}
	return fallback
}

// GetDuration returns the environment variable value as a time.Duration or the fallback if not set.
func GetDuration(key, fallback string) time.Duration {
	duration, _ := time.ParseDuration(lookup(key, fallback))
	return duration
}

// GetByteFromFile returns the content of the file specified in the environment variable or fallback if not set.
func GetByteFromFile(key, fallback string) ([]byte, error) {
	value := lookup(key, fallback)
	data, err := readFile(value)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", value, err)
	}
	return data, nil
}

func readFile(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", file, err)
	}
	return data, nil
}
