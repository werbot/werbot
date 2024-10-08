package alias

import (
	"regexp"
	"strings"

	"github.com/werbot/werbot/pkg/utils/strutil"
)

// CheckAlias is ...
func CheckAlias(alias string) bool {
	unixUserRegexp := regexp.MustCompile("^[a-z_][a-zA-Z0-9_]{0,31}$")
	return unixUserRegexp.MatchString(alias)
}

// FixAlias  is ...
func FixAlias(alias string) string {
	return strings.Join(strutil.SplitTrimmed(alias, "_"), "_")
}
