package route

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"shared/utilotel"
)

type handler struct {
	loginSvc loginService
}

func NewHandler(svc loginService) handler {
	return handler{loginSvc: svc}
}

func (h *handler) Router() *chi.Mux {
	r := chi.NewMux()

	r.Use(r.Middlewares()...)
	r.Use(utilotel.HTTPMiddleware("auth-api"))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {})

	r.Route("/auth/v1", func(r chi.Router) {
		r.Route("/token", func(r chi.Router) {
			r.Post("/login", h.login)
		})
	})

	return r
}
