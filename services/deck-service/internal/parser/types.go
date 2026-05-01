package parser

// ParsedCard holds a single flashcard pair returned to the client for review.
type ParsedCard struct {
	Front string
	Back  string
}
