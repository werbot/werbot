package errutil

import (
	"fmt"
	"strings"
)

type ErrorMap map[string]string

func (em ErrorMap) Error() string {
	var b strings.Builder
	for key, value := range em {
		fmt.Fprintf(&b, "%s: %s\n", key, value)
	}
	return b.String()
}
