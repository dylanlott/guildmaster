package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

func TestAdminUsersMemberCannotView(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	_ = mustCreateUser(t, dbStore, "owner", "Owner")
	member := mustCreateUser(t, dbStore, "member", "Member")
	sessionToken := mustCreateSession(t, dbStore, member.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(http.HandlerFunc(srv.HandleAdminUsers))

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected member GET 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminUsersAllowsAdminViewButRejectsAdminRoleMutation(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	_ = mustCreateUser(t, dbStore, "owner", "Owner")
	admin := mustCreateUser(t, dbStore, "admin", "Admin")
	member := mustCreateUser(t, dbStore, "member", "Member")
	if err := dbStore.UpdateUserRole(admin.ID, roleAdmin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	sessionToken := mustCreateSession(t, dbStore, admin.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(http.HandlerFunc(srv.HandleAdminUsers))

	getReq := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	getReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected admin GET 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	form := url.Values{
		"csrf_token": {"csrf-admin"},
		"user_id":    {strconv.FormatInt(member.ID, 10)},
		"role":       {roleAdmin},
	}
	postReq := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	postReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-admin"})
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusForbidden {
		t.Fatalf("expected admin POST 403, got %d body=%s", postRec.Code, postRec.Body.String())
	}
}

func TestAdminUsersOwnerCanMutateRole(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	member := mustCreateUser(t, dbStore, "member", "Member")
	sessionToken := mustCreateSession(t, dbStore, owner.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(http.HandlerFunc(srv.HandleAdminUsers))

	form := url.Values{
		"csrf_token": {"csrf-owner"},
		"user_id":    {strconv.FormatInt(member.ID, 10)},
		"role":       {roleAdmin},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-owner"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected owner POST 303, got %d body=%s", rec.Code, rec.Body.String())
	}

	reloaded, err := dbStore.GetUserByID(member.ID)
	if err != nil {
		t.Fatalf("reload member: %v", err)
	}
	if reloaded.Role != roleAdmin {
		t.Fatalf("expected member promoted to admin, got %q", reloaded.Role)
	}
}

func TestAdminUsersRejectsInvalidRole(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	member := mustCreateUser(t, dbStore, "member", "Member")
	sessionToken := mustCreateSession(t, dbStore, owner.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(http.HandlerFunc(srv.HandleAdminUsers))

	form := url.Values{
		"csrf_token": {"csrf-owner"},
		"user_id":    {strconv.FormatInt(member.ID, 10)},
		"role":       {"superadmin"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-owner"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected invalid role POST 200 with rendered error, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Invalid role") {
		t.Fatalf("expected invalid role message, got body=%s", rec.Body.String())
	}

	reloaded, err := dbStore.GetUserByID(member.ID)
	if err != nil {
		t.Fatalf("reload member: %v", err)
	}
	if reloaded.Role != roleMember {
		t.Fatalf("expected member role unchanged, got %q", reloaded.Role)
	}
}
