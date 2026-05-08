package route

import (
	"net/http"

	"github.com/go-chi/chi"

	"auth/route/middleware"
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
	r.Use(middleware.RouteTag())

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {})

	r.Route("/auth/v1", func(r chi.Router) {
		r.Route("/token", func(r chi.Router) {
			r.Post("/login", h.login)
		})
	})

	return r
}
