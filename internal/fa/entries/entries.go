package entries

type EntryType uint8

const (
	EntryTypeInvalid EntryType = iota
	EntryTypeNote
	EntryTypeSubmissionComment
	EntryTypeJournalComment
	EntryTypeJournal
)

func EntriesOther() []EntryType {
	return []EntryType{
		EntryTypeJournal,
		EntryTypeSubmissionComment,
		EntryTypeJournalComment,
	}
}
