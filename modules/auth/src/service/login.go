package service

import (
	"auth/domain"
	"auth/infra/database/db"
	"context"
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
		return nil, utilhttp.DBError{Message: "Failed to retrieve user from database"}
	}

	if user.Password != input.Password {
		return nil, utilhttp.UnauthorizedError{Message: "Invalid credentials"}
	}

	return &domain.Token{}, nil
}
