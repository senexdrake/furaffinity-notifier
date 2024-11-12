package tmpl

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
)
