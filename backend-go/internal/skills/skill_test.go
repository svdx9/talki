package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Fixture(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata")
	fsys := os.DirFS(dir)
	skills, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", dir, err)
	}
	if len(skills) == 0 {
		t.Fatal("expected at least one skill from testdata fixture")
	}
	if skills[0].ID != "test-skill" {
		t.Errorf("expected skill ID 'test-skill', got %q", skills[0].ID)
	}
	if skills[0].Title != "Test skill for unit tests" {
		t.Errorf("unexpected title: %q", skills[0].Title)
	}
}

func TestLoad_DisallowUnknownFields(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	subDir := filepath.Join(tmp, "catalog")
	err := os.Mkdir(subDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(subDir, "unknown-field.json")
	err = os.WriteFile(fixture, []byte(`{
  "id": "unknown-field-test",
  "title": "Test",
  "level": "A1",
  "locale": "en-US",
  "voice": "voice",
  "persona": {"name": "N", "role": "R"},
  "constraints": {"vocabulary": "v", "turn_length": "t", "corrections": "c"},
  "opening_line": "Hi",
  "system_prompt": "You",
  "unknown_field": "should error"
}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Load(os.DirFS(subDir))
	if err == nil {
		t.Error("expected error for unknown field, got nil")
	}
}

func TestLoad_Catalog(t *testing.T) {
	t.Parallel()
	skills, err := Load(Catalog)
	if err != nil {
		t.Fatalf("Load(Catalog) returned error: %v", err)
	}
	if len(skills) == 0 {
		t.Error("expected at least one skill in catalog")
	}
	ids := make([]string, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
	}
	t.Logf("loaded %d skills: %v", len(skills), ids)
}

func TestLoad_NoSkills(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	subDir := filepath.Join(tmp, "empty")
	err := os.Mkdir(subDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Load(os.DirFS(subDir))
	if err == nil {
		t.Error("expected error for empty catalog, got nil")
	}
}
