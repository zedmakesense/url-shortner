package domain

type User struct {
	ID             int64
	Name           string
	Username       string
	HashedPassword string
}
