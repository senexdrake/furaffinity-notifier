package entries

import (
	"fmt"

	"github.com/fanonwue/goutils/dsext"
)

type EntryType uint8

const (
	EntryTypeInvalid EntryType = iota
	EntryTypeNote
	EntryTypeSubmission
	EntryTypeSubmissionComment
	EntryTypeJournal
	EntryTypeJournalComment
)

func ValidEntryTypes() []EntryType {
	return []EntryType{
		EntryTypeNote,
		EntryTypeSubmission,
		EntryTypeSubmissionComment,
		EntryTypeJournal,
		EntryTypeJournalComment,
	}
}

func ValidEntryTypesSet() dsext.Set[EntryType] {
	return dsext.NewSetSlice(ValidEntryTypes())
}

func EntryTypes() []EntryType {
	return append(ValidEntryTypes(), EntryTypeInvalid)
}

func EntryTypesSet() dsext.Set[EntryType] {
	return dsext.NewSetSlice(EntryTypes())
}

func (e EntryType) Name() string {
	switch e {
	case EntryTypeInvalid:
		return "INVALID"
	case EntryTypeNote:
		return "Note"
	case EntryTypeSubmission:
		return "Submission"
	case EntryTypeSubmissionComment:
		return "Submission Comment"
	case EntryTypeJournal:
		return "Journal"
	case EntryTypeJournalComment:
		return "Journal Comment"
	}
	panic(fmt.Sprintf("unreachable: unknown entry type %d", e))
}

// FilterEnvVar returns the non-prefixed environment variable name corresponding to the user filter list of this [EntryType].
// The environment variable name might be empty (e.g., for [EntryTypeInvalid]); callers should handle this accordingly.
func (e EntryType) FilterEnvVar() string {
	switch e {
	case EntryTypeSubmission:
		return "SUBMISSIONS_USER_FILTER"
	case EntryTypeJournal:
		return "JOURNALS_USER_FILTER"
	case EntryTypeNote:
		return "NOTES_USER_FILTER"
	case EntryTypeJournalComment, EntryTypeSubmissionComment:
		return "COMMENTS_USER_FILTER"
	case EntryTypeInvalid:
		// The invalid entry type should not cause a panic, but it doesn't have an env var either
		return ""
	}
	panic(fmt.Sprintf("unreachable: unknown entry type %d", e))
}

func (e EntryType) String() string {
	return e.Name()
}
