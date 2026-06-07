package server

import (
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dylanlott/guildmaster/internal/db"
	"github.com/dylanlott/guildmaster/internal/scoring"
)

var podSlugPattern = regexp.MustCompile(`^[a-z0-9-]{3,50}$`)
var supportedPodFormats = []string{"commander", "standard", "draft", "sealed", "pioneer", "modern", "legacy", "custom"}

type podSummaryRow struct {
	Slug        string
	Name        string
	Description string
	Public      bool
}

type podGameDetailPlayerRow struct {
	Placement            int
	Player               string
	EloBefore            int
	Delta                int
	EloAfter             int
	DeckName             string
	FinalLife            string
	Eliminations         int
	CommanderDamageDealt int
	CommanderDamageTaken int
	TurnEliminated       string
}

func isValidPodSlug(slug string) bool {
	return podSlugPattern.MatchString(slug)
}

func isValidPodFormat(format string) bool {
	for _, candidate := range supportedPodFormats {
		if format == candidate {
			return true
		}
	}
	return false
}

func (s *Server) HandlePodNew(w http.ResponseWriter, r *http.Request) {
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	if r.URL.Path != "/pods/new" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.renderPodNewPage(w, r, "")
	case http.MethodPost:
		s.handlePodNewPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) renderPodNewPage(w http.ResponseWriter, r *http.Request, errMsg string) {
	t, err := template.New("pods_new.tmpl").ParseFS(tmplFS, "pods_new.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Error      string
		PodFormats []string
		Nav        navData
	}{
		Error:      errMsg,
		PodFormats: supportedPodFormats,
		Nav:        navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePodNewPost(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.validateCSRF(r) {
		s.renderPodNewPage(w, r, "Your session expired. Refresh and try again.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderPodNewPage(w, r, "Invalid form submission")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.ToLower(strings.TrimSpace(r.FormValue("slug")))
	description := strings.TrimSpace(r.FormValue("description"))
	format := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	isPublic := r.FormValue("visibility") != "private"

	if name == "" || slug == "" {
		s.renderPodNewPage(w, r, "Name and slug are required")
		return
	}
	if !isValidPodSlug(slug) {
		s.renderPodNewPage(w, r, "Slug must be 3-50 chars: lowercase letters, numbers, and hyphens")
		return
	}
	if format == "" {
		format = "commander"
	}
	if !isValidPodFormat(format) {
		s.renderPodNewPage(w, r, "Invalid format selected")
		return
	}

	pod, err := s.dbStore.CreatePod(user.ID, slug, name, description, format, isPublic)
	if err != nil {
		s.renderPodNewPage(w, r, "Unable to create pod (slug may already exist)")
		return
	}
	if err := s.dbStore.AddPodMember(pod.ID, user.ID, "owner"); err != nil {
		http.Error(w, "failed to create pod membership: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/pods/"+url.PathEscape(pod.Slug), http.StatusSeeOther)
}

func (s *Server) HandlePods(w http.ResponseWriter, r *http.Request) {
	if s.dbStore == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}

	if r.URL.Path == "/pods" {
		s.handlePodsListPage(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/pods/") {
		http.NotFound(w, r)
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/pods/")
	if strings.TrimSpace(rest) == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(rest, "/")
	rawSlug := strings.TrimSpace(parts[0])
	if rawSlug == "" {
		http.NotFound(w, r)
		return
	}
	slug, err := url.PathUnescape(rawSlug)
	if err != nil || !isValidPodSlug(slug) {
		http.NotFound(w, r)
		return
	}

	switch len(parts) {
	case 1:
		s.handlePodLeaderboardPage(w, r, slug)
		return
	case 2:
		switch parts[1] {
		case "games":
			s.handlePodGamesPage(w, r, slug)
			return
		case "join":
			s.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
				s.handlePodJoin(w, r, slug)
			})(w, r)
			return
		case "submit":
			s.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
				s.handlePodSubmitPage(w, r, slug)
			})(w, r)
			return
		}
	case 3:
		switch parts[1] {
		case "games":
			s.handlePodGameDetailPage(w, r, slug, parts[2])
			return
		case "submit":
			if parts[2] == "create" {
				s.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
					s.handlePodSubmitCreate(w, r, slug)
				})(w, r)
				return
			}
		}
	}

	http.NotFound(w, r)
}

func (s *Server) handlePodsListPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publicPods, err := s.dbStore.ListPublicPods()
	if err != nil {
		http.Error(w, "failed to list public pods: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := UserFromContext(r.Context())
	userPods := make([]*db.Pod, 0)
	if user != nil {
		userPods, err = s.dbStore.ListUserPods(user.ID)
		if err != nil {
			http.Error(w, "failed to list your pods: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	seen := make(map[int64]struct{}, len(userPods))
	userRows := make([]podSummaryRow, 0, len(userPods))
	for _, pod := range userPods {
		seen[pod.ID] = struct{}{}
		userRows = append(userRows, podSummaryRow{Slug: pod.Slug, Name: pod.Name, Description: pod.Description, Public: pod.Public})
	}

	publicRows := make([]podSummaryRow, 0, len(publicPods))
	for _, pod := range publicPods {
		if _, ok := seen[pod.ID]; ok {
			continue
		}
		publicRows = append(publicRows, podSummaryRow{Slug: pod.Slug, Name: pod.Name, Description: pod.Description, Public: pod.Public})
	}

	t, err := template.New("pods.tmpl").ParseFS(tmplFS, "pods.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		UserPods   []podSummaryRow
		PublicPods []podSummaryRow
		Nav        navData
	}{
		UserPods:   userRows,
		PublicPods: publicRows,
		Nav:        navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePodLeaderboardPage(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pod, userMember, role, ok := s.loadAccessiblePod(w, r, slug)
	if !ok {
		return
	}

	leaderboard, err := s.dbStore.GetPodLeaderboard(pod.ID)
	if err != nil {
		http.Error(w, "failed to load pod leaderboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	memberCount, err := s.dbStore.CountPodMembers(pod.ID)
	if err != nil {
		http.Error(w, "failed to load member count: "+err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := template.New("pod_detail.tmpl").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}).ParseFS(tmplFS, "pod_detail.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Pod          *db.Pod
		Leaderboard  []db.PodLeaderboardRow
		MemberCount  int
		IsMember     bool
		CanJoin      bool
		CanSubmit    bool
		CanConfigure bool
		Nav          navData
	}{
		Pod:          pod,
		Leaderboard:  leaderboard,
		MemberCount:  memberCount,
		IsMember:     userMember,
		CanJoin:      UserFromContext(r.Context()) != nil && !userMember && pod.Public,
		CanSubmit:    userMember,
		CanConfigure: role == "owner" || role == "admin",
		Nav:          navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePodGamesPage(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pod, _, _, ok := s.loadAccessiblePod(w, r, slug)
	if !ok {
		return
	}

	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			http.Error(w, "page must be a positive integer", http.StatusBadRequest)
			return
		}
		page = parsed
	}

	const limit = 25
	games, total, err := s.dbStore.ListPodGames(pod.ID, page, limit)
	if err != nil {
		http.Error(w, "failed to list pod games: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]gameHistoryRow, 0, len(games))
	for _, game := range games {
		players := make([]string, 0, len(game.Placements))
		for _, placement := range game.Placements {
			players = append(players, placement.Player)
		}
		rows = append(rows, gameHistoryRow{
			ID:       game.ID,
			PlayedAt: humanDate(game.PlayedAt),
			Source:   game.Source,
			Players:  strings.Join(players, ", "),
			Count:    len(players),
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		http.NotFound(w, r)
		return
	}

	t, err := template.New("pod_games.tmpl").ParseFS(tmplFS, "pod_games.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Pod        *db.Pod
		Games      []gameHistoryRow
		Page       int
		Total      int
		TotalPages int
		PrevPage   int
		NextPage   int
		HasPrev    bool
		HasNext    bool
		Nav        navData
	}{
		Pod:        pod,
		Games:      rows,
		Page:       page,
		Total:      total,
		TotalPages: totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		Nav:        navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePodGameDetailPage(w http.ResponseWriter, r *http.Request, slug, rawID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pod, _, _, ok := s.loadAccessiblePod(w, r, slug)
	if !ok {
		return
	}

	id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}

	game, err := s.dbStore.GetGameWithStats(id)
	if err != nil {
		http.Error(w, "failed to load game: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if game == nil || game.PodID != pod.ID {
		http.NotFound(w, r)
		return
	}

	podGames, err := s.dbStore.ListPodGamesChronological(pod.ID)
	if err != nil {
		http.Error(w, "failed to load pod games for elo replay: "+err.Error(), http.StatusInternalServerError)
		return
	}

	snapshot := make(map[string]int)
	deltaForPlayer := make(map[string]int)
	beforeForPlayer := make(map[string]int)
	afterForPlayer := make(map[string]int)
	for _, replayGame := range podGames {
		rankings := placementsToRankings(replayGame.Placements)
		for _, player := range rankings {
			if _, ok := snapshot[player]; !ok {
				snapshot[player] = scoring.DefaultStartingScore
			}
		}

		deltas, err := scoring.ScoreGame(rankings, s.K, s.D, snapshot)
		if err != nil {
			http.Error(w, "failed to score replay game: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if replayGame.ID == game.ID {
			for _, player := range rankings {
				beforeForPlayer[player] = snapshot[player]
				deltaForPlayer[player] = deltas[player]
				afterForPlayer[player] = snapshot[player] + deltas[player]
			}
			break
		}
		scoring.ApplyDeltas(snapshot, deltas)
	}

	playerRows := make([]podGameDetailPlayerRow, 0, len(game.Placements))
	for _, placement := range game.Placements {
		finalLife := "-"
		if placement.FinalLife != nil {
			finalLife = strconv.Itoa(*placement.FinalLife)
		}
		turnEliminated := "-"
		if placement.TurnEliminated != nil {
			turnEliminated = strconv.Itoa(*placement.TurnEliminated)
		}
		playerRows = append(playerRows, podGameDetailPlayerRow{
			Placement:            placement.Placement,
			Player:               placement.Player,
			EloBefore:            beforeForPlayer[placement.Player],
			Delta:                deltaForPlayer[placement.Player],
			EloAfter:             afterForPlayer[placement.Player],
			DeckName:             placement.DeckName,
			FinalLife:            finalLife,
			Eliminations:         placement.Eliminations,
			CommanderDamageDealt: placement.CommanderDamageDealt,
			CommanderDamageTaken: placement.CommanderDamageTaken,
			TurnEliminated:       turnEliminated,
		})
	}

	t, err := template.New("pod_game_detail.tmpl").Funcs(template.FuncMap{
		"humanDate": humanDate,
		"deltaClass": func(v int) string {
			switch {
			case v > 0:
				return "pos"
			case v < 0:
				return "neg"
			default:
				return "zero"
			}
		},
	}).ParseFS(tmplFS, "pod_game_detail.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Pod       *db.Pod
		Game      *db.GameWithStats
		TurnCount string
		Notes     string
		Players   []podGameDetailPlayerRow
		Nav       navData
	}{
		Pod:       pod,
		Game:      game,
		TurnCount: "-",
		Notes:     "-",
		Players:   playerRows,
		Nav:       navForRequest(r),
	}
	if game.TurnCount != nil {
		data.TurnCount = strconv.Itoa(*game.TurnCount)
	}
	if game.Notes != nil && strings.TrimSpace(*game.Notes) != "" {
		data.Notes = *game.Notes
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePodJoin(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	pod, _, _, ok := s.loadAccessiblePod(w, r, slug)
	if !ok {
		return
	}
	if !pod.Public {
		http.Error(w, "private pods cannot be joined directly", http.StatusForbidden)
		return
	}

	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	isMember, err := s.dbStore.IsPodMember(pod.ID, user.ID)
	if err != nil {
		http.Error(w, "failed to check pod membership: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !isMember {
		if err := s.dbStore.AddPodMember(pod.ID, user.ID, "member"); err != nil {
			http.Error(w, "failed to join pod: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, "/pods/"+url.PathEscape(pod.Slug), http.StatusSeeOther)
}

func (s *Server) handlePodSubmitPage(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pod, _, _, ok := s.loadAccessiblePod(w, r, slug)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	isMember, err := s.dbStore.IsPodMember(pod.ID, user.ID)
	if err != nil {
		http.Error(w, "failed to check pod membership: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !isMember {
		http.Error(w, "pod membership required", http.StatusForbidden)
		return
	}

	leaderboard, err := s.dbStore.GetPodLeaderboard(pod.ID)
	if err != nil {
		http.Error(w, "failed to load pod players: "+err.Error(), http.StatusInternalServerError)
		return
	}
	players, err := s.dbStore.GetPodMemberNames(pod.ID)
	if err != nil {
		http.Error(w, "failed to load pod members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	elos := make(map[string]int, len(leaderboard))
	for _, row := range leaderboard {
		elos[row.Name] = row.Elo
	}

	t, err := template.New("pod_submit.tmpl").ParseFS(tmplFS, "pod_submit.tmpl")
	if err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Pod         *db.Pod
		Players     []string
		CurrentElos map[string]int
		NowLocal    string
		Nav         navData
	}{
		Pod:         pod,
		Players:     players,
		CurrentElos: elos,
		NowLocal:    time.Now().Format("2006-01-02T15:04"),
		Nav:         navForRequest(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePodSubmitCreate(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	pod, _, _, ok := s.loadAccessiblePod(w, r, slug)
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	isMember, err := s.dbStore.IsPodMember(pod.ID, user.ID)
	if err != nil {
		http.Error(w, "failed to check pod membership: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !isMember {
		http.Error(w, "pod membership required", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	rankings := normalizeRankings(r.Form["placement"])
	if len(rankings) < 2 {
		http.Error(w, "at least two ranked players are required", http.StatusBadRequest)
		return
	}

	seen := make(map[string]struct{}, len(rankings))
	for _, player := range rankings {
		if _, dup := seen[player]; dup {
			http.Error(w, "duplicate players are not allowed", http.StatusBadRequest)
			return
		}
		seen[player] = struct{}{}
	}

	playedAt := time.Now().UTC()
	if raw := strings.TrimSpace(r.FormValue("played_at")); raw != "" {
		parsed, err := time.Parse("2006-01-02T15:04", raw)
		if err != nil {
			http.Error(w, "played_at must be a valid datetime", http.StatusBadRequest)
			return
		}
		playedAt = parsed.UTC()
	}

	turnCount, err := parseOptionalFormInt(r.FormValue("turn_count"))
	if err != nil {
		http.Error(w, "turn_count must be a valid integer", http.StatusBadRequest)
		return
	}
	var notes *string
	if rawNotes := strings.TrimSpace(r.FormValue("notes")); rawNotes != "" {
		notes = &rawNotes
	}

	leaderboard, err := s.dbStore.GetPodLeaderboard(pod.ID)
	if err != nil {
		http.Error(w, "failed to load pod leaderboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	snapshot := make(map[string]int, len(leaderboard))
	for _, row := range leaderboard {
		snapshot[row.Name] = row.Elo
	}

	deltas, err := scoring.ScoreGame(rankings, s.K, s.D, snapshot)
	if err != nil {
		http.Error(w, "failed to score game: "+err.Error(), http.StatusBadRequest)
		return
	}
	scoring.ApplyDeltas(snapshot, deltas)

	playerStats, err := parsePlayerStatsForm(rankings, r.Form)
	if err != nil {
		http.Error(w, "invalid per-player stats: "+err.Error(), http.StatusBadRequest)
		return
	}

	gameID, err := s.dbStore.RecordPodGameWithStats(pod.ID, playedAt, "pod_submit", turnCount, notes, rankings, playerStats, snapshot)
	if err != nil {
		http.Error(w, "failed to submit game: "+err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/pods/"+url.PathEscape(pod.Slug)+"/games/"+strconv.FormatInt(gameID, 10), http.StatusSeeOther)
}

func (s *Server) loadAccessiblePod(w http.ResponseWriter, r *http.Request, slug string) (*db.Pod, bool, string, bool) {
	pod, err := s.dbStore.GetPodBySlug(slug)
	if err != nil {
		http.Error(w, "failed to load pod: "+err.Error(), http.StatusInternalServerError)
		return nil, false, "", false
	}
	if pod == nil {
		http.NotFound(w, r)
		return nil, false, "", false
	}

	user := UserFromContext(r.Context())
	isMember := false
	role := ""
	if user != nil {
		member, err := s.dbStore.GetPodMember(pod.ID, user.ID)
		if err != nil {
			http.Error(w, "failed to load pod member: "+err.Error(), http.StatusInternalServerError)
			return nil, false, "", false
		}
		if member != nil {
			isMember = true
			role = member.Role
		}
	}

	if !pod.Public && !isMember {
		http.Error(w, "pod is private", http.StatusForbidden)
		return nil, false, "", false
	}

	return pod, isMember, role, true
}

func parseOptionalFormInt(raw string) (*int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parsePlayerStatsForm(rankings []string, form url.Values) ([]db.PlayerGameStat, error) {
	players := normalizeRankings(form["stat_player"])
	decks := form["stat_deck"]
	finalLives := form["stat_final_life"]
	eliminations := form["stat_eliminations"]
	cmdDealt := form["stat_cmd_dealt"]
	cmdTaken := form["stat_cmd_taken"]
	turnEliminated := form["stat_turn_eliminated"]

	stats := make([]db.PlayerGameStat, 0, len(players))
	validPlayers := make(map[string]struct{}, len(rankings))
	for _, player := range rankings {
		validPlayers[player] = struct{}{}
	}

	for idx, player := range players {
		if _, ok := validPlayers[player]; !ok {
			continue
		}

		deck := valueAt(decks, idx)
		finalLife, err := parseOptionalFormInt(valueAt(finalLives, idx))
		if err != nil {
			return nil, err
		}
		elims, err := parseRequiredFormIntDefault(valueAt(eliminations, idx), 0)
		if err != nil {
			return nil, err
		}
		dealt, err := parseRequiredFormIntDefault(valueAt(cmdDealt, idx), 0)
		if err != nil {
			return nil, err
		}
		taken, err := parseRequiredFormIntDefault(valueAt(cmdTaken, idx), 0)
		if err != nil {
			return nil, err
		}
		turnOut, err := parseOptionalFormInt(valueAt(turnEliminated, idx))
		if err != nil {
			return nil, err
		}

		if deck == "" && finalLife == nil && elims == 0 && dealt == 0 && taken == 0 && turnOut == nil {
			continue
		}

		stats = append(stats, db.PlayerGameStat{
			Player:               player,
			DeckName:             deck,
			StartingLife:         40,
			FinalLife:            finalLife,
			Eliminations:         elims,
			CommanderDamageDealt: dealt,
			CommanderDamageTaken: taken,
			TurnEliminated:       turnOut,
		})
	}

	return stats, nil
}

func parseRequiredFormIntDefault(raw string, defaultValue int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

func valueAt(values []string, idx int) string {
	if idx < 0 || idx >= len(values) {
		return ""
	}
	return strings.TrimSpace(values[idx])
}
