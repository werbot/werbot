package cache

import "time"

// Cache is ...
type Cache interface {
	Ping() error
	Set(key string, value any, expiration time.Duration) error
	Get(key string) (string, error)
	Delete(key string) (int64, error)
}
