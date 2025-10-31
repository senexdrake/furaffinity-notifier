package tmpl

import "github.com/senexdrake/furaffinity-notifier/internal/fa"

type (
	NewNotesContent struct {
		ID       uint
		Title    string
		UserLink string
		UserName string
		Content  string
		Link     string
	}

	NewCommentsContent struct {
		ID       uint
		OnEntry  string
		UserLink string
		UserName string
		Content  string
		Link     string
	}

	NewJournalsContent struct {
		ID       uint
		Title    string
		UserLink string
		UserName string
		Content  string
		Link     string
	}

	NewSubmissionsContent struct {
		ID           uint
		Title        string
		UserLink     string
		Link         string
		UserName     string
		Description  string
		ThumbnailUrl string
		FullViewUrl  string
		Rating       fa.SubmissionRating
		Type         fa.SubmissionType
	}
)
