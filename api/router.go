package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// API represents the main API structure.
type API struct {
	Router chi.Router
	Huma   huma.API
}

// NewAPI creates a new API instance.
func NewAPI() *API {
	router := chi.NewMux()
	config := huma.DefaultConfig("Rule Manager API", "1.0.0")
	humaAPI := humachi.New(router, config)

	return &API{
		Router: router,
		Huma:   humaAPI,
	}
}

// Start starts the API server on the given address.
func (a *API) Start(addr string) error {
	return http.ListenAndServe(addr, a.Router)
}
