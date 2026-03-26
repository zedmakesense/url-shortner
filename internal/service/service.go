package service

type Repository interface {
}

type Service interface {
}

type service struct {
	repo Repository
}

func NewService(repo Repository) *service {
	return &service{repo: repo}
}
