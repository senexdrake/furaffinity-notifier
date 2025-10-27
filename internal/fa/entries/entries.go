package entries

type EntryType uint8

const (
	EntryTypeInvalid EntryType = iota
	EntryTypeNote
	EntryTypeSubmission
	EntryTypeSubmissionComment
	EntryTypeJournal
	EntryTypeJournalComment
)

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
	panic("unreachable")
}

func (e EntryType) String() string {
	return e.Name()
}
