package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/Pjt727/classy/data"
	logginghelpers "github.com/Pjt727/classy/data/logging-helpers"
	serverget "github.com/Pjt727/classy/server/get"
	servermanage "github.com/Pjt727/classy/server/manage"
	serversync "github.com/Pjt727/classy/server/sync"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func Serve() {
	r := chi.NewRouter()
	cors := cors.New(cors.Options{
		// Allow the github page to make to make requests for when running locally
		AllowedOrigins:   []string{"https://pjt727.github.io"},
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
		slog.Error("Fatal cannot connect to main db", "err", err)
		return
	}

	baseLogger := slog.New(logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: false,
		Level:     slog.LevelInfo,
		NoColor:   false,
	}))

	r.Route("/get", func(r chi.Router) {
		serverget.PopulateGetRoutes(&r, dbPool, *baseLogger)
	})
	r.Route("/sync", func(r chi.Router) {
		serversync.PopulateSyncRoutes(&r, dbPool, *baseLogger)
	})

	fileServer(r, "/static", http.Dir(filepath.Join(projectpath.Root, "server", "static")))

	dbTestPool, err := data.NewPool(context.Background(), true)
	if err != nil {
		panic(fmt.Sprintf("Cannot connect to test db %v", err))
	}
	r.Route("/manage", func(r chi.Router) {
		servermanage.PopulateManagementRoutes(&r, dbPool, dbTestPool, *baseLogger)
	})
	port := 3000
	slog.Info("Running server on", "port", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)

	// continously look for collections to collect
	scheduler := collection.NewScheduler(dbPool)
	go func() {
		for {
			// the poll function takes care of waiting to not overwhelm the db
			_, err := scheduler.PollForCollections(context.Background())
			if err != nil {
				baseLogger.Error("Scheduled collection error", "error", err)
			}
		}
	}()

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
