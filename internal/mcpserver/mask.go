package mcpserver

import "regexp"

// secretKey matches argument names whose values are never recorded.
var secretKey = regexp.MustCompile(`(?i)password|secret|token|key`)

// maskArguments copies args with the value of every key that matches
// secretKey, at any depth, replaced by "[masked]".
func maskArguments(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for k, v := range args {
		if secretKey.MatchString(k) {
			out[k] = "[masked]"
			continue
		}
		out[k] = maskValue(v)
	}
	return out
}

func maskValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		return maskArguments(v)
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = maskValue(e)
		}
		return out
	}
	return v
}
