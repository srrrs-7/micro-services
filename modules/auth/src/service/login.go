package service

import (
	"auth/domain"
	"auth/route/request"
)

type LoginService struct{}

func (s LoginService) Post(req request.LoginRequest) (domain.Token, error) {
	return domain.Token{}, nil
}
