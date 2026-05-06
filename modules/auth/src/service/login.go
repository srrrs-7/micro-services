package service

import (
	"auth/domain"
	"context"
)

type LoginService struct{}

func NewLoginService() LoginService {
	return LoginService{}
}

func (s LoginService) Post(ctx context.Context, input domain.LoginInput) (domain.Token, error) {
	return domain.Token{}, nil
}
