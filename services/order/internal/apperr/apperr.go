package apperr

import "errors"

type Kind int

const (
	KindInvalid Kind = iota + 1
	KindNotFound
	KindForbidden
	KindConflict
	KindUnauthorized
	KindUnavailable
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Because(cause error) *Error {
	e.Err = cause
	return e
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Invalid(code, message string) *Error      { return New(KindInvalid, code, message) }
func NotFound(code, message string) *Error     { return New(KindNotFound, code, message) }
func Forbidden(code, message string) *Error    { return New(KindForbidden, code, message) }
func Conflict(code, message string) *Error     { return New(KindConflict, code, message) }
func Unauthorized(code, message string) *Error { return New(KindUnauthorized, code, message) }
func Unavailable(code, message string) *Error  { return New(KindUnavailable, code, message) }

func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return 0
}

func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
