package domain

type User struct {
	ID             int64
	Name           string
	Username       string
	HashedPassword string
}

type UserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
