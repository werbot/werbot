// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: info.proto

package info

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

	_ = user.RoleUser(0)
)

// define the regex for a UUID once up-front
var _info_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on UserStatistics with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *UserStatistics) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UserStatistics with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in UserStatisticsMultiError,
// or nil if none found.
func (m *UserStatistics) ValidateAll() error {
	return m.validate(true)
}

func (m *UserStatistics) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return UserStatisticsMultiError(errors)
	}

	return nil
}

// UserStatisticsMultiError is an error wrapping multiple validation errors
// returned by UserStatistics.ValidateAll() if the designated constraints
// aren't met.
type UserStatisticsMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UserStatisticsMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UserStatisticsMultiError) AllErrors() []error { return m }

// UserStatisticsValidationError is the validation error returned by
// UserStatistics.Validate if the designated constraints aren't met.
type UserStatisticsValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UserStatisticsValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UserStatisticsValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UserStatisticsValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UserStatisticsValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UserStatisticsValidationError) ErrorName() string { return "UserStatisticsValidationError" }

// Error satisfies the builtin error interface
func (e UserStatisticsValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUserStatistics.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UserStatisticsValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UserStatisticsValidationError{}

// Validate checks the field values on UserStatistics_Request with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *UserStatistics_Request) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UserStatistics_Request with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// UserStatistics_RequestMultiError, or nil if none found.
func (m *UserStatistics_Request) ValidateAll() error {
	return m.validate(true)
}

func (m *UserStatistics_Request) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if m.GetUserId() != "" {

		if err := m._validateUuid(m.GetUserId()); err != nil {
			err = UserStatistics_RequestValidationError{
				field:  "UserId",
				reason: "value must be a valid UUID",
				cause:  err,
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

	}

	// no validation rules for Role

	if len(errors) > 0 {
		return UserStatistics_RequestMultiError(errors)
	}

	return nil
}

func (m *UserStatistics_Request) _validateUuid(uuid string) error {
	if matched := _info_uuidPattern.MatchString(uuid); !matched {
		return errors.New("invalid uuid format")
	}

	return nil
}

// UserStatistics_RequestMultiError is an error wrapping multiple validation
// errors returned by UserStatistics_Request.ValidateAll() if the designated
// constraints aren't met.
type UserStatistics_RequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UserStatistics_RequestMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UserStatistics_RequestMultiError) AllErrors() []error { return m }

// UserStatistics_RequestValidationError is the validation error returned by
// UserStatistics_Request.Validate if the designated constraints aren't met.
type UserStatistics_RequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UserStatistics_RequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UserStatistics_RequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UserStatistics_RequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UserStatistics_RequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UserStatistics_RequestValidationError) ErrorName() string {
	return "UserStatistics_RequestValidationError"
}

// Error satisfies the builtin error interface
func (e UserStatistics_RequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUserStatistics_Request.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UserStatistics_RequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UserStatistics_RequestValidationError{}

// Validate checks the field values on UserStatistics_Response with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *UserStatistics_Response) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UserStatistics_Response with the
// rules defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// UserStatistics_ResponseMultiError, or nil if none found.
func (m *UserStatistics_Response) ValidateAll() error {
	return m.validate(true)
}

func (m *UserStatistics_Response) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Users

	// no validation rules for Projects

	// no validation rules for Servers

	if len(errors) > 0 {
		return UserStatistics_ResponseMultiError(errors)
	}

	return nil
}

// UserStatistics_ResponseMultiError is an error wrapping multiple validation
// errors returned by UserStatistics_Response.ValidateAll() if the designated
// constraints aren't met.
type UserStatistics_ResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UserStatistics_ResponseMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UserStatistics_ResponseMultiError) AllErrors() []error { return m }

// UserStatistics_ResponseValidationError is the validation error returned by
// UserStatistics_Response.Validate if the designated constraints aren't met.
type UserStatistics_ResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UserStatistics_ResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UserStatistics_ResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UserStatistics_ResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UserStatistics_ResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UserStatistics_ResponseValidationError) ErrorName() string {
	return "UserStatistics_ResponseValidationError"
}

// Error satisfies the builtin error interface
func (e UserStatistics_ResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUserStatistics_Response.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UserStatistics_ResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UserStatistics_ResponseValidationError{}
