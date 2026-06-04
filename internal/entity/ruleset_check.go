package entity

type RulesetCheckOutcome int

const (
	RulesetCheckOK             RulesetCheckOutcome = iota // reachable, file present, content matches the declared format
	RulesetCheckUnreachable                               // DNS / connection / timeout — never got a response
	RulesetCheckHTTPError                                 // got a response, but a non-200 status (e.g. 404)
	RulesetCheckEmpty                                     // 200 but an empty body
	RulesetCheckFormatMismatch                            // downloaded, but the content isn't the declared format
)

// RulesetCheckResult carries the structured outcome of a Checker.Check probe.
type RulesetCheckResult struct {
	Outcome RulesetCheckOutcome
	Status  int    // HTTP status code (for RulesetCheckHTTPError)
	Size    int    // bytes downloaded (for RulesetCheckOK / RulesetCheckFormatMismatch)
	Detail  string // short technical detail for RulesetCheckUnreachable (the network error)
}
