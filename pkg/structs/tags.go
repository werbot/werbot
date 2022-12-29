package structs

import "github.com/werbot/werbot/pkg/strutil"

// ParseTagValueDefault parse like json tag value.
func ParseTagValueDefault(field, tagVal string) (mp SMap, err error) {
	ss := strutil.SplitTrimmed(tagVal, ",")
	ln := len(ss)
	if ln == 0 || tagVal == "," {
		return SMap{"name": field}, nil
	}

	mp = make(SMap, ln)
	if ln == 1 {
		// valid field name
		if ss[0] != "-" {
			mp["name"] = ss[0]
		}
		return
	}

	// ln > 1
	mp["name"] = ss[0]
	// other settings: omitempty, string
	for _, key := range ss[1:] {
		mp[key] = "true"
	}
	return
}
