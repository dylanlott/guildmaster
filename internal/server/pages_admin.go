package server

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/dylanlott/guildmaster/internal/db"
)

func (s *Server) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderAdminUsersPage(w, r, "")
	case http.MethodPost:
		s.handleAdminUsersPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminUsersPost(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if normalizeRole(user.Role) != roleOwner {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !s.validateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderAdminUsersPage(w, r, "Invalid form submission")
		return
	}

	userID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		s.renderAdminUsersPage(w, r, "Invalid user")
		return
	}
	role := strings.TrimSpace(r.FormValue("role"))
	switch role {
	case roleMember, roleAdmin, roleOwner:
	default:
		s.renderAdminUsersPage(w, r, "Invalid role")
		return
	}
	if err := s.dbStore.UpdateUserRole(userID, role); err != nil {
		if errors.Is(err, db.ErrLastOwner) {
			s.renderAdminUsersPage(w, r, "Guildmaster needs at least one owner.")
			return
		}
		http.Error(w, "failed to update user role", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) renderAdminUsersPage(w http.ResponseWriter, r *http.Request, errMsg string) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	role := normalizeRole(user.Role)
	if role != roleOwner && role != roleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}

	users, err := s.dbStore.ListUsers()
	if err != nil {
		http.Error(w, "failed to load users", http.StatusInternalServerError)
		return
	}

	t, err := template.New("admin_users.tmpl").ParseFS(tmplFS, "admin_users.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Error     string
		Nav       navData
		Users     []db.User
		CanManage bool
	}{
		Error:     errMsg,
		Nav:       navForRequest(r),
		Users:     users,
		CanManage: role == roleOwner,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}
