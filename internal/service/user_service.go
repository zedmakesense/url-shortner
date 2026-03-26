package service

type RepositoryInterface interface {
}

type ServiceInterface interface {
}

type serviceStruct struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) *serviceStruct {
	return &serviceStruct{repo: repo}
}
