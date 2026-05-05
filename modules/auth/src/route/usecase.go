package route

import (
	"auth/driver/routes/request"
	"auth/entity"
)

type LoginService interface {
	Post(request.LoginRequest) (entity.Token, error)
	Get(request.LoginRequest) (entity.Token, error)
	Put(request.LoginRequest) error
	Delete(request.LoginRequest) error
}

type SessionService interface {
	Post(request.SessionRequest) (entity.SessionID, error)
	Get(request.SessionRequest) (entity.SessionID, error)
	Put(request.SessionRequest) (entity.SessionID, error)
	Delete(request.SessionRequest) error
}
