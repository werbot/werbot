package system

import (
	_ "embed"
	"strings"
	"sync"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

//go:embed countries.tsv
var countriesTSV string

// countries parses the embedded "CODE<TAB>Name" list once, in file order.
var countries = sync.OnceValue(func() []*werbotv1.Country {
	var out []*werbotv1.Country
	for line := range strings.Lines(countriesTSV) {
		if code, name, ok := strings.Cut(strings.TrimRight(line, "\r\n"), "\t"); ok {
			out = append(out, &werbotv1.Country{Code: code, Name: name})
		}
	}
	return out
})
