package service

import (
	"auth/domain"
	"auth/infra/database/db"
	"context"
	"fmt"
	"shared/utilhttp"
)

type LoginService struct {
	repo db.Querier
}

func NewLoginService(repo db.Querier) LoginService {
	return LoginService{repo}
}

func (s LoginService) Post(ctx context.Context, input domain.LoginInput) (*domain.Token, error) {
	user, err := s.repo.GetUser(ctx, input.Email)
	if err != nil {
		return nil, utilhttp.NewDBError(fmt.Errorf("failed to retrieve user from database: %v", err))
	}

	if user.Password != input.Password {
		return nil, utilhttp.NewUnauthorizedError(fmt.Errorf("invalid credentials"))
	}

	return &domain.Token{}, nil
}
