package mathutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// RoundToHundred is ....
func RoundToHundred(n int32) int32 {
	return ((n + 50) / 100) * 100
}

// ToString convert intX/floatX value to string, return error on failed
func ToString(val any) (string, error) {
	return TryToString(val, true)
}

// TryToString try convert intX/floatX value to string
func TryToString(val any, defaultAsErr bool) (str string, err error) {
	if val == nil {
		return
	}

	switch value := val.(type) {
	case int:
		str = strconv.Itoa(value)
	case int8:
		str = strconv.Itoa(int(value))
	case int16:
		str = strconv.Itoa(int(value))
	case int32: // same as `rune`
		str = strconv.Itoa(int(value))
	case int64:
		str = strconv.FormatInt(value, 10)
	case uint:
		str = strconv.FormatUint(uint64(value), 10)
	case uint8:
		str = strconv.FormatUint(uint64(value), 10)
	case uint16:
		str = strconv.FormatUint(uint64(value), 10)
	case uint32:
		str = strconv.FormatUint(uint64(value), 10)
	case uint64:
		str = strconv.FormatUint(value, 10)
	case float32:
		str = strconv.FormatFloat(float64(value), 'f', -1, 32)
	case float64:
		str = strconv.FormatFloat(value, 'f', -1, 64)
	case time.Duration:
		str = strconv.FormatUint(uint64(value.Nanoseconds()), 10)
	case json.Number:
		str = value.String()
	default:
		if defaultAsErr {
			err = errors.New("convert value type error")
		} else {
			str = fmt.Sprint(value)
		}
	}
	return
}
