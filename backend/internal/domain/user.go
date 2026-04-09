package domain

import (
	"time"
)

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
	Password string `json:"password"` // #nosec G117
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

type URLResponse struct {
	ID         int        `json:"id"`
	ShortCode  string     `json:"short_code"`
	LongURL    string     `json:"long_url"`
	UserID     int        `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ClickCount int        `json:"click_count"`
}

type LongURL struct {
	LongURL string `json:"longURL"`
}

type ShortCode struct {
	ShortCode string `json:"shortCode"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
