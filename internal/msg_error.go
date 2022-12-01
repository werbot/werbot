package internal

const (
	// ErrBadRequest is bad request
	ErrBadRequest = "Bad request"

	// ErrNotFound is not Found
	ErrNotFound = "Not Found"

	// ErrUnauthorized is unauthorized
	ErrUnauthorized = "Unauthorized"

	// ErrValidateParams is error validating params
	ErrValidateParams = "Error validating params"

	// ErrValidateBodyParams is ...
	ErrValidateBodyParams = "Error validating body params"

	// ErrBadQueryParams is invalid query params
	ErrBadQueryParams = "Invalid query params"

	// ErrUnexpectedError is Unexpected error
	// Used in API to replace 500 error
	ErrUnexpectedError = "Unexpected error"

	// ErrInvalidPassword is ...
	ErrInvalidPassword = "Invalid password"
)
