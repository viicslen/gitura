package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"gitura/internal/db"
)

func TestSetPRLocalPath_InvalidIdentity_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	a, sqlDB := newTestAppWithStateDB(t, tmpDir)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	err := a.SetPRLocalPath("", "repo", 1, "/tmp/repo")
	if err == nil {
		t.Fatal("expected validation error for empty owner")
	}

	err = a.SetPRLocalPath("owner", "", 1, "/tmp/repo")
	if err == nil {
		t.Fatal("expected validation error for empty repo")
	}

	err = a.SetPRLocalPath("owner", "repo", 0, "/tmp/repo")
	if err == nil {
		t.Fatal("expected validation error for non-positive PR number")
	}
}

func TestPRLocalPath_PerPRIdentity_Isolated(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	a, sqlDB := newTestAppWithStateDB(t, tmpDir)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := a.SetPRLocalPath("acme", "api", 10, "/work/acme-api"); err != nil {
		t.Fatalf("set path for first PR: %v", err)
	}
	if err := a.SetPRLocalPath("acme", "web", 10, "/work/acme-web"); err != nil {
		t.Fatalf("set path for second PR: %v", err)
	}

	path1, err := a.GetPRLocalPath("acme", "api", 10)
	if err != nil {
		t.Fatalf("get path for first PR: %v", err)
	}
	if path1 != "/work/acme-api" {
		t.Fatalf("unexpected first PR path: got %q want %q", path1, "/work/acme-api")
	}

	path2, err := a.GetPRLocalPath("acme", "web", 10)
	if err != nil {
		t.Fatalf("get path for second PR: %v", err)
	}
	if path2 != "/work/acme-web" {
		t.Fatalf("unexpected second PR path: got %q want %q", path2, "/work/acme-web")
	}
}

func TestPRLocalPath_AcrossAppRestart_PersistsValue(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	a1, db1 := newTestAppWithStateDB(t, tmpDir)
	if err := a1.SetPRLocalPath("acme", "repo", 42, "/home/user/src/repo"); err != nil {
		db1.Close()
		t.Fatalf("set path before restart: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close db before restart: %v", err)
	}

	a2, db2 := newTestAppWithStateDB(t, tmpDir)
	t.Cleanup(func() {
		_ = db2.Close()
	})

	got, err := a2.GetPRLocalPath("acme", "repo", 42)
	if err != nil {
		t.Fatalf("get path after restart: %v", err)
	}
	if got != "/home/user/src/repo" {
		t.Fatalf("unexpected persisted path: got %q want %q", got, "/home/user/src/repo")
	}
}

func newTestAppWithStateDB(t *testing.T, stateDir string) (*App, *sql.DB) {
	t.Helper()

	sqlDB, err := db.Open(stateDir)
	if err != nil {
		t.Fatalf("open state db under %q: %v", filepath.Join(stateDir, "gitura", "state.db"), err)
	}

	a := NewApp()
	a.ctx = context.Background()
	a.queries = db.New(sqlDB)

	return a, sqlDB
}
