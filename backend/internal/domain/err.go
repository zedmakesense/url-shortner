package domain

import "errors"

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserDoesNotExist   = errors.New("user does not exist")
)

var (
	ErrTokenNotFound       = errors.New("session not found")
	ErrAccessTokenExpired  = errors.New("access token expired")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

var (
	ErrResendAPIKeyNotFound = errors.New("resend api key not found")
	ErrEmailAlreadyVerified = errors.New("email already verified")
)

var ErrEmailVerificationFailed = errors.New("email verification failed")

var (
	ErrURLAlreadyExist = errors.New("url short code already exists")
	ErrURLDoesNotExist = errors.New("url short code does not exists")
)

var ErrCachingFailed = errors.New("redis was unable to cache")
