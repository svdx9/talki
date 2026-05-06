package skills

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed catalog/*.json
var Catalog embed.FS

var ErrNoSkills = errors.New("no skills found in catalog")
var ErrNoSKillId = errors.New("no skill matching id")

type Persona struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type Constraints struct {
	Vocabulary  string `json:"vocabulary"`
	TurnLength  string `json:"turn_length"`
	Corrections string `json:"corrections"`
}

type SkillDescription struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Level string `json:"level"`
}

type Skill struct {
	SkillDescription
	Locale       string      `json:"locale"`
	Voice        string      `json:"voice"`
	Persona      Persona     `json:"persona"`
	Constraints  Constraints `json:"constraints"`
	OpeningLine  string      `json:"opening_line"`
	SystemPrompt string      `json:"system_prompt"`
}

type MemoryRepository struct {
	skills []Skill
}

func (s *MemoryRepository) Get(id string) (*Skill, error) {
	for _, skill := range s.skills {
		if skill.ID == id {
			return &skill, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoSKillId, id)
}

func (s *MemoryRepository) Descriptions() []SkillDescription {
	var descriptions []SkillDescription
	for _, skill := range s.skills {
		descriptions = append(descriptions, skill.SkillDescription)
	}
	return descriptions
}

func NewMemoryRepositoryFromFile(fsys fs.FS) (*MemoryRepository, error) {
	entries, err := fs.ReadDir(fsys, "catalog")
	if err != nil {
		return nil, fmt.Errorf("reading catalog directory: %w", err)
	}

	sp := MemoryRepository{
		skills: make([]Skill, 0),
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := "catalog/" + entry.Name()
		f, err := fsys.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", path, err)
		}

		var skill Skill
		dec := json.NewDecoder(f)
		dec.DisallowUnknownFields()
		err = dec.Decode(&skill)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		err = f.Close()
		if err != nil {
			return nil, fmt.Errorf("closing %s: %w", path, err)
		}

		sp.skills = append(sp.skills, skill)
	}

	if len(sp.skills) == 0 {
		return nil, ErrNoSkills
	}

	return &sp, nil
}

func NewMemoryRepository(s []Skill) *MemoryRepository {
	r := &MemoryRepository{}
	r.skills = append(r.skills, s...)
	return r
}
