package customerrors

import (
	"errors"
)

type AuthError struct {
	Type           string
	Message        string
	Err            error
	loginActionUrl string
	otpType        string // "email" | "max"
}

func (e *AuthError) Error() string {
	return e.Message
}

func NewAuthError(errType string, message string, err error) *AuthError {
	return &AuthError{
		Type:    errType,
		Message: err.Error(),
		Err:     errors.New(message),
	}
}

func (e *AuthError) SetLoginActionUrl(loginActionUrl string) *AuthError {
	e.loginActionUrl = loginActionUrl
	return e
}

func (e *AuthError) GetLoginActionUrl() string {
	return e.loginActionUrl
}

func (e *AuthError) SetOtpType(otpType string) *AuthError {
	e.otpType = otpType
	return e
}

func (e *AuthError) GetOtpType() string {
	if e.otpType == "" {
		return "email"
	}
	return e.otpType
}
