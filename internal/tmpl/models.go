package tmpl

import (
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
)

type (
	TemplateContent interface {
		EntryID() uint
		EntryTitle() string
		EntryContent() string
		EntryType() entries.EntryType
		EntryRating() fa.Rating
		EntryBlocked() bool
		ViewLink() string
	}
	NewNotesContent struct {
		ID      uint
		Title   string
		User    *fa.FurAffinityUser
		Content string
		Link    string
		Rating  fa.Rating
	}

	NewCommentsContent struct {
		ID      uint
		OnEntry string
		User    *fa.FurAffinityUser
		Content string
		Link    string
		Type    entries.EntryType
		Rating  fa.Rating
	}

	NewJournalsContent struct {
		ID      uint
		Title   string
		User    *fa.FurAffinityUser
		Content string
		Link    string
		Rating  fa.Rating
	}

	NewSubmissionsContent struct {
		ID           uint
		Title        string
		User         *fa.FurAffinityUser
		Link         string
		Description  string
		ThumbnailUrl string
		FullViewUrl  string
		Rating       fa.Rating
		Type         fa.SubmissionType
		Blocked      bool
	}
)

func (n *NewNotesContent) EntryID() uint {
	return n.ID
}
func (n *NewNotesContent) EntryTitle() string {
	return n.Title
}
func (n *NewNotesContent) EntryContent() string {
	return n.Content
}
func (n *NewNotesContent) EntryType() entries.EntryType {
	return entries.EntryTypeNote
}
func (n *NewNotesContent) ViewLink() string {
	return n.Link
}
func (n *NewNotesContent) EntryRating() fa.Rating {
	return n.Rating
}
func (n *NewNotesContent) EntryBlocked() bool {
	return false
}

func (n *NewJournalsContent) EntryID() uint {
	return n.ID
}
func (n *NewJournalsContent) EntryTitle() string {
	return n.Title
}
func (n *NewJournalsContent) EntryContent() string {
	return n.Content
}
func (n *NewJournalsContent) EntryType() entries.EntryType {
	return entries.EntryTypeJournal
}
func (n *NewJournalsContent) ViewLink() string {
	return n.Link
}
func (n *NewJournalsContent) EntryRating() fa.Rating {
	return n.Rating
}
func (n *NewJournalsContent) EntryBlocked() bool {
	return false
}

func (n *NewSubmissionsContent) EntryID() uint {
	return n.ID
}
func (n *NewSubmissionsContent) EntryTitle() string {
	return n.Title
}
func (n *NewSubmissionsContent) EntryContent() string {
	return n.Description
}
func (n *NewSubmissionsContent) EntryType() entries.EntryType {
	return entries.EntryTypeSubmission
}
func (n *NewSubmissionsContent) ViewLink() string {
	return n.Link
}
func (n *NewSubmissionsContent) EntryRating() fa.Rating {
	return n.Rating
}
func (n *NewSubmissionsContent) EntryBlocked() bool {
	return n.Blocked
}

func (n *NewCommentsContent) EntryID() uint {
	return n.ID
}
func (n *NewCommentsContent) EntryTitle() string {
	return n.OnEntry
}
func (n *NewCommentsContent) EntryContent() string {
	return n.Content
}
func (n *NewCommentsContent) EntryType() entries.EntryType {
	return n.Type
}
func (n *NewCommentsContent) ViewLink() string {
	return n.Link
}
func (n *NewCommentsContent) EntryRating() fa.Rating {
	return n.Rating
}
func (n *NewCommentsContent) EntryBlocked() bool {
	return false
}
