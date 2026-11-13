package domain

// LinkOutcome is what one LinkCores call did. Written is the rows newly written; Present is how many of the
// listed positions have a link afterwards, whoever wrote it; Mappable is how many of them could have one because
// their core question has a topic in the Roadmap of the interview's profile. A core without a topic is not
// mappable and never counts as refused.
type LinkOutcome struct {
	Written  int64
	Present  int64
	Mappable int64
}
