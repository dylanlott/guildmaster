package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/dylanlott/guildmaster/internal/auth"
	"github.com/dylanlott/guildmaster/internal/db"
)

const (
	sessionCookieName = "session"
	csrfCookieName    = "guildmaster_csrf"
	roleMember        = "member"
	roleAdmin         = "admin"
	roleOwner         = "owner"
)

func (s *Server) adminKey() string {
	return strings.TrimSpace(s.cfg.GuildmasterAdminKey)
}

func (s *Server) hasValidAdminBearer(r *http.Request) bool {
	key := s.adminKey()
	if key == "" {
		return false
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	return subtle.ConstantTimeCompare([]byte(provided), []byte(key)) == 1
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case roleOwner:
		return roleOwner
	case roleAdmin:
		return roleAdmin
	default:
		return roleMember
	}
}

func (s *Server) userCanAdmin(user *db.User) bool {
	if user == nil {
		return false
	}
	switch normalizeRole(user.Role) {
	case roleOwner, roleAdmin:
		return true
	default:
		return false
	}
}

func requestIsSecure(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r == nil {
		return false
	}
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if proto == "https" {
		return true
	}
	for _, part := range strings.Split(proto, ",") {
		if strings.TrimSpace(part) == "https" {
			return true
		}
	}
	return false
}

func generateCSRFCookieToken() (string, error) {
	return auth.GenerateToken()
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   maxAge,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil {
		value := strings.TrimSpace(cookie.Value)
		if value != "" {
			return value, nil
		}
	}

	token, err := generateCSRFCookieToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(12 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
	return token, nil
}

func (s *Server) validateCSRF(r *http.Request) bool {
	if r == nil {
		return false
	}
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	cookieToken := strings.TrimSpace(cookie.Value)
	if cookieToken == "" {
		return false
	}
	provided := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if provided == "" {
		provided = strings.TrimSpace(r.FormValue("csrf_token"))
	}
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(provided)) == 1
}

func (s *Server) authorizeAdminRequest(w http.ResponseWriter, r *http.Request) bool {
	if s.hasValidAdminBearer(r) {
		return true
	}
	user := UserFromContext(r.Context())
	if !s.userCanAdmin(user) {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if !s.validateCSRF(r) {
		writeAPIError(w, http.StatusForbidden, "invalid csrf token")
		return false
	}
	return true
}

// RequireAdmin allows either a privileged session user or Authorization: Bearer <key>.
// Session-authenticated POSTs must include a valid CSRF token.
func (s *Server) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminKey() == "" && !s.userCanAdmin(UserFromContext(r.Context())) {
			writeAPIError(w, http.StatusForbidden, "admin auth not configured")
			return
		}
		if !s.authorizeAdminRequest(w, r) {
			return
		}
		next(w, r)
	}
}
