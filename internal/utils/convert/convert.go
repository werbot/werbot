package convert

import (
	"reflect"
	"strconv"
	"strings"
)

// StringToInt32 is convert string to int32
func StringToInt32(message string) int32 {
	i64, _ := strconv.ParseInt(message, 10, 32)
	return int32(i64)
}

// Int32ToString is convert int32 to string
func Int32ToString(num int32) string {
	return strconv.FormatInt(int64(num), 10)
}

// StringToBool is convert string to bool
func StringToBool(message string) bool {
	b, _ := strconv.ParseBool(message)
	return b
}

// FloatToString is convert float to string
func FloatToString(inputNum float64) string {
	return strconv.FormatFloat(inputNum, 'f', 6, 64)
}

// RemoveEmptyStrings - Use this to remove empty string values inside an array.
// This happens when allocation is bigger and empty
func RemoveEmptyStrings(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

// StructToMap is convert struct to map
func StructToMap(item any) map[string]any {
	res := map[string]any{}
	if item == nil {
		return res
	}
	v := reflect.TypeOf(item)
	reflectValue := reflect.ValueOf(item)
	reflectValue = reflect.Indirect(reflectValue)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		tag := v.Field(i).Tag.Get("json")

		// remove omitEmpty
		omitEmpty := false
		if strings.HasSuffix(tag, "omitempty") {
			omitEmpty = true
			idx := strings.Index(tag, ",")
			if idx > 0 {
				tag = tag[:idx]
			} else {
				tag = ""
			}
		}

		if tag != "" && tag != "-" {
			field := reflectValue.Field(i).Interface()
			if v.Field(i).Type.Kind() == reflect.Struct {
				res[tag] = StructToMap(field)
			} else if !(omitEmpty && reflectValue.Field(i).IsZero()) {
				res[tag] = field
			}
		}
	}
	return res
}
