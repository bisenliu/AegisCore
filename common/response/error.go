package response

import (
	"errors"
	"net/http"
)

type Code string

const (
	CodeOK            Code = "OK"
	CodeBadRequest    Code = "BAD_REQUEST"
	CodeNotFound      Code = "NOT_FOUND"
	CodeInternalError Code = "INTERNAL_ERROR"
)

type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

func Wrap(err error, code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, Cause: err}
}

func BadRequestError(message string) *Error {
	return NewError(CodeBadRequest, message, http.StatusBadRequest)
}

func NotFoundError(message string) *Error {
	return NewError(CodeNotFound, message, http.StatusNotFound)
}

func InternalError(err error) *Error {
	return Wrap(err, CodeInternalError, "internal server error", http.StatusInternalServerError)
}

func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return InternalError(err)
}
