package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/Pjt727/classy/data"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	log "github.com/sirupsen/logrus"
)

func Serve() {
	r := chi.NewRouter()
	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum age for preflight requests
	})
	r.Use(cors.Handler)
	r.Use(middleware.Logger)

	dbPool, err := data.NewPool(context.Background(), false)
	if err != nil {
		log.Error("Fatal cannot connect to main db: ", err)
		return
	}

	r.Route("/get", func(r chi.Router) {
		populateGetRoutes(&r, dbPool)
	})
	r.Route("/sync", func(r chi.Router) {
		populateSyncRoutes(&r, dbPool)
	})

	fileServer(r, "/static", http.Dir(filepath.Join(projectpath.Root, "api", "static")))

	dbTestPool, err := data.NewPool(context.Background(), true)
	if err != nil {
		log.Warn("Cannot connect to test db: ", err)
		return
	}
	r.Route("/manage", func(r chi.Router) {
		populateManagementRoutes(&r, dbPool, dbTestPool)
	})
	port := 3000
	log.Infof("Running server on :%d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

// https://github.com/go-chi/chi/blob/master/_examples/fileserver/main.go
func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
