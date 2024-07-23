package entries

type EntryType uint8

const (
	EntryTypeInvalid EntryType = iota
	EntryTypeNote
	EntryTypeComment
	EntryTypeJournal
)
