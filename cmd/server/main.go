package main

import (
	"backup_server/internal/auth"
	"backup_server/internal/database"
	"backup_server/internal/handlers"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	db, err := database.InitDB("backup_server.db")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	sessions := auth.NewSessionStore()
	handler := handlers.NewHandler(db, sessions)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", handler.LoginPage)
	r.Post("/login", handler.Login)
	r.Get("/logout", handler.Logout)

	fileServer := http.FileServer(http.Dir("static/terramap"))
	r.Handle("/terramap/*", http.StripPrefix("/terramap", fileServer))

	r.Group(func(r chi.Router) {
		r.Use(handler.AuthMiddleware)
		r.Get("/files", handler.FilesPage)
		r.Get("/download", handler.DownloadFile)
		r.Get("/worldfile", handler.ServeWorldFile)
		r.Get("/viewer/terramap", handler.TerraMapViewer)
		r.Get("/admin/files", handler.AdminPage)
		r.Post("/admin/files/add", handler.AdminAddFile)
		r.Post("/admin/files/edit", handler.AdminEditFile)
		r.Post("/admin/files/delete", handler.AdminDeleteFile)
		r.Get("/admin/users", handler.AdminUsersPage)
		r.Post("/admin/users/add", handler.AdminAddUser)
		r.Post("/admin/users/edit", handler.AdminEditUser)
		r.Post("/admin/users/password", handler.AdminChangeUserPassword)
		r.Post("/admin/users/delete", handler.AdminDeleteUser)
		r.Get("/admin/groups", handler.AdminGroupsPage)
		r.Post("/admin/groups/add", handler.AdminAddGroup)
		r.Post("/admin/groups/edit", handler.AdminEditGroup)
		r.Post("/admin/groups/delete", handler.AdminDeleteGroup)
	})

	log.Println("Server starting on http://localhost:8090")
	log.Fatal(http.ListenAndServe(":8090", r))
}
