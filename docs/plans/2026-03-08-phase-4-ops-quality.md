# Phase 4 Ops & Quality Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement Phase 4.1-4.4 from PLAN.md: tests, config centralization, Docker updates, and CLI cleanup.

**Architecture:** Extend existing packages with focused tests and a centralized `internal/config` loader, then wire runtime entry points to config and simplify CLI binaries to one canonical server command. Keep behavior unchanged unless required by config/env plumbing.

**Tech Stack:** Go, net/http, httptest, modernc SQLite driver, Docker multi-stage build.

---

### Task 1: Phase 4.1 tests
- Expand scoring unit tests in `internal/scoring/store_test.go`.
- Add HTTP integration tests with in-memory SQLite DB.
- Add Sheets parser fixture tests with `fixtures/` data.
- Run `go test ./...` and commit.

### Task 2: Phase 4.2 config
- Add `internal/config` with `Config` and `Load()` env parsing/defaults.
- Replace hardcoded config in server/runtime with config values.
- Run tests/build (best effort under Go 1.18) and commit.

### Task 3: Phase 4.3 docker
- Update `Dockerfile` to multi-stage and minimal runtime.
- Set env-driven config and DB volume mount.
- Update `docker-compose.yml` to pass env vars and mount `./data`.
- Commit.

### Task 4: Phase 4.4 CLI cleanup
- Consolidate duplicate root CLI vs `cmd/analyze`.
- Keep `cmd/server` canonical; remove dead root main code.
- Validate build/tests as far as local toolchain allows and commit.
