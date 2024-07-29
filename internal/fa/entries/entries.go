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

var nameMap = map[EntryType]string{
	EntryTypeInvalid:           "INVALID",
	EntryTypeNote:              "Note",
	EntryTypeSubmission:        "Submission",
	EntryTypeSubmissionComment: "Submission Comment",
	EntryTypeJournal:           "Journal",
	EntryTypeJournalComment:    "Journal Comment",
}

func (e EntryType) Name() string {
	name, found := nameMap[e]
	if !found {
		return nameMap[EntryTypeInvalid]
	}
	return name
}
