package route

import (
	"auth/domain"
	"auth/route/request"
)

type loginService interface {
	Post(req request.LoginRequest) (domain.Token, error)
}
