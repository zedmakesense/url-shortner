package domain

import "errors"

var ErrEmailAlreadyExists = errors.New("email already exists")
var ErrTableAlreadyExists = errors.New("email already exists")
var ErrUserDoesNotExist = errors.New("user does not exist")

var ErrTokenNotFound = errors.New("session not found")
var ErrAccessTokenExpired = errors.New("access token expired")
var ErrRefreshTokenExpired = errors.New("refresh token expired")

var ErrResendApiKeyNotFound = errors.New("resend api key not found")
var ErrEmailAlreadyVerified = errors.New("email already verified")

var ErrEmailVerificationFailed = errors.New("email verification failed")
