package structs

// SMap is alias of map[string]string
type SMap map[string]string

// Value get from the data map
func (m SMap) Value(key string) (string, bool) {
	val, ok := m[key]
	return val, ok
}

// Default get value by key. if not found, return defVal
func (m SMap) Default(key, defVal string) string {
	if val, ok := m[key]; ok {
		return val
	}
	return defVal
}

// Get value by key
func (m SMap) Get(key string) string {
	return m[key]
}
