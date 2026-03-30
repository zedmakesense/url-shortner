package domain

import "errors"

var ErrEmailAlreadyExists = errors.New("email already exists")
var ErrUserDoesNotExist = errors.New("user does not exist")
var ErrTokenNotFound = errors.New("session not found")
