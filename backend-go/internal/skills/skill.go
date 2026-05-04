package skills

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed catalog/*.json
var Catalog embed.FS

type Persona struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type Constraints struct {
	Vocabulary  string `json:"vocabulary"`
	TurnLength  string `json:"turn_length"`
	Corrections string `json:"corrections"`
}

type Skill struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Level        string      `json:"level"`
	Locale       string      `json:"locale"`
	Voice        string      `json:"voice"`
	Persona      Persona     `json:"persona"`
	Constraints  Constraints `json:"constraints"`
	OpeningLine  string      `json:"opening_line"`
	SystemPrompt string      `json:"system_prompt"`
}

func Load(fsys fs.FS) ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, "catalog")
	if err != nil {
		return nil, fmt.Errorf("reading catalog directory: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := "catalog/" + entry.Name()
		f, err := fsys.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", path, err)
		}
		defer f.Close()

		var skill Skill
		dec := json.NewDecoder(f)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&skill); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}

		skills = append(skills, skill)
	}

	if len(skills) == 0 {
		return nil, fmt.Errorf("no skills found in catalog")
	}

	return skills, nil
}