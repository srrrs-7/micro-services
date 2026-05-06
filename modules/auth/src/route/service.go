package route

import (
	"auth/domain"
	"context"
)

type loginService interface {
	Post(ctx context.Context, input domain.LoginInput) (*domain.Token, error)
}
