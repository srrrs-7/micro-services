package routes

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"
)

type Router struct {
	l LoginUseCase
	s SessionUseCase
}

func NewRoutes(rt Router) *chi.Mux {
	r := chi.NewMux()

	r.Use(r.Middlewares()...)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {})

	r.Route("/auth/v1", func(r chi.Router) {
		r.Route("/user", func(r chi.Router) {})

		r.Route("/token", func(r chi.Router) {
			r.Post("/login", rt.login)
			r.Post("/logout", rt.login)
			r.Post("/session", rt.session)
		})

		r.Route("/role", func(r chi.Router) {})

		r.Route("/scope", func(r chi.Router) {})
	})

	return r
}

func injectCtx(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			param := chi.URLParam(r, key)
			ctx := context.WithValue(r.Context(), key, param)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
