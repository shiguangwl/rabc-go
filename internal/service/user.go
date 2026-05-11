package service

//go:generate go tool mockgen -source=user.go -destination=../../test/mocks/service/user.go

import (
	"context"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
)

type UserService interface {
	GetUser(ctx context.Context, id int64) (*model.User, error)
}

func NewUserService(
	service *Service,
	userRepository repository.UserRepository,
) UserService {
	return &userService{
		Service:        service,
		userRepository: userRepository,
	}
}

type userService struct {
	*Service
	userRepository repository.UserRepository
}

func (s *userService) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return s.userRepository.GetUser(ctx, id)
}
