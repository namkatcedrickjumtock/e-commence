package services

import "github.com/namkatcedrickjumtock/e-commence/persistence"

type ServiceImpl struct {
	respository persistence.Querier
}

type Service interface {
}

func NewService() Service {
	return &ServiceImpl{}
}
