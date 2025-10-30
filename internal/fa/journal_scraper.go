package fa

import "regexp"

var (
	journalIdRegex = regexp.MustCompile(".*/journal/(\\d*)/*")
)
