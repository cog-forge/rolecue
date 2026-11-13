package domain

// Bounds of a generated topic set (KAN-343). They are technical limits of the contract, not product rules:
// a Roadmap is a short ordered list a candidate can read at a glance, and every topic needs at least five
// distinct questions in the bank, so a bank of default size cannot support many more topics than this.
const (
	MinTopics                = 4
	MaxTopics                = 8
	MaxTopicTitleRunes       = 80
	MaxTopicDescriptionRunes = 400
)

// TopicRequirement is one confirmed requirement given to the topic generator. Evidence is a text excerpt of
// the job description written by a person, so it is untrusted data.
type TopicRequirement struct {
	Name         string
	Target       string
	Type         string
	EvidenceText *string
}

// TopicInput is everything the topic generator may use: the confirmed requirements, the seniority level and
// the candidate's refinement note. The generator never sees the raw job description.
type TopicInput struct {
	SeniorityName        string
	SeniorityDescription *string
	Requirements         []TopicRequirement
	RefinementNote       string
}

// GeneratedTopic is one proposed topic. The model never supplies ids or positions.
type GeneratedTopic struct {
	Title       string
	Description string
}

// TopicCandidate is an untrusted proposal from the model; only validation turns it into stored topics.
type TopicCandidate struct {
	Topics []GeneratedTopic
}
