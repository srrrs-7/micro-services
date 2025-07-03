package request

import "net/http"

type SessionRequest struct{}

func NewSessionRequest(r *http.Request) SessionRequest {
	return SessionRequest{}
}

func (r SessionRequest) Validate() error {
	return nil
}
