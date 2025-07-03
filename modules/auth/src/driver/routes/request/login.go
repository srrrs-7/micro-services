package request

import "net/http"

type LoginRequest struct{}

func NewLoginRequest(r *http.Request) LoginRequest {
	return LoginRequest{}
}

func (r LoginRequest) Validate() error {
	return nil
}
