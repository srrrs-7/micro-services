package routes

import (
	"auth/driver/routes/request"
	"auth/driver/routes/response"
	"auth/entity"
	"net/http"
)

func (rt Router) login(w http.ResponseWriter, r *http.Request) {
	req := request.NewLoginRequest(r)
	if err := req.Validate(); err != nil {
		response.ResponseBadRequest(w, struct {
			Body    request.LoginRequest `json:"body"`
			Message string               `json:"message"`
		}{req, err.Error()})
	}

	token, err := rt.l.Post(req)
	if err != nil {
		response.ResponseInternalServerError(w, struct {
			Message string `json:"message"`
		}{err.Error()})
	}

	response.ResponseOK(w, struct {
		Token entity.Token `json:"token"`
	}{token})
}

func (rt Router) session(w http.ResponseWriter, r *http.Request) {
	req := request.NewSessionRequest(r)
	if err := req.Validate(); err != nil {
		response.ResponseBadRequest(w, struct {
			Body    request.SessionRequest `json:"body"`
			Message string                 `json:"message"`
		}{req, err.Error()})
	}

	id, err := rt.s.Post(req)
	if err != nil {
		response.ResponseInternalServerError(w, struct {
			Message string `json:"message"`
		}{err.Error()})
	}

	response.ResponseOK(w, struct {
		Id entity.SessionID `json:"session_id"`
	}{id})
}
