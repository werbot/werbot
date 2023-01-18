package strutil

import (
	"strconv"
	"strings"
)

// ToSlice split string to array.
func ToSlice(s string, sep ...string) []string {
	if len(sep) > 0 {
		return strings.Split(s, sep[0])
	}
	return strings.Split(s, ",")
}

// StringInSlice is compare string in slice
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// ToInt32 is convert string to int32
func ToInt32(s string) int32 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 256
	}
	return int32(parsed)
}
