package util

import (
	"fmt"
	"reflect"
)

type ErrorCode int32

const (
	EC_BadRequest     = 100
	EC_NotSpecified   = 101
	EC_Parse          = 102
	EC_Range          = 103
	EC_InvalidData    = 104
	EC_BadToken       = 105
	EC_Unauthorized   = 106
	EC_NoPermission   = 107
	EC_NotImplemented = 108
	EC_Internal       = 200
	EC_Timeout        = 300
)

type Error struct {
	Code    ErrorCode
	Message string
	Name    string
	Cause   error
}

func NewError(code ErrorCode, message string) *Error {
	return &Error{code, message, "", nil}
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

var _ error = &Error{}

func NewNotSpecifiedError(name string) error {
	return &Error{EC_NotSpecified, fmt.Sprintf("%s not specified", name), name, nil}
}

func NewParseError(parseType string, cause error) error {
	return &Error{EC_Parse,
		fmt.Sprintf("could not parse %s", parseType), parseType, cause}
}

func NewInvalidDataError(dataType string, cause error) error {
	return &Error{EC_InvalidData,
		fmt.Sprintf("could not process %s", dataType), dataType, cause}
}

func NewInternalError(cause error) *Error {
	return &Error{EC_Internal, "internal error", "", cause}
}

// CheckNotNil checks that ref is not nil and produces an err with a Message if it is. name should be the
// name of what ref is
func CheckNotNil(ref interface{}, whatWasNil string) (err error) {
	v := reflect.ValueOf(ref)
	if ref == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		err = NewNotSpecifiedError(whatWasNil)
	}
	return
}

// CheckRange checks that ref is a valid index to a list of size max, and produces an err with a
// Message if it is not. name should be the name of what ref is.
func CheckRange(ref *int, name string, max int) (err error) {
	if err = CheckNotNil(ref, name); err != nil {
		return
	}
	var message string
	if *ref < 0 {
		message = fmt.Sprintf("%s out of range: %d < 0", name, *ref)
	}
	if *ref >= max {
		message = fmt.Sprintf("%s out of range: %d >= %d", name, *ref, max)
	}
	if message != "" {
		err = &Error{EC_Range, message, name, nil}
	}
	return
}
