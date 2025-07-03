package request

import "net/http"

type LogoutRequest struct{}

func NewLogoutRequest(r *http.Request) LogoutRequest {
	return LogoutRequest{}
}

func (r LogoutRequest) Validate() error {
	return nil
}
