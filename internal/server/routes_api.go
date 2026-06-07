package server

import "net/http"

// RegisterAPIRoutes wires all public API endpoints.
func (s *Server) RegisterAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/scores", s.HandleGetScores)
	mux.HandleFunc("/api/players/", s.HandleGetPlayer)
	mux.HandleFunc("/api/games", s.requireAdminForPost(s.HandleGames))
	mux.HandleFunc("/api/games/", s.HandleGetGameByID)
	mux.HandleFunc("/api/refresh", s.requireAdminForPost(s.HandleRefresh))
	mux.HandleFunc("/api/", s.HandleAPINotFound)
}

func (s *Server) requireAdminForPost(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.RequireAdmin(next)(w, r)
			return
		}
		next(w, r)
	}
}
