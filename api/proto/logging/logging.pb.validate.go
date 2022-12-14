// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: logging.proto

package logging

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
)

// Validate checks the field values on AddLogRecord with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *AddLogRecord) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on AddLogRecord with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in AddLogRecordMultiError, or
// nil if none found.
func (m *AddLogRecord) ValidateAll() error {
	return m.validate(true)
}

func (m *AddLogRecord) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return AddLogRecordMultiError(errors)
	}

	return nil
}

// AddLogRecordMultiError is an error wrapping multiple validation errors
// returned by AddLogRecord.ValidateAll() if the designated constraints aren't met.
type AddLogRecordMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m AddLogRecordMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m AddLogRecordMultiError) AllErrors() []error { return m }

// AddLogRecordValidationError is the validation error returned by
// AddLogRecord.Validate if the designated constraints aren't met.
type AddLogRecordValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e AddLogRecordValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e AddLogRecordValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e AddLogRecordValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e AddLogRecordValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e AddLogRecordValidationError) ErrorName() string { return "AddLogRecordValidationError" }

// Error satisfies the builtin error interface
func (e AddLogRecordValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sAddLogRecord.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = AddLogRecordValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = AddLogRecordValidationError{}

// Validate checks the field values on AddLogRecord_Request with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *AddLogRecord_Request) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on AddLogRecord_Request with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// AddLogRecord_RequestMultiError, or nil if none found.
func (m *AddLogRecord_Request) ValidateAll() error {
	return m.validate(true)
}

func (m *AddLogRecord_Request) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Logger

	// no validation rules for Event

	// no validation rules for Id

	if len(errors) > 0 {
		return AddLogRecord_RequestMultiError(errors)
	}

	return nil
}

// AddLogRecord_RequestMultiError is an error wrapping multiple validation
// errors returned by AddLogRecord_Request.ValidateAll() if the designated
// constraints aren't met.
type AddLogRecord_RequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m AddLogRecord_RequestMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m AddLogRecord_RequestMultiError) AllErrors() []error { return m }

// AddLogRecord_RequestValidationError is the validation error returned by
// AddLogRecord_Request.Validate if the designated constraints aren't met.
type AddLogRecord_RequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e AddLogRecord_RequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e AddLogRecord_RequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e AddLogRecord_RequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e AddLogRecord_RequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e AddLogRecord_RequestValidationError) ErrorName() string {
	return "AddLogRecord_RequestValidationError"
}

// Error satisfies the builtin error interface
func (e AddLogRecord_RequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sAddLogRecord_Request.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = AddLogRecord_RequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = AddLogRecord_RequestValidationError{}

// Validate checks the field values on AddLogRecord_Response with the rules
// defined in the proto definition for this message. If any rules are
// violated, the first error encountered is returned, or nil if there are no violations.
func (m *AddLogRecord_Response) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on AddLogRecord_Response with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// AddLogRecord_ResponseMultiError, or nil if none found.
func (m *AddLogRecord_Response) ValidateAll() error {
	return m.validate(true)
}

func (m *AddLogRecord_Response) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(errors) > 0 {
		return AddLogRecord_ResponseMultiError(errors)
	}

	return nil
}

// AddLogRecord_ResponseMultiError is an error wrapping multiple validation
// errors returned by AddLogRecord_Response.ValidateAll() if the designated
// constraints aren't met.
type AddLogRecord_ResponseMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m AddLogRecord_ResponseMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m AddLogRecord_ResponseMultiError) AllErrors() []error { return m }

// AddLogRecord_ResponseValidationError is the validation error returned by
// AddLogRecord_Response.Validate if the designated constraints aren't met.
type AddLogRecord_ResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e AddLogRecord_ResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e AddLogRecord_ResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e AddLogRecord_ResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e AddLogRecord_ResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e AddLogRecord_ResponseValidationError) ErrorName() string {
	return "AddLogRecord_ResponseValidationError"
}

// Error satisfies the builtin error interface
func (e AddLogRecord_ResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sAddLogRecord_Response.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = AddLogRecord_ResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = AddLogRecord_ResponseValidationError{}
