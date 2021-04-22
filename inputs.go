package main

type exportCondition int

const (
	invalid exportCondition = iota
	always
	never
	onFailure
)

func parseExportCondition(condition string) exportCondition {
	switch condition {
	case "always":
		return always
	case "never":
		return never
	case "on_failure":
		return onFailure
	default:
		return invalid
	}
}
