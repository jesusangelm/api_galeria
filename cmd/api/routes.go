package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

// Routes, fileserver and middleware setup
func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	// Routes definition
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	// Dynamic middleware managed by alice with some custom middlewares
	dynamic := alice.New() // Auth and similars middleware here

	// Categories routes
	router.Handler(http.MethodGet, "/v1/categories", dynamic.ThenFunc(app.listCategories))
	router.Handler(http.MethodPost, "/v1/categories", dynamic.ThenFunc(app.createCategory))
	router.Handler(http.MethodGet, "/v1/categories/:id", dynamic.ThenFunc(app.showCategory))
	router.Handler(http.MethodPatch, "/v1/categories/:id", dynamic.ThenFunc(app.updateCategory))
	router.Handler(http.MethodDelete, "/v1/categories/:id", dynamic.ThenFunc(app.deleteCategory))
	// Items routes
	router.Handler(http.MethodGet, "/v1/items", dynamic.ThenFunc(app.listItems))
	router.Handler(http.MethodPost, "/v1/items", dynamic.ThenFunc(app.createItem))
	router.Handler(http.MethodPost, "/v1/items_multipart", dynamic.ThenFunc(app.multipartCreateItem))
	router.Handler(http.MethodGet, "/v1/items/:id", dynamic.ThenFunc(app.showItem))
	router.Handler(http.MethodPatch, "/v1/items/:id", dynamic.ThenFunc(app.updateItem))
	router.Handler(http.MethodDelete, "/v1/items/:id", dynamic.ThenFunc(app.deleteItem))

	// Standard middleware managed by alice with some custom middlewares
	standard := alice.New(app.recoverPanic, app.enableCORS, app.rateLimit)

	return standard.Then(router)
}
