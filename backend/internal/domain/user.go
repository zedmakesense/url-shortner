package domain

import "time"

type User struct {
	ID              int
	Name            string
	Email           string
	HashedPassword  string
	IsEmailVerified bool
	CreatedAt       time.Time
}

type UserResponse struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	IsEmailVerified bool      `json:"is_email_verified"`
	CreatedAt       time.Time `json:"created_at"`
}

type UserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Token struct {
	SessionID int
	UserID    int
	Token     []byte
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type EmailToken struct {
	ID          int
	UserID      int
	HashedToken []byte
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
}

type URL struct {
	ID         int
	ShortCode  string
	LongURL    string
	UserID     int
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	ClickCount int
}

type ErrorResponse struct {
	Error string `json:"error"`
}
