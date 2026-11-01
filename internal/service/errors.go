package service

type ErrorCode string

const (
	CodeInvalid      ErrorCode = "invalid"
	CodeUnauthorized ErrorCode = "unauthorized"
	CodeForbidden    ErrorCode = "forbidden"
	CodeNotFound     ErrorCode = "not_found"
	CodeConflict     ErrorCode = "conflict"
)

type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func newError(code ErrorCode, message string) error {
	return &Error{Code: code, Message: message}
}
