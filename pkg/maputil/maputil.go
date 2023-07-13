package maputil

import (
	"reflect"
	"strconv"
	"strings"
)

// GetByPath get value by key path from a map(map[string]any). eg "top" "top.sub"
func GetByPath(path string, mp map[string]any) (val any, ok bool) {
	if val, ok := mp[path]; ok {
		return val, true
	}

	// no sub key
	if len(mp) == 0 || strings.IndexByte(path, '.') < 1 {
		return nil, false
	}

	// has sub key. eg. "top.sub"
	keys := strings.Split(path, ".")
	return GetByPathKeys(mp, keys)
}

// GetByPathKeys get value by path keys from a map(map[string]any). eg "top" "top.sub"
func GetByPathKeys(mp map[string]any, keys []string) (val any, ok bool) {
	kl := len(keys)
	if kl == 0 {
		return mp, true
	}

	// find top item data use top key
	var item any

	topK := keys[0]
	if item, ok = mp[topK]; !ok {
		return
	}

	// find sub item data use sub key
	for i, k := range keys[1:] {
		switch tData := item.(type) {
		case map[string]string: // is string map
			if item, ok = tData[k]; !ok {
				return
			}
		case map[string]any: // is map(decode from toml/json/yaml)
			if item, ok = tData[k]; !ok {
				return
			}
		case map[any]any: // is map(decode from yaml.v2)
			if item, ok = tData[k]; !ok {
				return
			}
		case []map[string]any: // is an any-map slice
			if k == "*" {
				if kl == i+2 {
					return tData, true
				}

				// * is not last key, find sub item data
				sl := make([]any, 0)
				for _, v := range tData {
					if val, ok = GetByPathKeys(v, keys[i+2:]); ok {
						sl = append(sl, val)
					}
				}

				if len(sl) > 0 {
					return sl, true
				}
				return nil, false
			}

			// k is index number
			idx, err := strconv.Atoi(k)
			if err != nil {
				return nil, false
			}

			if idx >= len(tData) {
				return nil, false
			}
			item = tData[idx]
		default:
			rv := reflect.ValueOf(tData)
			// check is slice
			if rv.Kind() == reflect.Slice {
				i, err := strconv.Atoi(k)
				if err != nil {
					return nil, false
				}
				if i >= rv.Len() {
					return nil, false
				}

				item = rv.Index(i).Interface()
				continue
			}

			// as error
			return nil, false
		}
	}

	return item, true
}
