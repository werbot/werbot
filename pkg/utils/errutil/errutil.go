package errutil

import (
	"strings"
)

type ErrorMap map[string]string

// Error implements the error interface for ErrorMap
func (em ErrorMap) Error() string {
	var b strings.Builder
	for key, value := range em {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\n")
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// StringToErrorMap converts a string to an ErrorMap
func StringToErrorMap(s string) ErrorMap {
	result := make(ErrorMap)
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if colonIndex := strings.IndexByte(line, ':'); colonIndex != -1 {
			key := strings.TrimSpace(line[:colonIndex])
			value := strings.TrimSpace(line[colonIndex+1:])
			result[key] = value
		}
	}
	return result
}
