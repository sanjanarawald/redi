package routes

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"redi/config"
	"redi/handlers"
	rediauth "redi/middleware"
)

func Setup(cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.Use(handlers.WithConfig(cfg))

	// Serve UI first so / and /index.html match before /api
	webDir := "web"
	if exe, err := os.Executable(); err == nil {
		webDir = filepath.Join(filepath.Dir(exe), "web")
	}
	indexPath := filepath.Join(webDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		// Fallback for "go run" when exe is in temp dir
		indexPath = filepath.Join("web", "index.html")
	}
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, indexPath)
	})
	r.Get("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, indexPath)
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/register", handlers.Register)
		r.Post("/login", handlers.Login)
		r.Get("/img/post/{id}", handlers.GetPostImage)

		r.Group(func(r chi.Router) {
			r.Use(rediauth.JWTAuth(cfg.JWTSecret))
			r.Get("/me", handlers.Me)
			r.Get("/me/saved", handlers.GetSavedPosts)

			r.Route("/posts", func(r chi.Router) {
				r.Post("/", handlers.CreatePost)
				r.Get("/", handlers.GetFeed)
				r.Get("/{id}", handlers.GetPost)
				r.Patch("/{id}", handlers.UpdatePost)
				r.Delete("/{id}", handlers.DeletePost)
				r.Post("/{id}/like", handlers.LikePost)
				r.Delete("/{id}/like", handlers.UnlikePost)
				r.Post("/{id}/save", handlers.SavePost)
				r.Delete("/{id}/save", handlers.UnsavePost)
				r.Get("/{id}/comments", handlers.GetComments)
				r.Post("/{id}/comments", handlers.CreateComment)
			})

			r.Route("/comments", func(r chi.Router) {
				r.Post("/{id}/reply", handlers.ReplyToComment)
				r.Post("/{id}/like", handlers.LikeComment)
				r.Delete("/{id}/like", handlers.UnlikeComment)
			})
		})
	})

	return r
}
