package route

import (
	"auth/domain"
	"auth/route/request"
	"net/http"
	"shared/utilhttp"
)

type response struct {
	Token domain.Token `json:"token"`
}

func newResponse(token domain.Token) response {
	return response{Token: token}
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	req, err := utilhttp.RequestBody[request.LoginRequest](r)
	if err != nil {
		utilhttp.ResponseError(w, err)
		return
	}

	if err := req.Validate(); err != nil {
		utilhttp.ResponseError(w, err)
		return
	}

	token, err := h.loginSvc.Post(r.Context(), domain.NewLoginInput(req.Email, req.Password))
	if err != nil {
		utilhttp.ResponseError(w, err)
		return
	}

	utilhttp.ResponseOk(w, newResponse(token))
}
