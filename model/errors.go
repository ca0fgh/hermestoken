package model

import "errors"

// Common errors
var (
	ErrDatabase = errors.New("database error")
)

// User auth errors
var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUserEmptyCredentials = errors.New("empty credentials")
	ErrEmailAlreadyTaken    = errors.New("email already taken")
	ErrEmailNotFound        = errors.New("email not found")
	ErrEmailAmbiguous       = errors.New("email matches multiple users")
)

// localizedAuthError carries a user-facing message while still matching one of
// the sentinels above under errors.Is. Login errors are surfaced to the client
// verbatim, so the message must stay localized even though callers and tests
// match on the sentinel.
type localizedAuthError struct {
	message  string
	sentinel error
}

func (e *localizedAuthError) Error() string { return e.message }
func (e *localizedAuthError) Unwrap() error { return e.sentinel }

func newLocalizedAuthError(message string, sentinel error) error {
	return &localizedAuthError{message: message, sentinel: sentinel}
}

// Token auth errors
var (
	ErrTokenNotProvided = errors.New("token not provided")
	ErrTokenInvalid     = errors.New("token invalid")
)

// Redemption errors
var ErrRedeemFailed = errors.New("redeem.failed")

// 2FA errors
var ErrTwoFANotEnabled = errors.New("2fa not enabled")
