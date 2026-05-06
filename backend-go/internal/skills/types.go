package skills

type Repository interface {
	Get(string) (*Skill, error)
	Descriptions() []SkillDescription
}
