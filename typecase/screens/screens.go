package screens

// Screen is one assembled mockup page in the screens demo.
type Screen struct {
	ID    string
	Title string
	Body  func() string
}

// All returns every screen route in stable order.
func All() []Screen {
	return []Screen{
		{ID: "notes", Title: "7.4 Notes", Body: Notes},
		{ID: "files", Title: "7.5 Files", Body: Files},
		{ID: "contacts", Title: "7.1 Contacts", Body: Contacts},
		{ID: "io", Title: "7.9 Import and export", Body: ImportExport},
		{ID: "duplicates", Title: "7.7 Duplicates", Body: Duplicates},
		{ID: "states", Title: "8. States", Body: States},
	}
}

// ByID finds one screen or nil.
func ByID(id string) *Screen {
	for _, s := range All() {
		if s.ID == id {
			cp := s
			return &cp
		}
	}
	return nil
}
