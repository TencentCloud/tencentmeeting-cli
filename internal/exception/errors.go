package exception

import (
	"errors"
	"fmt"
)

// TmeetError represents a Tmeet error.
type TmeetError struct {
	Code    int
	Message string
}

func (e *TmeetError) Error() string {
	return e.Message
}

func (e *TmeetError) With(format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)
	if e == nil || message == "" {
		return e
	}
	return NewTmeetError(e.Code, message)
}

// NewTmeetError creates a new Tmeet error.
func NewTmeetError(code int, message string) *TmeetError {
	return &TmeetError{Code: code, Message: message}
}

// Is checks whether two errors are equal.
func Is(err, target error) bool {
	tmeetErr, errOk := err.(*TmeetError)
	targetTmeetErr, targetOk := target.(*TmeetError)
	if !errOk || !targetOk {
		return errors.Is(err, target)
	}
	return tmeetErr.Code == targetTmeetErr.Code
}

var (
	// InvalidArgsError indicates invalid arguments.
	InvalidArgsError = NewTmeetError(ClientCodeInvalidArgs, "invalid args")
	// InitializeFailedError indicates initialization failure.
	InitializeFailedError = NewTmeetError(ClientCodeInitializeFailed, "initialize failed")
	// GetUserConfigUnknownError indicates an unknown error when retrieving user configuration.
	GetUserConfigUnknownError = NewTmeetError(ClientCodeGetUserConfigUnknown, "get user config unknown error, please retry later or use 'tmeet auth logout' and 'tmeet auth login'")
	// ParseUserConfigError indicates failure to parse user configuration.
	ParseUserConfigError = NewTmeetError(ClientCodeParseUserConfig, "parse user config error, please use 'tmeet auth logout' and 'tmeet auth login', then try again")
	// GetUserConfigEmptyError indicates the user configuration is empty.
	GetUserConfigEmptyError = NewTmeetError(ClientCodeGetUserConfigEmpty, "user config is empty, please use 'tmeet auth login'")
	// UserHasBeenInitializedError indicates the user has already been initialized and does not need to be re-initialized.
	UserHasBeenInitializedError = NewTmeetError(ClientCodeUserHasBeenInitialized, "user has been login, please use 'tmeet cmd [flags]' to use")
	// UserIdentityExpiredError indicates the user identity has expired.
	UserIdentityExpiredError = NewTmeetError(ClientCodeUserIdentityExpired, "user identity expired, please use 'tmeet auth login'")
	// LogoutFailedError indicates logout failure.
	LogoutFailedError = NewTmeetError(ClientCodeLogoutFailed, "logout failed, please retry later")

	// NetworkError indicates a network error.
	NetworkError = NewTmeetError(ClientCodeNetwork, "network error, please retry later")
	// InvalidRestApiMethodError indicates an invalid REST request method.
	InvalidRestApiMethodError = NewTmeetError(ClientCodeInvalidRestApiMethod, "invalid rest api method")
	// InvalidNormalApiMethodError indicates an invalid normal request method.
	InvalidNormalApiMethodError = NewTmeetError(ClientCodeInvalidNormalApiMethod, "invalid normal api method")
	// ResponseDecodeError indicates failure to decode the response.
	ResponseDecodeError = NewTmeetError(ClientCodeResponseDecode, "response decode error")
	// TokenExpiredError indicates the token has expired.
	TokenExpiredError = NewTmeetError(ClientCodeTokenExpired, "your token has been expired, please use 'tmeet auth login', and try again")
	// RefreshTokenFailedError indicates failure to refresh the token.
	RefreshTokenFailedError = NewTmeetError(ClientCodeRefreshTokenFailed, "refresh token failed, please use 'tmeet auth login', and try again")
	// RestBusinessError rest business error
	RestBusinessError = NewTmeetError(ClientCodeRestBusiness, "rest business error")
	// UploadToCosError upload to cos failed
	UploadToCosError = NewTmeetError(ClientUploadToCos, "upload to cos failed")

	// GetAuthCodeError indicates failure to obtain auth_code.
	GetAuthCodeError = NewTmeetError(ClientCodeGetAuthCode, "get auth_code failed, please try later")
	// AuthorizationTimeoutError indicates authorization timeout.
	AuthorizationTimeoutError = NewTmeetError(ClientCodeAuthorizationTimeout, "authorization timeout, please try 'tmeet auth login' again")
	// AuthorizationFailedError indicates authorization failure.
	AuthorizationFailedError = NewTmeetError(ClientCodeAuthorizationFailed, "authorization failed, please try 'tmeet auth login' again")
)
