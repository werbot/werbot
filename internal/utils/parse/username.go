package parse

import (
	"strings"

	"github.com/werbot/werbot/internal/utils/convert"
)

// UsernameParseInfo is parse username and return username array
func UsernameParseInfo(name string) []string {
	nameArray := strings.SplitN(name, "_", 3)
	return convert.RemoveEmptyStrings(nameArray)
}
