package routes

import (
	"auth/driver/routes/request"
	"auth/entity"
)

type LoginUseCase interface {
	Post(request.LoginRequest) (entity.Token, error)
	Get(request.LoginRequest) (entity.Token, error)
	Put(request.LoginRequest) error
	Delete(request.LoginRequest) error
}

type SessionUseCase interface {
	Post(request.SessionRequest) (entity.SessionID, error)
	Get(request.SessionRequest) (entity.SessionID, error)
	Put(request.SessionRequest) (entity.SessionID, error)
	Delete(request.SessionRequest) error
}
