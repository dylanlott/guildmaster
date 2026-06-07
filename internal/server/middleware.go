package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/dylanlott/guildmaster/internal/db"
)

type contextKey string

const (
	userContextKey contextKey = "session_user"
	csrfContextKey contextKey = "csrf_token"
)

func UserFromContext(ctx context.Context) *db.User {
	if ctx == nil {
		return nil
	}
	user, _ := ctx.Value(userContextKey).(*db.User)
	return user
}

func csrfTokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	token, _ := ctx.Value(csrfContextKey).(string)
	return token
}

func (s *Server) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		csrfToken, err := s.ensureCSRFCookie(w, r)
		if err == nil && csrfToken != "" {
			ctx = context.WithValue(ctx, csrfContextKey, csrfToken)
		}

		if s.dbStore != nil {
			cookie, err := r.Cookie(sessionCookieName)
			if err == nil && strings.TrimSpace(cookie.Value) != "" {
				user, lookupErr := s.dbStore.GetSessionUser(cookie.Value)
				if lookupErr == nil && user != nil {
					ctx = context.WithValue(ctx, userContextKey, user)
				}
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}
