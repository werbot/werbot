package uuid

import "github.com/google/uuid"

// New generates a new UUID and returns it as a string.
func New() string {
	return uuid.New().String()
}

// IsValid checks if the provided string is a valid UUID.
func IsValid(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
