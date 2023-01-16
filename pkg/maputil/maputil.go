package maputil

import (
	"reflect"
	"strconv"
	"strings"
)

// Get value by key path. eg "top" "top.sub"
func Get(mp map[string]any, path string) (val any) {
	val, _ = GetByPath(path, mp)
	return
}

// GetByPath get value by key path from a map(map[string]any)
func GetByPath(path string, mp map[string]any) (val any, ok bool) {
	if val, ok := mp[path]; ok {
		return val, true
	}

	// no sub key
	if len(mp) == 0 || !strings.ContainsRune(path, '.') {
		return nil, false
	}

	// has sub key. eg. "top.sub"
	keys := strings.Split(path, ".")
	topK := keys[0]

	// find top item data use top key
	var item any
	if item, ok = mp[topK]; !ok {
		return
	}

	for _, k := range keys[1:] {
		switch tData := item.(type) {
		case map[string]string: // is simple map
			if item, ok = tData[k]; !ok {
				return
			}
		case map[string]any: // is map(decode from toml/json)
			if item, ok = tData[k]; !ok {
				return
			}
		case map[any]any: // is map(decode from yaml)
			if item, ok = tData[k]; !ok {
				return
			}
		case []any: // is a slice
			if item, ok = getBySlice(k, tData); !ok {
				return
			}
		case []string, []int, []float32, []float64, []bool, []rune:
			slice := reflect.ValueOf(tData)
			sData := make([]any, slice.Len())
			for i := 0; i < slice.Len(); i++ {
				sData[i] = slice.Index(i).Interface()
			}
			if item, ok = getBySlice(k, sData); !ok {
				return
			}
		default: // error
			return nil, false
		}
	}

	return item, true
}

func getBySlice(k string, slice []any) (val any, ok bool) {
	i, err := strconv.ParseInt(k, 10, 64)
	if err != nil {
		return nil, false
	}
	if size := int64(len(slice)); i >= size {
		return nil, false
	}
	return slice[i], true
}
