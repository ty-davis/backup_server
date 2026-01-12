package handlers

import (
	"backup_server/internal/auth"
	"context"
	"net/http"
)

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := auth.GetSessionFromRequest(r)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		session, exists := h.Sessions.Get(sessionID)
		if !exists {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), "session", session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
