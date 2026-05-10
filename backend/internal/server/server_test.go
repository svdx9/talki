package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/svdx9/talki/backend/internal/config"
	"github.com/svdx9/talki/backend/internal/skills"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	repo := skills.NewMemoryRepository([]skills.Skill{
		//exhaustruct:ignore
		{
			//exhaustruct:ignore
			SkillDescription: skills.SkillDescription{
				ID:    "1",
				Title: "Test skill for unit tests",
			},
		},
	})

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil)) //exhaustruct:ignore
	cfg := config.Config{Port: 0, LogLevel: slog.LevelInfo} //exhaustruct:ignore

	srv, err := New(cfg, logger, repo)
	if err != nil {
		t.Errorf("unexpected erorr")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %s", ct)
	}

	var resp struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}

	ts, err := time.Parse(time.RFC3339, resp.Timestamp)
	if err != nil {
		t.Errorf("expected RFC3339 timestamp, got %s: %v", resp.Timestamp, err)
	}
	now := time.Now().UTC()
	if now.Sub(ts) > time.Minute {
		t.Errorf("timestamp not recent: %v", ts)
	}
}

func TestSkills(t *testing.T) {
	t.Parallel()
	testSkills := []skills.Skill{
		//exhaustruct:ignore
		{SkillDescription: skills.SkillDescription{ID: "skill-1", Title: "French Basics", Level: "beginner"}},
		//exhaustruct:ignore
		{SkillDescription: skills.SkillDescription{ID: "skill-2", Title: "French Advanced", Level: "advanced"}},
		//exhaustruct:ignore
		{SkillDescription: skills.SkillDescription{ID: "skill-3", Title: "Spanish Basics", Level: "beginner"}},
	}
	repo := skills.NewMemoryRepository(testSkills)
	//exhaustruct:ignore

	//exhaustruct:ignore
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	//exhaustruct:ignore
	cfg := config.Config{Port: 0, LogLevel: slog.LevelInfo}

	srv, err := New(cfg, logger, repo)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr := httptest.NewRecorder()

	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %s", ct)
	}

	type skillSubset struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Level string `json:"level"`
	}
	var resp []skillSubset
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(resp) != len(testSkills) {
		t.Errorf("expected %d skills, got %d", len(testSkills), len(resp))
	}

	for i, sk := range resp {
		if sk.ID != testSkills[i].ID {
			t.Errorf("skill[%d]: expected id=%s, got %s", i, testSkills[i].ID, sk.ID)
		}
		if sk.Title != testSkills[i].Title {
			t.Errorf("skill[%d]: expected title=%s, got %s", i, testSkills[i].Title, sk.Title)
		}
		if sk.Level != testSkills[i].Level {
			t.Errorf("skill[%d]: expected level=%s, got %s", i, testSkills[i].Level, sk.Level)
		}
	}
}
