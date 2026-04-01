package domain

import "time"

type User struct {
	ID             int
	Name           string
	Email          string
	HashedPassword string
	CreatedAt      time.Time
}

type UserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
