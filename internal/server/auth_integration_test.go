package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dylanlott/guildmaster/internal/auth"
	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

func TestAPIAllowsOwnerSessionWithCSRF(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	sessionToken := mustCreateSession(t, dbStore, owner.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newAPIMux(srv))

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewBufferString(`{"rankings":["A","B"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-owner"})
	req.Header.Set("X-CSRF-Token", "csrf-owner")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIRejectsSessionAdminWithoutCSRF(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	sessionToken := mustCreateSession(t, dbStore, owner.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newAPIMux(srv))

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewBufferString(`{"rankings":["A","B"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-owner"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitCreateRejectsNonAdminMember(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	_ = mustCreateUser(t, dbStore, "owner", "Owner")
	member := mustCreateUser(t, dbStore, "member", "Member")
	sessionToken := mustCreateSession(t, dbStore, member.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(http.HandlerFunc(srv.HandleSubmitCreate))

	body := bytes.NewBufferString("csrf_token=csrf-member&placement=Alice&placement=Bob")
	req := httptest.NewRequest(http.MethodPost, "/submit/create", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-member"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func mustCreateUser(t *testing.T, dbStore *db.Store, username, displayName string) *db.User {
	t.Helper()
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := dbStore.CreateUser(username, displayName, hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func mustCreateSession(t *testing.T, dbStore *db.Store, userID int64) string {
	t.Helper()
	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := dbStore.CreateSession(userID, token, time.Now().UTC().Add(auth.SessionDuration)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}
