package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

type API struct {
	Router chi.Router
	Huma   huma.API
}

func NewAPI() *API {
	router := chi.NewMux()
	config := huma.DefaultConfig("Rule Manager API", "1.0.0")
	humaAPI := humachi.New(router, config)

	return &API{
		Router: router,
		Huma:   humaAPI,
	}
}

func (a *API) Start(addr string) error {
	return http.ListenAndServe(addr, a.Router)
}
