// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: auth.proto

package auth

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"

	user "github.com/werbot/werbot/api/proto/user"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort

	_ = user.Role(0)
)

// define the regex for a UUID once up-front
var _auth_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on SignIn with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *SignIn) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on SignIn with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in SignInMultiError, or nil if none found.
func (m *SignIn) ValidateAll() error {
	return m.validate(true)
}

func (m *SignIn) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return SignInMultiError(errors)
	}

	return nil
}

// SignInMultiError is an error wrapping multiple validation errors returned by
// SignIn.ValidateAll() if the designated constraints aren't met.
type SignInMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m SignInMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m SignInMultiError) AllErrors() []error { return m }

// SignInValidationError is the validation error returned by SignIn.Validate if
// the designated constraints aren't met.
type SignInValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SignInValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SignInValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SignInValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SignInValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SignInValidationError) ErrorName() string { return "SignInValidationError" }

// Error satisfies the builtin error interface
func (e SignInValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSignIn.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SignInValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SignInValidationError{}

// Validate checks the field values on ResetPassword with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *ResetPassword) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ResetPassword with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in ResetPasswordMultiError, or
// nil if none found.
func (m *ResetPassword) ValidateAll() error {
	return m.validate(true)
}

func (m *ResetPassword) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return ResetPasswordMultiError(errors)
	}

	return nil
}

// ResetPasswordMultiError is an error wrapping multiple validation errors
// returned by ResetPassword.ValidateAll() if the designated constraints
// aren't met.
type ResetPasswordMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ResetPasswordMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ResetPasswordMultiError) AllErrors() []error { return m }

// ResetPasswordValidationError is the validation error returned by
// ResetPassword.Validate if the designated constraints aren't met.
type ResetPasswordValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ResetPasswordValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ResetPasswordValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ResetPasswordValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ResetPasswordValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ResetPasswordValidationError) ErrorName() string { return "ResetPasswordValidationError" }

// Error satisfies the builtin error interface
func (e ResetPasswordValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sResetPassword.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ResetPasswordValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ResetPasswordValidationError{}

// Validate checks the field values on RefreshTokenRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *RefreshTokenRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on RefreshTokenRequest with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// RefreshTokenRequestMultiError, or nil if none found.
func (m *RefreshTokenRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *RefreshTokenRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if err := m._validateUuid(m.GetRefreshToken()); err != nil {
		err = RefreshTokenRequestValidationError{
			field:  "RefreshToken",
			reason: "value must be a valid UUID",
			cause:  err,
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return RefreshTokenRequestMultiError(errors)
	}

	return nil
}

func (m *RefreshTokenRequest) _validateUuid(uuid string) error {
	if matched := _auth_uuidPattern.MatchString(uuid); !matched {
		return errors.New("invalid uuid format")
	}

	return nil
}

// RefreshTokenRequestMultiError is an error wrapping multiple validation
// errors returned by RefreshTokenRequest.ValidateAll() if the designated
// constraints aren't met.
type RefreshTokenRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m RefreshTokenRequestMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m RefreshTokenRequestMultiError) AllErrors() []error { return m }

// RefreshTokenRequestValidationError is the validation error returned by
// RefreshTokenRequest.Validate if the designated constraints aren't met.
type RefreshTokenRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e RefreshTokenRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e RefreshTokenRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e RefreshTokenRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e RefreshTokenRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e RefreshTokenRequestValidationError) ErrorName() string {
	return "RefreshTokenRequestValidationError"
}

// Error satisfies the builtin error interface
func (e RefreshTokenRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sRefreshTokenRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = RefreshTokenRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = RefreshTokenRequestValidationError{}

// Validate checks the field values on UserParameters with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *UserParameters) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UserParameters with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in UserParametersMultiError,
// or nil if none found.
func (m *UserParameters) ValidateAll() error {
	return m.validate(true)
}

func (m *UserParameters) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for UserName

	// no validation rules for UserId

	// no validation rules for Roles

	// no validation rules for Sub

	if len(errors) > 0 {
		return UserParametersMultiError(errors)
	}

	return nil
}

// UserParametersMultiError is an error wrapping multiple validation errors
// returned by UserParameters.ValidateAll() if the designated constraints
// aren't met.
type UserParametersMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UserParametersMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UserParametersMultiError) AllErrors() []error { return m }

// UserParametersValidationError is the validation error returned by
// UserParameters.Validate if the designated constraints aren't met.
type UserParametersValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UserParametersValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UserParametersValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UserParametersValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UserParametersValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UserParametersValidationError) ErrorName() string { return "UserParametersValidationError" }

// Error satisfies the builtin error interface
func (e UserParametersValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUserParameters.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UserParametersValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UserParametersValidationError{}

// Validate checks the field values on SignIn_Request with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *SignIn_Request) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on SignIn_Request with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in SignIn_RequestMultiError,
// or nil if none found.
func (m *SignIn_Request) ValidateAll() error {
	return m.validate(true)
}

func (m *SignIn_Request) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if err := m._validateEmail(m.GetEmail()); err != nil {
		err = SignIn_RequestValidationError{
			field:  "Email",
			reason: "value must be a valid email address",
			cause:  err,
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if l := utf8.RuneCountInString(m.GetPassword()); l < 8 || l > 32 {
		err := SignIn_RequestValidationError{
			field:  "Password",
			reason: "value length must be between 8 and 32 runes, inclusive",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return SignIn_RequestMultiError(errors)
	}

	return nil
}

func (m *SignIn_Request) _validateHostname(host string) error {
	s := strings.ToLower(strings.TrimSuffix(host, "."))

	if len(host) > 253 {
		return errors.New("hostname cannot exceed 253 characters")
	}

	for _, part := range strings.Split(s, ".") {
		if l := len(part); l == 0 || l > 63 {
			return errors.New("hostname part must be non-empty and cannot exceed 63 characters")
		}

		if part[0] == '-' {
			return errors.New("hostname parts cannot begin with hyphens")
		}

		if part[len(part)-1] == '-' {
			return errors.New("hostname parts cannot end with hyphens")
		}

		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("hostname parts can only contain alphanumeric characters or hyphens, got %q", string(r))
			}
		}
	}

	return nil
}

func (m *SignIn_Request) _validateEmail(addr string) error {
	a, err := mail.ParseAddress(addr)
	if err != nil {
		return err
	}
	addr = a.Address

	if len(addr) > 254 {
		return errors.New("email addresses cannot exceed 254 characters")
	}

	parts := strings.SplitN(addr, "@", 2)

	if len(parts[0]) > 64 {
		return errors.New("email address local phrase cannot exceed 64 characters")
	}

	return m._validateHostname(parts[1])
}

// SignIn_RequestMultiError is an error wrapping multiple validation errors
// returned by SignIn_Request.ValidateAll() if the designated constraints
// aren't met.
type SignIn_RequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m SignIn_RequestMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m SignIn_RequestMultiError) AllErrors() []error { return m }

// SignIn_RequestValidationError is the validation error returned by
// SignIn_Request.Validate if the designated constraints aren't met.
type SignIn_RequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SignIn_RequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SignIn_RequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SignIn_RequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SignIn_RequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SignIn_RequestValidationError) ErrorName() string { return "SignIn_RequestValidationError" }

// Error satisfies the builtin error interface
func (e SignIn_RequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSignIn_Request.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SignIn_RequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SignIn_RequestValidationError{}

// Validate checks the field values on SignIn_Response with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *SignIn_Response) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on SignIn_Response with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// SignIn_ResponseMultiError, or nil if none found.
func (m *SignIn_Response) ValidateAll() error {
	return m.validate(true)
}

func (m *SignIn_Response) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for UserId

	// no validation rules for Name

	// no validation rules for Email

	// no validation rules for UserRole

	if len(errors) > 0 {
		return SignIn_ResponseMultiError(errors)
	}

	return nil
}

// SignIn_ResponseMultiError is an error wrapping multiple validation errors
// returned by SignIn_Response.ValidateAll() if the designated constraints
// aren't met.
type SignIn_ResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m SignIn_ResponseMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m SignIn_ResponseMultiError) AllErrors() []error { return m }

// SignIn_ResponseValidationError is the validation error returned by
// SignIn_Response.Validate if the designated constraints aren't met.
type SignIn_ResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SignIn_ResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SignIn_ResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SignIn_ResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SignIn_ResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SignIn_ResponseValidationError) ErrorName() string { return "SignIn_ResponseValidationError" }

// Error satisfies the builtin error interface
func (e SignIn_ResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSignIn_Response.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SignIn_ResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SignIn_ResponseValidationError{}

// Validate checks the field values on ResetPassword_Request with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *ResetPassword_Request) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ResetPassword_Request with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ResetPassword_RequestMultiError, or nil if none found.
func (m *ResetPassword_Request) ValidateAll() error {
	return m.validate(true)
}

func (m *ResetPassword_Request) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if m.GetToken() != "" {

		if err := m._validateUuid(m.GetToken()); err != nil {
			err = ResetPassword_RequestValidationError{
				field:  "Token",
				reason: "value must be a valid UUID",
				cause:  err,
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

	}

	switch v := m.Request.(type) {
	case *ResetPassword_Request_Email:
		if v == nil {
			err := ResetPassword_RequestValidationError{
				field:  "Request",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

		if err := m._validateEmail(m.GetEmail()); err != nil {
			err = ResetPassword_RequestValidationError{
				field:  "Email",
				reason: "value must be a valid email address",
				cause:  err,
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

	case *ResetPassword_Request_Password:
		if v == nil {
			err := ResetPassword_RequestValidationError{
				field:  "Request",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

		if l := utf8.RuneCountInString(m.GetPassword()); l < 8 || l > 32 {
			err := ResetPassword_RequestValidationError{
				field:  "Password",
				reason: "value length must be between 8 and 32 runes, inclusive",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

	default:
		_ = v // ensures v is used
	}

	if len(errors) > 0 {
		return ResetPassword_RequestMultiError(errors)
	}

	return nil
}

func (m *ResetPassword_Request) _validateHostname(host string) error {
	s := strings.ToLower(strings.TrimSuffix(host, "."))

	if len(host) > 253 {
		return errors.New("hostname cannot exceed 253 characters")
	}

	for _, part := range strings.Split(s, ".") {
		if l := len(part); l == 0 || l > 63 {
			return errors.New("hostname part must be non-empty and cannot exceed 63 characters")
		}

		if part[0] == '-' {
			return errors.New("hostname parts cannot begin with hyphens")
		}

		if part[len(part)-1] == '-' {
			return errors.New("hostname parts cannot end with hyphens")
		}

		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("hostname parts can only contain alphanumeric characters or hyphens, got %q", string(r))
			}
		}
	}

	return nil
}

func (m *ResetPassword_Request) _validateEmail(addr string) error {
	a, err := mail.ParseAddress(addr)
	if err != nil {
		return err
	}
	addr = a.Address

	if len(addr) > 254 {
		return errors.New("email addresses cannot exceed 254 characters")
	}

	parts := strings.SplitN(addr, "@", 2)

	if len(parts[0]) > 64 {
		return errors.New("email address local phrase cannot exceed 64 characters")
	}

	return m._validateHostname(parts[1])
}

func (m *ResetPassword_Request) _validateUuid(uuid string) error {
	if matched := _auth_uuidPattern.MatchString(uuid); !matched {
		return errors.New("invalid uuid format")
	}

	return nil
}

// ResetPassword_RequestMultiError is an error wrapping multiple validation
// errors returned by ResetPassword_Request.ValidateAll() if the designated
// constraints aren't met.
type ResetPassword_RequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ResetPassword_RequestMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ResetPassword_RequestMultiError) AllErrors() []error { return m }

// ResetPassword_RequestValidationError is the validation error returned by
// ResetPassword_Request.Validate if the designated constraints aren't met.
type ResetPassword_RequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ResetPassword_RequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ResetPassword_RequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ResetPassword_RequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ResetPassword_RequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ResetPassword_RequestValidationError) ErrorName() string {
	return "ResetPassword_RequestValidationError"
}

// Error satisfies the builtin error interface
func (e ResetPassword_RequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sResetPassword_Request.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ResetPassword_RequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ResetPassword_RequestValidationError{}

// Validate checks the field values on ResetPassword_Response with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *ResetPassword_Response) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ResetPassword_Response with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ResetPassword_ResponseMultiError, or nil if none found.
func (m *ResetPassword_Response) ValidateAll() error {
	return m.validate(true)
}

func (m *ResetPassword_Response) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Message

	// no validation rules for Token

	if len(errors) > 0 {
		return ResetPassword_ResponseMultiError(errors)
	}

	return nil
}

// ResetPassword_ResponseMultiError is an error wrapping multiple validation
// errors returned by ResetPassword_Response.ValidateAll() if the designated
// constraints aren't met.
type ResetPassword_ResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ResetPassword_ResponseMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ResetPassword_ResponseMultiError) AllErrors() []error { return m }

// ResetPassword_ResponseValidationError is the validation error returned by
// ResetPassword_Response.Validate if the designated constraints aren't met.
type ResetPassword_ResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ResetPassword_ResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ResetPassword_ResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ResetPassword_ResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ResetPassword_ResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ResetPassword_ResponseValidationError) ErrorName() string {
	return "ResetPassword_ResponseValidationError"
}

// Error satisfies the builtin error interface
func (e ResetPassword_ResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sResetPassword_Response.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ResetPassword_ResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ResetPassword_ResponseValidationError{}
