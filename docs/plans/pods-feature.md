# Feature Design: Pod-Specific Leaderboards

## Overview

A **Pod** is a named playgroup — a persistent collection of players with their own leaderboard, game history, and Elo ratings independent from the global scoreboard. Think of it like a subreddit: anyone can be in multiple pods, each pod has its own context.

---

## Core Concepts

### Pod
- Has a name, slug (URL-safe identifier), and optional description
- Has an **owner** (the creator) and any number of **members**
- Owns its own game log — games recorded in a pod only count toward that pod's Elo
- Has a global all-time leaderboard + ability to view filtered subsets (date ranges, seasons)

### Player/User
- Currently players are just names — this feature requires real user accounts
- A user has: username, display name, optional avatar, created_at
- A user can belong to multiple pods
- A player's Elo is **per-pod** (your Elo in Pod A is independent of Pod B)

### Invite System
- Pod owners/admins can invite by username (if the user already has an account)
- Or generate an **invite link** — a single-use or multi-use token that lets someone create an account and auto-join the pod
- Invite links expire after 7 days by default

---

## Data Model

```sql
-- Users (new table — replaces bare player names)
CREATE TABLE users (
    id           INTEGER PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,             -- bcrypt
    global_elo   INTEGER NOT NULL DEFAULT 1500, -- all-time across all pods
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Sessions
CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Pods
CREATE TABLE pods (
    id          INTEGER PRIMARY KEY,
    slug        TEXT UNIQUE NOT NULL,       -- URL identifier: /pods/my-group
    name        TEXT NOT NULL,
    description TEXT,
    owner_id    INTEGER NOT NULL REFERENCES users(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Pod membership
CREATE TABLE pod_members (
    pod_id      INTEGER NOT NULL REFERENCES pods(id),
    user_id     INTEGER NOT NULL REFERENCES users(id),
    role        TEXT NOT NULL DEFAULT 'member', -- 'owner', 'admin', 'member'
    joined_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (pod_id, user_id)
);

-- Invite tokens
CREATE TABLE pod_invites (
    id          INTEGER PRIMARY KEY,
    pod_id      INTEGER NOT NULL REFERENCES pods(id),
    token       TEXT UNIQUE NOT NULL,       -- random 32-char hex
    created_by  INTEGER NOT NULL REFERENCES users(id),
    max_uses    INTEGER DEFAULT NULL,       -- NULL = unlimited
    uses        INTEGER NOT NULL DEFAULT 0,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Games now belong to a pod (replaces bare source string)
ALTER TABLE games ADD COLUMN pod_id INTEGER REFERENCES pods(id);
-- NULL pod_id = legacy/global game

-- Per-pod Elo (replace single elo column on players)
CREATE TABLE pod_elos (
    pod_id      INTEGER NOT NULL REFERENCES pods(id),
    user_id     INTEGER NOT NULL REFERENCES users(id),
    elo         INTEGER NOT NULL DEFAULT 1500,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (pod_id, user_id)
);
```

### Migration path
- Existing `players` table stays; existing games get `pod_id = NULL` (global/legacy)
- New pods use the full user+pod model
- A "default" global pod can be created to house the existing data if needed

---

## URL Structure

```
/pods                           → list all pods (user's pods if logged in)
/pods/new                       → create a pod
/pods/:slug                     → pod leaderboard (all-time)
/pods/:slug/games               → pod game history (paginated)
/pods/:slug/games/:id           → pod game detail
/pods/:slug/players/:username   → player profile within this pod
/pods/:slug/submit              → submit a game to this pod (admin/member)
/pods/:slug/settings            → pod settings (owner/admin)
/pods/:slug/invite              → manage invite links (owner/admin)

/join/:token                    → accept an invite (creates account if needed)
/register                       → create an account
/login                          → log in (simple password or magic link)
/profile                        → current user's profile across all pods
```

---

## Auth Model

The current admin key is a blunt instrument. Pods require real user sessions.

**Recommended: simple cookie sessions + bcrypt passwords**
- No JWT overhead, no OAuth complexity for v1
- `sessions` table: `(token TEXT, user_id INTEGER, expires_at DATETIME)`
- Middleware checks cookie → session → user
- Password reset via email (optional for v1, can skip)

**Alternative: magic link only (no passwords)**
- User enters email → gets a one-time login link
- Simpler to implement, no password storage
- Downside: requires email config (SMTP)

**Recommendation:** Start with username + password. Magic link or OAuth can come later.

---

## Elo Scoping

Each pod maintains its own independent Elo ratings:

- `pod_elos` stores `(pod_id, user_id, elo)` — replaces the global `players.elo`
- The global leaderboard (current behavior) becomes the "default pod" or a special view
- `GetPlayerStats()` gets a `podID` parameter
- Scoring engine is unchanged — it still just takes a ranked list and returns deltas

---

## Subset/Season Views

"Smaller subsets of games" = filtered views of the pod game log:

**Simple approach — date range filters:**
- `/pods/:slug?from=2026-01-01&to=2026-03-01` filters the leaderboard to games in that range
- Elo is **replayed** from scratch over that range (same replay engine we already have)
- No schema changes needed — just filter `WHERE played_at BETWEEN ? AND ?`

**Richer approach — named seasons:**
```sql
CREATE TABLE pod_seasons (
    id          INTEGER PRIMARY KEY,
    pod_id      INTEGER NOT NULL REFERENCES pods(id),
    name        TEXT NOT NULL,              -- "Season 1", "Spring 2026"
    starts_at   DATETIME NOT NULL,
    ends_at     DATETIME,                   -- NULL = active season
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```
- Leaderboard can show current season or all-time
- `/pods/:slug/seasons/:id` → season-specific leaderboard

**Recommendation:** Ship date range filters first (zero schema cost), add named seasons in a follow-up.

---

## Implementation Phases

### Phase A — User Accounts
1. `users` table + registration/login routes
2. Cookie session middleware
3. Migrate existing player names to users (optional: create accounts for existing names or leave as legacy)

### Phase B — Pods
1. `pods`, `pod_members`, `pod_elos` tables
2. Pod CRUD (create, view, settings)
3. Pod-scoped game submission and Elo tracking
4. Pod leaderboard page using per-pod Elo

### Phase C — Invites
1. `pod_invites` table
2. Invite link generation (admin/owner only)
3. `/join/:token` flow — creates account (if needed) + joins pod
4. Add-by-username flow for existing users

### Phase D — Subset Views
1. Date range filter on pod leaderboard
2. (Optional) Named seasons

---

## Decisions

1. **Auth:** Username + password (bcrypt). Magic link / OAuth deferred.

2. **Guest players:** Pods support unregistered guest players (bare name, no account). A guest can later "claim" their name by registering with a matching username — their game history transfers over.

3. **Elo is dual-tracked:**
   - `pod_elos` — per-pod Elo, independent per group
   - `users.global_elo` — a single all-time Elo that aggregates every game the user has played across all pods, treated as one continuous rating
   - Guest players also accumulate a global Elo, stored on the `players` table until claimed

4. **Pod visibility:** Public by default (anyone can view leaderboard/games), owner can toggle private.

5. **Single admin key vs per-pod auth:** `GUILDMASTER_ADMIN_KEY` stays as a superadmin bypass. Pod owners manage their own pods via session auth.

---

## Effort Estimate

| Phase | Complexity | Rough effort |
|---|---|---|
| A — User accounts | Medium | 1-2 days |
| B — Pods core | Medium-High | 2-3 days |
| C — Invites | Low-Medium | 1 day |
| D — Subset views (date range) | Low | half day |

Total: **~1 week** of focused Codex work across multiple agents.
