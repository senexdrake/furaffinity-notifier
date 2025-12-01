package tmpl

import (
	"testing"

	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/stretchr/testify/assert"
)

type TestStruct[T any] struct {
	name     string
	content  TemplateContent
	expected T
}

type TestStructList[T any] []TestStruct[T]

// Test fixtures for NewNotesContent
var (
	notesContentWithData = &NewNotesContent{
		ID:      12345,
		Title:   "Test Note",
		Content: "Note content",
		Link:    "http://example.com/note/12345",
		Rating:  fa.RatingAdult,
	}
	notesContentEmpty = &NewNotesContent{
		ID:      0,
		Title:   "",
		Content: "",
		Link:    "",
	}
	notesContentSpecialChars = &NewNotesContent{
		ID:      999,
		Title:   "Title with @#$%^&*()",
		Content: "Line 1\nLine 2\nLine 3",
		Link:    "http://example.com?user=john&action=view",
	}
)

func TestNewNotesContent_EntryID(t *testing.T) {
	tests := TestStructList[uint]{
		{
			name:     "returns correct ID",
			content:  notesContentWithData,
			expected: 12345,
		},
		{
			name:     "returns zero ID",
			content:  notesContentEmpty,
			expected: 0,
		},
		{
			name:     "returns large ID",
			content:  notesContentSpecialChars,
			expected: 999,
		},
	}

	runTests(t, tests, func(tc TemplateContent) uint {
		return tc.EntryID()
	})
}

func TestNewNotesContent_EntryTitle(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct title",
			content:  notesContentWithData,
			expected: "Test Note",
		},
		{
			name:     "returns empty title",
			content:  notesContentEmpty,
			expected: "",
		},
		{
			name:     "returns title with special characters",
			content:  notesContentSpecialChars,
			expected: "Title with @#$%^&*()",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryTitle()
	})
}

func TestNewNotesContent_EntryContent(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct content",
			content:  notesContentWithData,
			expected: "Note content",
		},
		{
			name:     "returns empty content",
			content:  notesContentEmpty,
			expected: "",
		},
		{
			name:     "returns multiline content",
			content:  notesContentSpecialChars,
			expected: "Line 1\nLine 2\nLine 3",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryContent()
	})
}

func TestNewNotesContent_EntryType(t *testing.T) {
	tests := TestStructList[entries.EntryType]{
		{
			name:     "returns EntryTypeNote for normal content",
			content:  notesContentWithData,
			expected: entries.EntryTypeNote,
		},
		{
			name:     "returns EntryTypeNote for empty content",
			content:  notesContentEmpty,
			expected: entries.EntryTypeNote,
		},
		{
			name:     "returns EntryTypeNote for special chars content",
			content:  notesContentSpecialChars,
			expected: entries.EntryTypeNote,
		},
	}

	runTests(t, tests, func(tc TemplateContent) entries.EntryType {
		return tc.EntryType()
	})
}

func TestNewNotesContent_ViewLink(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct link",
			content:  notesContentWithData,
			expected: "http://example.com/note/12345",
		},
		{
			name:     "returns empty link",
			content:  notesContentEmpty,
			expected: "",
		},
		{
			name:     "returns link with query parameters",
			content:  notesContentSpecialChars,
			expected: "http://example.com?user=john&action=view",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.ViewLink()
	})
}

func TestNewNewNotesContent_EntryRating(t *testing.T) {
	tests := TestStructList[fa.Rating]{
		{
			name:     "returns adult rating for notes content with data",
			content:  notesContentWithData,
			expected: fa.RatingAdult,
		},
		{
			name:     "returns general rating for empty",
			content:  notesContentEmpty,
			expected: fa.RatingGeneral,
		},
	}

	runTests(t, tests, func(tc TemplateContent) fa.Rating {
		return tc.EntryRating()
	})
}

// Test fixtures for NewJournalsContent
var (
	journalsContentWithData = &NewJournalsContent{
		ID:      98765,
		Title:   "My Journal Entry",
		User:    &fa.FurAffinityUser{},
		Content: "This is a journal entry",
		Link:    "http://example.com/journal/98765",
		Rating:  fa.RatingMature,
	}
	journalsContentEmpty = &NewJournalsContent{
		ID:      0,
		Title:   "",
		User:    nil,
		Content: "",
		Link:    "",
	}
	journalsContentWithMultiline = &NewJournalsContent{
		ID:      555,
		Title:   "Multiline Journal",
		User:    &fa.FurAffinityUser{},
		Content: "Line 1\nLine 2\nLine 3\nLine 4",
		Link:    "http://example.com/journal/555",
		Rating:  fa.RatingAdult,
	}
)

func TestNewJournalsContent_EntryID(t *testing.T) {
	tests := TestStructList[uint]{
		{
			name:     "returns correct ID",
			content:  journalsContentWithData,
			expected: 98765,
		},
		{
			name:     "returns zero ID",
			content:  journalsContentEmpty,
			expected: 0,
		},
		{
			name:     "returns ID for multiline",
			content:  journalsContentWithMultiline,
			expected: 555,
		},
	}

	runTests(t, tests, func(tc TemplateContent) uint {
		return tc.EntryID()
	})
}

func TestNewJournalsContent_EntryTitle(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct title",
			content:  journalsContentWithData,
			expected: "My Journal Entry",
		},
		{
			name:     "returns empty title",
			content:  journalsContentEmpty,
			expected: "",
		},
		{
			name:     "returns title for multiline",
			content:  journalsContentWithMultiline,
			expected: "Multiline Journal",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryTitle()
	})
}

func TestNewJournalsContent_EntryContent(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct content",
			content:  journalsContentWithData,
			expected: "This is a journal entry",
		},
		{
			name:     "returns empty content",
			content:  journalsContentEmpty,
			expected: "",
		},
		{
			name:     "returns multiline content",
			content:  journalsContentWithMultiline,
			expected: "Line 1\nLine 2\nLine 3\nLine 4",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryContent()
	})
}

func TestNewJournalsContent_EntryType(t *testing.T) {
	tests := TestStructList[entries.EntryType]{
		{
			name:     "returns EntryTypeJournal for normal content",
			content:  journalsContentWithData,
			expected: entries.EntryTypeJournal,
		},
		{
			name:     "returns EntryTypeJournal for empty content",
			content:  journalsContentEmpty,
			expected: entries.EntryTypeJournal,
		},
		{
			name:     "returns EntryTypeJournal for multiline",
			content:  journalsContentWithMultiline,
			expected: entries.EntryTypeJournal,
		},
	}

	runTests(t, tests, func(tc TemplateContent) entries.EntryType {
		return tc.EntryType()
	})
}

func TestNewJournalsContent_ViewLink(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct link",
			content:  journalsContentWithData,
			expected: "http://example.com/journal/98765",
		},
		{
			name:     "returns empty link",
			content:  journalsContentEmpty,
			expected: "",
		},
		{
			name:     "returns link for multiline",
			content:  journalsContentWithMultiline,
			expected: "http://example.com/journal/555",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.ViewLink()
	})
}

func TestNewNewJournalsContent_EntryRating(t *testing.T) {
	tests := TestStructList[fa.Rating]{
		{
			name:     "returns mature rating for full data",
			content:  journalsContentWithData,
			expected: fa.RatingMature,
		},
		{
			name:     "returns general rating for empty",
			content:  journalsContentEmpty,
			expected: fa.RatingGeneral,
		},
		{
			name:     "returns adult rating for multiline",
			content:  journalsContentWithMultiline,
			expected: fa.RatingAdult,
		},
	}

	runTests(t, tests, func(tc TemplateContent) fa.Rating {
		return tc.EntryRating()
	})
}

// Test fixtures for NewSubmissionsContent
var (
	submissionsContentWithData = &NewSubmissionsContent{
		ID:           54321,
		Title:        "Beautiful Artwork",
		User:         &fa.FurAffinityUser{},
		Link:         "http://example.com/submission/54321",
		Description:  "This is a beautiful submission",
		ThumbnailUrl: "http://example.com/thumb.jpg",
		FullViewUrl:  "http://example.com/view.jpg",
		Rating:       fa.RatingMature,
		Type:         fa.SubmissionTypeImage,
	}
	submissionsContentEmpty = &NewSubmissionsContent{
		ID:           0,
		Title:        "",
		User:         nil,
		Link:         "",
		Description:  "",
		ThumbnailUrl: "",
		FullViewUrl:  "",
	}
	submissionsContentMinimal = &NewSubmissionsContent{
		ID:          999,
		Title:       "Simple Work",
		User:        &fa.FurAffinityUser{},
		Link:        "http://example.com/submission/999",
		Description: "A simple description",
	}
)

func TestNewSubmissionsContent_EntryID(t *testing.T) {
	tests := TestStructList[uint]{
		{
			name:     "returns correct ID",
			content:  submissionsContentWithData,
			expected: 54321,
		},
		{
			name:     "returns zero ID",
			content:  submissionsContentEmpty,
			expected: 0,
		},
		{
			name:     "returns ID for minimal",
			content:  submissionsContentMinimal,
			expected: 999,
		},
	}

	runTests(t, tests, func(tc TemplateContent) uint {
		return tc.EntryID()
	})
}

func TestNewSubmissionsContent_EntryTitle(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct title",
			content:  submissionsContentWithData,
			expected: "Beautiful Artwork",
		},
		{
			name:     "returns empty title",
			content:  submissionsContentEmpty,
			expected: "",
		},
		{
			name:     "returns title for minimal",
			content:  submissionsContentMinimal,
			expected: "Simple Work",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryTitle()
	})
}

func TestNewSubmissionsContent_EntryContent(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns description as content",
			content:  submissionsContentWithData,
			expected: "This is a beautiful submission",
		},
		{
			name:     "returns empty description",
			content:  submissionsContentEmpty,
			expected: "",
		},
		{
			name:     "returns description for minimal",
			content:  submissionsContentMinimal,
			expected: "A simple description",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryContent()
	})
}

func TestNewSubmissionsContent_EntryType(t *testing.T) {
	tests := TestStructList[entries.EntryType]{
		{
			name:     "returns EntryTypeSubmission for full data",
			content:  submissionsContentWithData,
			expected: entries.EntryTypeSubmission,
		},
		{
			name:     "returns EntryTypeSubmission for empty",
			content:  submissionsContentEmpty,
			expected: entries.EntryTypeSubmission,
		},
		{
			name:     "returns EntryTypeSubmission for minimal",
			content:  submissionsContentMinimal,
			expected: entries.EntryTypeSubmission,
		},
	}

	runTests(t, tests, func(tc TemplateContent) entries.EntryType {
		return tc.EntryType()
	})
}

func TestNewSubmissionsContent_ViewLink(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns correct link",
			content:  submissionsContentWithData,
			expected: "http://example.com/submission/54321",
		},
		{
			name:     "returns empty link",
			content:  submissionsContentEmpty,
			expected: "",
		},
		{
			name:     "returns link for minimal",
			content:  submissionsContentMinimal,
			expected: "http://example.com/submission/999",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.ViewLink()
	})
}

func TestNewSubmissionsContent_EntryRating(t *testing.T) {
	tests := TestStructList[fa.Rating]{
		{
			name:     "returns mature rating for full data",
			content:  submissionsContentWithData,
			expected: fa.RatingMature,
		},
		{
			name:     "returns general rating for empty",
			content:  submissionsContentEmpty,
			expected: fa.RatingGeneral,
		},
		{
			name:     "returns general rating for minimal",
			content:  submissionsContentMinimal,
			expected: fa.RatingGeneral,
		},
	}

	runTests(t, tests, func(tc TemplateContent) fa.Rating {
		return tc.EntryRating()
	})
}

// Test fixtures for NewCommentsContent
var (
	commentsContentOnSubmission = &NewCommentsContent{
		ID:      11111,
		OnEntry: "On Submission #54321",
		User:    &fa.FurAffinityUser{},
		Content: "This is a great submission!",
		Link:    "http://example.com/submission/54321#comment-11111",
		Type:    entries.EntryTypeSubmissionComment,
		Rating:  fa.RatingMature,
	}
	commentsContentOnJournal = &NewCommentsContent{
		ID:      22222,
		OnEntry: "On Journal Entry",
		User:    &fa.FurAffinityUser{},
		Content: "Nice journal post!",
		Link:    "http://example.com/journal/98765#comment-22222",
		Type:    entries.EntryTypeJournalComment,
	}
	commentsContentOnNote = &NewCommentsContent{
		ID:      33333,
		OnEntry: "On Note",
		User:    nil,
		Content: "Thanks for the note",
		Link:    "http://example.com/note#comment-33333",
		Type:    entries.EntryTypeNote,
	}
	commentsContentEmpty = &NewCommentsContent{
		ID:      0,
		OnEntry: "",
		User:    nil,
		Content: "",
		Link:    "",
		Type:    entries.EntryTypeSubmission,
	}
)

func TestNewCommentsContent_EntryID(t *testing.T) {
	tests := TestStructList[uint]{
		{
			name:     "returns ID for submission comment",
			content:  commentsContentOnSubmission,
			expected: 11111,
		},
		{
			name:     "returns ID for journal comment",
			content:  commentsContentOnJournal,
			expected: 22222,
		},
		{
			name:     "returns ID for note comment",
			content:  commentsContentOnNote,
			expected: 33333,
		},
		{
			name:     "returns zero ID for empty",
			content:  commentsContentEmpty,
			expected: 0,
		},
	}

	runTests(t, tests, func(tc TemplateContent) uint {
		return tc.EntryID()
	})
}

func TestNewCommentsContent_EntryTitle(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns OnEntry for submission comment",
			content:  commentsContentOnSubmission,
			expected: "On Submission #54321",
		},
		{
			name:     "returns OnEntry for journal comment",
			content:  commentsContentOnJournal,
			expected: "On Journal Entry",
		},
		{
			name:     "returns OnEntry for note comment",
			content:  commentsContentOnNote,
			expected: "On Note",
		},
		{
			name:     "returns empty OnEntry",
			content:  commentsContentEmpty,
			expected: "",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryTitle()
	})
}

func TestNewCommentsContent_EntryContent(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns content for submission comment",
			content:  commentsContentOnSubmission,
			expected: "This is a great submission!",
		},
		{
			name:     "returns content for journal comment",
			content:  commentsContentOnJournal,
			expected: "Nice journal post!",
		},
		{
			name:     "returns content for note comment",
			content:  commentsContentOnNote,
			expected: "Thanks for the note",
		},
		{
			name:     "returns empty content",
			content:  commentsContentEmpty,
			expected: "",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.EntryContent()
	})
}

func TestNewCommentsContent_EntryType(t *testing.T) {
	tests := TestStructList[entries.EntryType]{
		{
			name:     "returns submission comment type",
			content:  commentsContentOnSubmission,
			expected: entries.EntryTypeSubmissionComment,
		},
		{
			name:     "returns journal comment type",
			content:  commentsContentOnJournal,
			expected: entries.EntryTypeJournalComment,
		},
		{
			name:     "returns note type",
			content:  commentsContentOnNote,
			expected: entries.EntryTypeNote,
		},
		{
			name:     "returns submission type for empty",
			content:  commentsContentEmpty,
			expected: entries.EntryTypeSubmission,
		},
	}

	runTests(t, tests, func(tc TemplateContent) entries.EntryType {
		return tc.EntryType()
	})
}

func TestNewCommentsContent_ViewLink(t *testing.T) {
	tests := TestStructList[string]{
		{
			name:     "returns link for submission comment",
			content:  commentsContentOnSubmission,
			expected: "http://example.com/submission/54321#comment-11111",
		},
		{
			name:     "returns link for journal comment",
			content:  commentsContentOnJournal,
			expected: "http://example.com/journal/98765#comment-22222",
		},
		{
			name:     "returns link for note comment",
			content:  commentsContentOnNote,
			expected: "http://example.com/note#comment-33333",
		},
		{
			name:     "returns empty link",
			content:  commentsContentEmpty,
			expected: "",
		},
	}

	runTests(t, tests, func(tc TemplateContent) string {
		return tc.ViewLink()
	})
}

func TestNewCommentsContent_EntryRating(t *testing.T) {
	tests := TestStructList[fa.Rating]{
		{
			name:     "returns mature rating for submission comment",
			content:  commentsContentOnSubmission,
			expected: fa.RatingMature,
		},
		{
			name:     "returns general rating for empty",
			content:  commentsContentEmpty,
			expected: fa.RatingGeneral,
		},
	}

	runTests(t, tests, func(tc TemplateContent) fa.Rating {
		return tc.EntryRating()
	})
}

func runTests[T any](t *testing.T, tests TestStructList[T], resultFunc func(tc TemplateContent) T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resultFunc(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}
