package server

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/dylanlott/guildmaster/internal/auth"
)

func (s *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if UserFromContext(r.Context()) != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.renderRegisterPage(w, r, "")
	case http.MethodPost:
		s.handleRegisterPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRegisterPost(w http.ResponseWriter, r *http.Request) {
	if UserFromContext(r.Context()) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	if !s.validateCSRF(r) {
		s.renderRegisterPage(w, r, "Your session expired. Refresh and try again.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderRegisterPage(w, r, "Invalid form submission")
		return
	}

	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	if username == "" || displayName == "" || password == "" {
		s.renderRegisterPage(w, r, "Username, display name, and password are required")
		return
	}
	if password != confirm {
		s.renderRegisterPage(w, r, "Passwords do not match")
		return
	}
	if len(password) < 8 {
		s.renderRegisterPage(w, r, "Password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user, err := s.dbStore.CreateUser(username, displayName, hash)
	if err != nil {
		s.renderRegisterPage(w, r, "Unable to create account (username may already exist)")
		return
	}

	token, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "failed to create session token", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().UTC().Add(auth.SessionDuration)
	if err := s.dbStore.CreateSession(user.ID, token, expiresAt); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	s.setSessionCookie(w, r, token, expiresAt)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) renderRegisterPage(w http.ResponseWriter, r *http.Request, errMsg string) {
	t, err := template.New("register.tmpl").ParseFS(tmplFS, "register.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Error string
		Nav   navData
	}{
		Error: errMsg,
		Nav:   navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if UserFromContext(r.Context()) != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.renderLoginPage(w, r, "")
	case http.MethodPost:
		s.handleLoginPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if UserFromContext(r.Context()) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	if !s.validateCSRF(r) {
		s.renderLoginPage(w, r, "Your session expired. Refresh and try again.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderLoginPage(w, r, "Invalid form submission")
		return
	}

	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	password := r.FormValue("password")
	if username == "" || password == "" {
		s.renderLoginPage(w, r, "Username and password are required")
		return
	}

	user, err := s.dbStore.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "failed to load user", http.StatusInternalServerError)
		return
	}
	if user == nil || !auth.CheckPassword(user.PasswordHash, password) {
		s.renderLoginPage(w, r, "Invalid username or password")
		return
	}

	token, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "failed to create session token", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().UTC().Add(auth.SessionDuration)
	if err := s.dbStore.CreateSession(user.ID, token, expiresAt); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	s.setSessionCookie(w, r, token, expiresAt)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) renderLoginPage(w http.ResponseWriter, r *http.Request, errMsg string) {
	t, err := template.New("login.tmpl").ParseFS(tmplFS, "login.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Error string
		Nav   navData
	}{
		Error: errMsg,
		Nav:   navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" && s.dbStore != nil {
		_ = s.dbStore.DeleteSession(cookie.Value)
	}
	s.clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
