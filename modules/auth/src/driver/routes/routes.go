package routes

import (
	"net/http"

	"github.com/go-chi/chi"
)

func NewRoutes() *chi.Mux {
	r := chi.NewMux()

	r.Use(r.Middlewares()...)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {})

	return r
}
