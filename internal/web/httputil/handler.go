package httputil

// HTTPResponse represents response body of API
type HTTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Result  any    `json:"result,omitempty"`
}

// HTTPError is ...
// type HTTPError struct {
// 	Code    int         `json:"code"`
// 	Message string      `json:"message"`
// 	Result  any `json:"result,omitempty"`
// }
