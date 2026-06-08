package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dylanlott/guildmaster/internal/config"
	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

func TestPodAccessIsMemberOnlyAndDirectJoinIsRemoved(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	outsider := mustCreateUser(t, dbStore, "outsider", "Outsider")
	pod := mustCreatePodWithMember(t, dbStore, owner, "private-pod", "Private Pod", "owner")
	outsiderSession := mustCreateSession(t, dbStore, outsider.ID)

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newPodMux(srv))

	req := httptest.NewRequest(http.MethodGet, "/pods/"+pod.Slug, nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: outsiderSession})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected non-member GET /pods/:slug to return 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	joinReq := httptest.NewRequest(http.MethodPost, "/pods/"+pod.Slug+"/join", strings.NewReader("csrf_token=csrf-outsider"))
	joinReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	joinReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: outsiderSession})
	joinReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-outsider"})
	joinRec := httptest.NewRecorder()
	handler.ServeHTTP(joinRec, joinReq)
	if joinRec.Code != http.StatusNotFound {
		t.Fatalf("expected direct join to return 404, got %d body=%s", joinRec.Code, joinRec.Body.String())
	}
}

func TestPodInviteManagementRequiresOwnerOrAdmin(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	member := mustCreateUser(t, dbStore, "member", "Member")
	pod := mustCreatePodWithMember(t, dbStore, owner, "invite-admin", "Invite Admin", "owner")
	if err := dbStore.AddPodMember(pod.ID, member.ID, "member"); err != nil {
		t.Fatalf("AddPodMember(member) failed: %v", err)
	}

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newPodMux(srv))

	ownerReq := httptest.NewRequest(http.MethodGet, "/pods/"+pod.Slug+"/invites", nil)
	ownerReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustCreateSession(t, dbStore, owner.ID)})
	ownerRec := httptest.NewRecorder()
	handler.ServeHTTP(ownerRec, ownerReq)
	if ownerRec.Code != http.StatusOK {
		t.Fatalf("expected owner invite page 200, got %d body=%s", ownerRec.Code, ownerRec.Body.String())
	}

	memberReq := httptest.NewRequest(http.MethodGet, "/pods/"+pod.Slug+"/invites", nil)
	memberReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustCreateSession(t, dbStore, member.ID)})
	memberRec := httptest.NewRecorder()
	handler.ServeHTTP(memberRec, memberReq)
	if memberRec.Code != http.StatusForbidden {
		t.Fatalf("expected member invite page 403, got %d body=%s", memberRec.Code, memberRec.Body.String())
	}
}

func TestInviteAcceptFlowCreatesPodMembership(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	alice := mustCreateUser(t, dbStore, "alice", "Alice")
	pod := mustCreatePodWithMember(t, dbStore, owner, "accept-invite", "Accept Invite", "owner")
	maxUses := 1
	invite, err := dbStore.CreatePodInvite(pod.ID, "accept-token", owner.ID, nil, &maxUses, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreatePodInvite() failed: %v", err)
	}

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newPodMux(srv))

	getReq := httptest.NewRequest(http.MethodGet, "/join/"+invite.Token, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected anonymous invite page 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), "Accept Invite") {
		t.Fatalf("expected invite page to include pod name, body=%s", getRec.Body.String())
	}

	body := strings.NewReader("csrf_token=csrf-alice")
	postReq := httptest.NewRequest(http.MethodPost, "/join/"+invite.Token, body)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustCreateSession(t, dbStore, alice.ID)})
	postReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-alice"})
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusSeeOther {
		t.Fatalf("expected invite accept redirect, got %d body=%s", postRec.Code, postRec.Body.String())
	}
	if postRec.Header().Get("Location") != "/pods/"+pod.Slug {
		t.Fatalf("expected redirect to pod, got %q", postRec.Header().Get("Location"))
	}
	isMember, err := dbStore.IsPodMember(pod.ID, alice.ID)
	if err != nil {
		t.Fatalf("IsPodMember(alice) failed: %v", err)
	}
	if !isMember {
		t.Fatalf("expected alice to become a pod member")
	}
}

func TestPodSubmitRejectsNonMemberParticipant(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	alice := mustCreateUser(t, dbStore, "alice", "Alice")
	pod := mustCreatePodWithMember(t, dbStore, owner, "submit-members", "Submit Members", "owner")
	if err := dbStore.AddPodMember(pod.ID, alice.ID, "member"); err != nil {
		t.Fatalf("AddPodMember(alice) failed: %v", err)
	}

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newPodMux(srv))

	form := url.Values{
		"csrf_token": {"csrf-owner"},
		"placement":  {"Owner", "Guest"},
	}
	req := httptest.NewRequest(http.MethodPost, "/pods/"+pod.Slug+"/submit/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustCreateSession(t, dbStore, owner.ID)})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-owner"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected non-member participant rejection, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "all players in a pod game must be current pod members") {
		t.Fatalf("unexpected rejection body: %s", rec.Body.String())
	}

	validForm := url.Values{
		"csrf_token": {"csrf-owner"},
		"placement":  {"Owner", "Alice"},
	}
	validReq := httptest.NewRequest(http.MethodPost, "/pods/"+pod.Slug+"/submit/create", strings.NewReader(validForm.Encode()))
	validReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustCreateSession(t, dbStore, owner.ID)})
	validReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-owner"})
	validRec := httptest.NewRecorder()
	handler.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusSeeOther {
		t.Fatalf("expected member-only submission redirect, got %d body=%s", validRec.Code, validRec.Body.String())
	}
}

func TestPodCreatorAndJoinedMemberCanSubmitToPodHistory(t *testing.T) {
	dbStore := mustNewInMemoryDB(t)
	defer func() { _ = dbStore.Close() }()

	owner := mustCreateUser(t, dbStore, "owner", "Owner")
	member := mustCreateUser(t, dbStore, "member", "Member")
	pod := mustCreatePodWithMember(t, dbStore, owner, "history-submit", "History Submit", "owner")
	maxUses := 1
	invite, err := dbStore.CreatePodInvite(pod.ID, "history-submit-token", owner.ID, &member.ID, &maxUses, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreatePodInvite() failed: %v", err)
	}

	srv := New(scoring.NewStore(), dbStore, config.Config{})
	handler := srv.SessionMiddleware(newPodMux(srv))
	ownerSession := mustCreateSession(t, dbStore, owner.ID)
	memberSession := mustCreateSession(t, dbStore, member.ID)

	ownerGetReq := httptest.NewRequest(http.MethodGet, "/pods/"+pod.Slug+"/submit", nil)
	ownerGetReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ownerSession})
	ownerGetRec := httptest.NewRecorder()
	handler.ServeHTTP(ownerGetRec, ownerGetReq)
	if ownerGetRec.Code != http.StatusOK {
		t.Fatalf("expected owner submit page 200, got %d body=%s", ownerGetRec.Code, ownerGetRec.Body.String())
	}

	joinBody := strings.NewReader("csrf_token=csrf-member")
	joinReq := httptest.NewRequest(http.MethodPost, "/join/"+invite.Token, joinBody)
	joinReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	joinReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: memberSession})
	joinReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-member"})
	joinRec := httptest.NewRecorder()
	handler.ServeHTTP(joinRec, joinReq)
	if joinRec.Code != http.StatusSeeOther {
		t.Fatalf("expected invite accept redirect, got %d body=%s", joinRec.Code, joinRec.Body.String())
	}

	memberGetReq := httptest.NewRequest(http.MethodGet, "/pods/"+pod.Slug+"/submit", nil)
	memberGetReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: memberSession})
	memberGetRec := httptest.NewRecorder()
	handler.ServeHTTP(memberGetRec, memberGetReq)
	if memberGetRec.Code != http.StatusOK {
		t.Fatalf("expected joined member submit page 200, got %d body=%s", memberGetRec.Code, memberGetRec.Body.String())
	}
	if !strings.Contains(memberGetRec.Body.String(), "Submit Game to "+pod.Name) {
		t.Fatalf("expected joined member submit page body, got %s", memberGetRec.Body.String())
	}

	memberForm := url.Values{
		"csrf_token": {"csrf-member"},
		"placement":  {"Owner", "Member"},
		"notes":      {"Joined member submitted this pod game."},
	}
	memberPostReq := httptest.NewRequest(http.MethodPost, "/pods/"+pod.Slug+"/submit/create", strings.NewReader(memberForm.Encode()))
	memberPostReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	memberPostReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: memberSession})
	memberPostReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-member"})
	memberPostRec := httptest.NewRecorder()
	handler.ServeHTTP(memberPostRec, memberPostReq)
	if memberPostRec.Code != http.StatusSeeOther {
		t.Fatalf("expected joined member submit redirect, got %d body=%s", memberPostRec.Code, memberPostRec.Body.String())
	}
	if !strings.HasPrefix(memberPostRec.Header().Get("Location"), "/pods/"+pod.Slug+"/games/") {
		t.Fatalf("expected redirect into pod history, got %q", memberPostRec.Header().Get("Location"))
	}

	games, total, err := dbStore.ListPodGames(pod.ID, 1, 10)
	if err != nil {
		t.Fatalf("ListPodGames() failed: %v", err)
	}
	if total != 1 || len(games) != 1 {
		t.Fatalf("expected one pod game in history, got total=%d len=%d", total, len(games))
	}
	if games[0].Source != "pod_submit" {
		t.Fatalf("expected pod_submit source, got %q", games[0].Source)
	}
	if len(games[0].Placements) != 2 || games[0].Placements[0].Player != "Owner" || games[0].Placements[1].Player != "Member" {
		t.Fatalf("unexpected pod history placements: %#v", games[0].Placements)
	}

	detailReq := httptest.NewRequest(http.MethodGet, memberPostRec.Header().Get("Location"), nil)
	detailReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: memberSession})
	detailRec := httptest.NewRecorder()
	handler.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected joined member pod history detail 200, got %d body=%s", detailRec.Code, detailRec.Body.String())
	}
	if !strings.Contains(detailRec.Body.String(), "Owner") || !strings.Contains(detailRec.Body.String(), "Member") {
		t.Fatalf("expected pod history detail to include both players, body=%s", detailRec.Body.String())
	}
}

func newPodMux(srv *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/pods/new", srv.RequireAuth(srv.HandlePodNew))
	mux.HandleFunc("/pods/", srv.HandlePods)
	mux.HandleFunc("/pods", srv.HandlePods)
	mux.HandleFunc("/join/", srv.HandleJoinInvite)
	return mux
}

func mustCreatePodWithMember(t *testing.T, dbStore *db.Store, user *db.User, slug, name, role string) *db.Pod {
	t.Helper()
	pod, err := dbStore.CreatePod(user.ID, slug, name, "", "commander", false)
	if err != nil {
		t.Fatalf("CreatePod(%s) failed: %v", slug, err)
	}
	if err := dbStore.AddPodMember(pod.ID, user.ID, role); err != nil {
		t.Fatalf("AddPodMember(%s) failed: %v", role, err)
	}
	return pod
}
