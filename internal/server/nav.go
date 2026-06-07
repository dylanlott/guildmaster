package server

import "net/http"

type navData struct {
	LoggedIn  bool
	Username  string
	Role      string
	CanAdmin  bool
	CSRFToken string
}

func navForRequest(r *http.Request) navData {
	nav := navData{
		CSRFToken: csrfTokenFromContext(r.Context()),
	}
	user := UserFromContext(r.Context())
	if user == nil {
		return nav
	}
	nav.LoggedIn = true
	nav.Username = user.Username
	nav.Role = normalizeRole(user.Role)
	nav.CanAdmin = nav.Role == roleOwner || nav.Role == roleAdmin
	return nav
}
