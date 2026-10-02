package bind_test

import (
	"testing"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

func ptr[T any](v T) *T { return &v }

var words = bind.RailWords{Form: "rail", Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}

func TestTop(t *testing.T) {
	brand := w.Brand{Title: "Contacts"}
	bar := view.TopBar{
		Search: view.SearchBox{Query: "anna", Action: "/app/search", Name: "q"},
		New: view.Action{Label: "New note", Icon: "i-quill", Href: "/app/notes/quick", Method: "GET", Enabled: true, Primary: true,
			Arrow: []view.MenuItem{{Action: view.Action{Label: "Note", Icon: "i-quill", Href: "/n", Enabled: true}}, {Action: view.Action{Label: "Contact", Href: "/c", Enabled: true}, Separator: true}}},
		Signed:   view.SignedIn{Login: "mix"},
		UserHref: "/app/settings/account",
		Refresh:  view.Refresh{Action: view.Action{ID: "poll.again", Label: "Refresh", Icon: "i-refresh", Href: "/app/contacts?again=1", Method: "GET", Enabled: true}, Answered: "2 min ago"},
	}
	same(t, "the top bar", bind.Top(bar, bind.TopWords{Brand: brand, Search: "Search everything…"}), w.Top{
		Brand:  brand,
		Search: &w.Search{Query: w.Query{Value: "anna", Placeholder: "Search everything…", Label: "Search everything…", Name: "q"}, Action: "/app/search"},
		Actions: []w.TopAction{
			w.TopDrop{
				Trigger: w.Button{Label: "New note", Icon: "quill", Kind: w.ButtonPrimary, Href: "/app/notes/quick", Menu: true},
				Menu:    w.Menu{Items: []w.MenuPart{w.MenuItem{Icon: "quill", Label: "Note", Href: "/n"}, w.MenuSep{}, w.MenuItem{Label: "Contact", Href: "/c"}}, Pop: w.MenuUnder},
			},
			w.Button{Label: "2 min ago", Icon: "refresh", Href: "/app/contacts?again=1"},
			w.UserMenu{Name: "mix", Href: "/app/settings/account"},
		},
	})
	bar.New.Arrow, bar.Refresh = nil, view.Refresh{}
	got := bind.Top(bar, bind.TopWords{Brand: brand})
	same(t, "no arrow, no refresh", got.Actions, []w.TopAction{
		w.Button{Label: "New note", Icon: "quill", Kind: w.ButtonPrimary, Href: "/app/notes/quick"},
		w.UserMenu{Name: "mix", Href: "/app/settings/account"},
	})
	// The account's menu: the button opens it (a closed menu, without a
	// script too) — who is signed in and where, the items; Sign out posts its
	// form, with the session's token.
	bar.Signed.Instance = "carrel.example"
	bar.User = []view.MenuItem{
		{Action: view.Action{ID: "user.settings", Label: "Settings", Icon: "i-gear", Href: "/app/settings/account", Method: "GET", Enabled: true}},
		{Action: view.Action{ID: "user.logout", Label: "Sign out", Href: "/logout", Method: "POST", Enabled: true, CSRF: "c"}, Separator: true},
	}
	got = bind.Top(bar, bind.TopWords{Brand: brand})
	same(t, "the account's menu", got.Actions[1], w.TopAction(w.TopDrop{
		Trigger: w.UserMenu{Name: "mix"},
		Menu: w.Menu{Items: []w.MenuPart{
			w.MenuHeading{Rubric: w.Rubric{Text: "mix", Items: []w.RubricPart{w.Mono{Text: "carrel.example", Variant: w.MonoTightPlain}}, Form: w.RubricItem, Variant: w.RubricItemLine}, Form: w.MenuHeadingSplit},
			w.MenuItem{Icon: "gear", Label: "Settings", Href: "/app/settings/account"},
			w.MenuSep{},
			w.MenuItem{Label: "Sign out", Form: "user.logout"},
		}, Width: w.MenuWidth290},
		End: true,
	}))
	same(t, "the forms of the top bar", bind.TopForms(bar), []w.PostForm{{ID: "user.logout", Action: "/logout", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}}})
}

func TestMenuItems(t *testing.T) {
	items := []view.MenuItem{
		{Action: view.Action{Label: "Архив", Href: "/a/new", Enabled: true}, Colour: "#a05a2c"},
		{Action: view.Action{Label: "Контакты", Href: "/k/new", Enabled: true, On: true}, Colour: "#4a6b52", Aside: "last used"},
		{Action: view.Action{Label: "Команда", Href: "/t/new", Reason: "Read-only"}, Colour: "#6b4e7a", ReadOnly: true},
		{Action: view.Action{Label: "Delete", Icon: "i-trash", Href: "/d", Enabled: true, Danger: true}, Separator: true},
		{Action: view.Action{ID: "record.delete", Label: "Delete now", Href: "/d?etag=e", Method: "POST", Enabled: true, Danger: true, CSRF: "c"}},
		{Action: view.Action{ID: "record.off", Label: "Off", Href: "/o", Method: "POST", Reason: "No"}},
	}
	same(t, "the items", bind.MenuItems(items, "ro"), []w.MenuPart{
		w.MenuItem{Label: "Архив", Href: "/a/new", Mark: &w.Mark{Colour: "#a05a2c"}},
		w.MenuItem{Label: "Контакты", Href: "/k/new", On: true, Mark: &w.Mark{Colour: "#4a6b52"}, Trail: w.Dim{Parts: []w.Inline{w.Text("last used")}}},
		w.MenuItem{Label: "Команда", Reason: "Read-only", Mark: &w.Mark{Colour: "#6b4e7a"}, Trail: w.Badge{Label: "ro"}},
		w.MenuSep{},
		w.MenuItem{Icon: "trash", Label: "Delete", Href: "/d", Tone: w.MenuToneDanger},
		w.MenuItem{Label: "Delete now", Form: "record.delete", Tone: w.MenuToneDanger},
		w.MenuItem{Label: "Off", Reason: "No"},
	})
	same(t, "the forms of the items that post", bind.MenuForms(items), []w.PostForm{{ID: "record.delete", Action: "/d?etag=e", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}}})

	// An item that carries a file (Action.File: «From this computer») is the
	// label of the file input of its form, which the browser opens without
	// a script (Typecase's task 109); «Upload», shown only without a script,
	// sends the file picked — the screen's word.
	attach := []view.MenuItem{
		{Action: view.Action{ID: "attach.add", Label: "From this computer", Icon: "i-up", Href: "/e?op=attach", Method: "POST", Enabled: true, CSRF: "c", File: "file"}},
		{Action: view.Action{ID: "attach.files", Label: "From Files", Icon: "i-file", Href: "/app/files/picker", Method: "GET", Enabled: true}},
		{Action: view.Action{ID: "attach.off", Label: "Off", Href: "/o", Method: "POST", File: "file", Reason: "No"}},
	}
	same(t, "an item with a file", bind.MenuItems(attach, ""), []w.MenuPart{
		w.MenuItem{Icon: "up", Label: "From this computer", File: &w.FileField{Form: "attach.add", Name: "file", Label: "From this computer"}},
		w.MenuItem{Icon: "file", Label: "From Files", Href: "/app/files/picker"},
		w.MenuItem{Label: "Off", Reason: "No"},
	})
	same(t, "its Upload", bind.Uploads(attach, "Upload"), []w.MenuPart{w.MenuItem{Label: "Upload", NoScript: true, Form: "attach.add"}})
	same(t, "the form of the item with a file", bind.MenuForms(attach), []w.PostForm{{ID: "attach.add", Action: "/e?op=attach", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}, File: "file"}})
}

func TestFileButton(t *testing.T) {
	// A button that carries a file (Action.File: the photo's Replace and Add)
	// is the label of the file input of its form (Typecase's task 109);
	// «Upload» beside it, shown only without a script, sends the file.
	replace := view.Action{ID: "photo.replace", Label: "Replace", Href: "/p?op=upload", Method: "POST", Enabled: true, CSRF: "c", File: "photo"}
	same(t, "a button with a file", bind.FileButton(replace, "photo.replace"),
		w.Button{Label: "Replace", File: &w.FileField{Form: "photo.replace", Name: "photo", Label: "Replace"}})
	same(t, "its Upload", bind.Upload(replace, "photo.replace", "Upload"), &w.Button{Label: "Upload", Form: "photo.replace", NoScript: true})
	crop := view.Action{ID: "photo.crop", Label: "Crop", Href: "/p?op=crop", Method: "POST", Enabled: true}
	same(t, "a button without a file", bind.FileButton(crop, "photo.crop"), w.Button{Label: "Crop", Form: "photo.crop"})
	same(t, "no file: no Upload", bind.Upload(crop, "photo.crop", "Upload"), (*w.Button)(nil))
	replace.Enabled, replace.Reason = false, "Read-only"
	same(t, "switched off: no file", bind.FileButton(replace, "photo.replace"), w.Button{Label: "Replace", State: w.ButtonDisabled, Reason: "Read-only"})
	same(t, "switched off: no Upload", bind.Upload(replace, "photo.replace", "Upload"), (*w.Button)(nil))
}

func TestNavItem(t *testing.T) {
	same(t, "a section with its count", bind.NavItem(view.NavRow{Action: view.Action{Label: "Contacts", Icon: "i-con", Href: "/app/contacts"}, Count: ptr(6)}),
		w.NavItem{Icon: "con", Label: "Contacts", Href: "/app/contacts", Count: &w.Num{Value: 6}})
	same(t, "a section not counted", bind.NavItem(view.NavRow{Action: view.Action{Label: "Files", Icon: "i-file", Href: "/app/files"}}),
		w.NavItem{Icon: "file", Label: "Files", Href: "/app/files"})
}

func TestTickRow(t *testing.T) {
	for name, c := range map[string]struct {
		row   view.SourceRow
		words bind.RailWords
		want  w.TickRow
	}{
		"the combined view, open": {
			view.SourceRow{Href: "/app/contacts", Label: "Combined address book", Count: ptr(6), Tick: view.TickPart, On: true}, words,
			w.TickRow{Ticked: w.TickedPart, Kind: w.TickRowAll, Mark: w.Mark{Kind: w.MarkMulti}, Label: "Combined address book", Meta: "6", Current: true,
				Send: w.Send{Form: "rail", Name: "toggle", Value: "/app/contacts"}},
		},
		// Nothing ticked (states-14): the row of the combined view is ticked
		// off, and stays as it is — it is every source, not one hidden.
		"the combined view, nothing ticked": {
			view.SourceRow{Href: "/app/calendar", Label: "Combined calendar", Tick: view.TickOff, On: true}, words,
			w.TickRow{Ticked: w.TickedOff, Kind: w.TickRowAll, Mark: w.Mark{Kind: w.MarkMulti}, Label: "Combined calendar", Meta: "—", Current: true,
				Send: w.Send{Form: "rail", Name: "toggle", Value: "/app/calendar"}},
		},
		"read-only": {
			view.SourceRow{Href: "/k", Label: "Команда", Colour: "#6b4e7a", Count: ptr(1), Tick: view.TickOn, ReadOnly: true}, words,
			w.TickRow{Mark: w.Mark{Colour: "#6b4e7a"}, Label: "Команда", Meta: "1", Trail: w.Badge{Label: "ro"}, Send: w.Send{Form: "rail", Name: "toggle", Value: "/k"}},
		},
		"ticked off, not counted, on the rail of return": {
			view.SourceRow{Href: "/r", Label: "Рассылки", Colour: "#6b4e7a", Tick: view.TickOff}, bind.RailWords{None: "—"},
			w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#6b4e7a"}, Label: "Рассылки", Meta: "—", Hidden: true},
		},
		"a feed": {
			view.SourceRow{Href: "/f", Label: "Праздники РФ", Colour: "#8c6239", Tick: view.TickOn, Feed: true, ReadOnly: true}, bind.RailWords{ReadOnly: "ro"},
			// The row is the feed's (is-ext), its mark only its colour: no frame
			// draws the mark is-ext in a rail (s-contacts-7, s-agenda-1).
			w.TickRow{Kind: w.TickRowExternal, Mark: w.Mark{Colour: "#8c6239"}, Label: "Праздники РФ", Meta: "ro"},
		},
		"no answer: a mark, the reason its hint, no retry (states-16)": {
			view.SourceRow{Href: "/o", Label: "Общий", Colour: "#9a9280", Tick: view.TickOn, State: view.CollectionUnavailable, Detail: "401 Unauthorized"}, bind.RailWords{None: "—"},
			w.TickRow{Mark: w.Mark{Colour: "#9a9280"}, Label: "Общий", MetaTitle: "401 Unauthorized", Error: true},
		},
	} {
		same(t, name, bind.TickRow(c.row, c.words), c.want)
	}
}

func TestSources(t *testing.T) {
	rail := view.Rail{
		Sections: []view.RailSection{
			{Rows: []view.SourceRow{{Href: "/all", Label: "All", Count: ptr(2), Tick: view.TickOn}}},
			{Account: "nextcloud", Rows: []view.SourceRow{{Href: "/k", Label: "Контакты", Colour: "#4a6b52", Count: ptr(2), Tick: view.TickOn}}},
		},
		Tags: []view.Tag{{Label: "all", Href: "/t", On: true}, {Label: "работа", Href: "/t?r"}},
	}
	all := w.TickRow{Kind: w.TickRowAll, Mark: w.Mark{Kind: w.MarkMulti}, Label: "All", Meta: "2"}
	k := w.TickRow{Mark: w.Mark{Colour: "#4a6b52"}, Label: "Контакты", Meta: "2"}
	tags := w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: "Tags"}}, Tags: []w.Tag{{Label: "all", On: true, Href: "/t"}, {Label: "работа", Href: "/t?r"}}}
	same(t, "a rail that sends nothing", bind.Sources(rail, words), []w.RailPart{
		w.RailSec{Items: []w.RailSecPart{all}},
		w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: "nextcloud"}, k}},
		tags,
	})
	same(t, "no form", bind.RailForm(rail, "rail"), (*w.PostForm)(nil))
	rail.Ticks = &view.Form{Action: "/app/sources/contacts", Sets: []view.FieldSet{{Rows: []view.FormRow{{Kind: view.InputHidden, Name: "_csrf", Value: "c"}}}}}
	all.Send = w.Send{Form: "rail", Name: "toggle", Value: "/all"}
	same(t, "a rail that sends", bind.Sources(rail, words)[0], w.RailPart(w.RailSec{Items: []w.RailSecPart{all}}))
	same(t, "its form", bind.RailForm(rail, "rail"), &w.PostForm{ID: "rail", Action: "/app/sources/contacts", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}})
	// The section of feeds read from outside (RailSection.Feeds): its rubric
	// carries the screen's badge «external» (s-agenda-1, s-contacts-7,
	// s-tasks-1); an account's rubric carries none.
	feeds := view.Rail{Sections: []view.RailSection{
		{Account: "nextcloud", Rows: []view.SourceRow{{Href: "/k", Label: "Личный", Colour: "#4a6b52", Count: ptr(7), Tick: view.TickOn}}},
		{Account: "Subscriptions", Feeds: true, Rows: []view.SourceRow{{Href: "/f", Label: "Праздники РФ", Colour: "#8c6239", Feed: true, ReadOnly: true, Tick: view.TickOn}}},
	}}
	same(t, "the feeds' rubric", bind.Sources(feeds, words), []w.RailPart{
		w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: "nextcloud"}, w.TickRow{Mark: w.Mark{Colour: "#4a6b52"}, Label: "Личный", Meta: "7"}}},
		w.RailSec{Items: []w.RailSecPart{
			w.Rubric{Text: "Subscriptions", Items: []w.RubricPart{w.Badge{Label: "external"}}},
			w.TickRow{Kind: w.TickRowExternal, Mark: w.Mark{Colour: "#8c6239"}, Label: "Праздники РФ", Meta: "ro"},
		}},
	})
}

func TestFoot(t *testing.T) {
	rail := view.Rail{
		Foot:    []view.Action{{Label: "New address book", Icon: "i-plus", Href: "/new", Method: "GET", Enabled: true}},
		About:   view.RailAbout{Action: view.Action{Label: "Carrel", Href: "/about"}, Build: "0.0.1 · 786949a"},
		Davloom: "davloom · running · 0 devices",
	}
	same(t, "the foot", bind.Foot(rail, words), w.RailFoot{
		Items: []w.RailFootPart{w.Button{Icon: "plus", Title: "New address book", Href: "/new"}, w.Emblem{Symbol: "davloom", Title: "davloom · running · 0 devices"}},
		About: &w.About{Name: "Carrel", Build: w.Mono{Text: "0.0.1 · 786949a"}, Href: "/about"},
	})
	rail.Davloom, rail.Foot = "", nil
	same(t, "davloom off", bind.Foot(rail, words).Items, []w.RailFootPart(nil))
}

func TestRunbar(t *testing.T) {
	done := &view.PollBar{Percent: 100, Sources: []view.SourceProgress{{State: view.SourceDone}, {State: view.SourceEmpty}}}
	same(t, "a poll finished clean", bind.Runbar(done), (*w.Runbar)(nil))
	same(t, "no poll", bind.Runbar(nil), (*w.Runbar)(nil))
	failed := &view.PollBar{Percent: 83, Summary: view.Runs{{Text: "Polled 5 of 6 · matched by "}, {Text: "a@b", Mark: view.RunMono}, {Text: " · "}, {Text: "baikal unavailable", Mark: view.RunDanger}},
		Sources: []view.SourceProgress{{State: view.SourceDone}, {State: view.SourceError}}}
	same(t, "a source did not answer", bind.Runbar(failed), &w.Runbar{Meter: w.Meter{Percent: 83}, Parts: []w.RunbarPart{
		w.Text("Polled 5 of 6 · matched by "), w.Mono{Text: "a@b"}, w.Text(" · "), w.Span{Face: w.FaceDanger, Text: "baikal unavailable"},
	}})
	same(t, "running", bind.Running(&view.PollBar{Running: true}), true)
	// What the bar says on its right (Aside, s-agenda-1: «from cache · 2 min
	// ago», states-13: how long the poll runs), monospaced and pushed to the
	// end.
	failed.Aside = "from cache · 2 min ago"
	same(t, "its right", bind.Runbar(failed).Parts[4:], []w.RunbarPart{w.Mono{Text: "from cache · 2 min ago", Place: w.Pushed}})

	// What did not answer is the red «N failed» with its menu of refusals
	// (states-16): each source's mark, name, what was asked and answered, its
	// Retry; Retry all when there are more than one.
	retry := func(src string) *view.Action {
		return &view.Action{ID: "source.retry", Label: "Retry", Href: "/retry?source=" + src, Method: "POST", Enabled: true}
	}
	two := &view.PollBar{Percent: 66, Summary: view.Runs{{Text: "Polled 4 of 6 sources · 9 events"}}, Failed: "2 failed",
		RetryAll: &view.Action{ID: "poll.retry", Label: "Retry all", Href: "/retry", Method: "POST", Enabled: true},
		Sources: []view.SourceProgress{{State: view.SourceDone},
			{Stamp: "baikal · Общий", Colour: "#9a9280", State: view.SourceError, Answer: "PROPFIND /c/ → 401 Unauthorized", Retry: retry("b")},
			{Stamp: "radicale · vps · Проекты", Colour: "#6b4e7a", State: view.SourceError, Answer: "REPORT /p/ → 503 Service Unavailable", Retry: retry("r")}}}
	same(t, "two refused", bind.Runbar(two), &w.Runbar{Meter: w.Meter{Percent: 66}, Parts: []w.RunbarPart{
		w.Text("Polled 4 of 6 sources · 9 events"), w.Text(" · "),
		w.Drop{
			Trigger: w.Button{Label: "2 failed", Menu: true, Kind: w.ButtonDanger, Small: true},
			Menu: w.Menu{Width: w.MenuWidth420, Pop: w.MenuUnderEnd, Items: []w.MenuPart{
				w.MenuRefusal{Mark: w.Mark{Colour: "#9a9280"}, Source: "baikal · Общий", Answer: "PROPFIND /c/ → 401 Unauthorized",
					Retry: w.Button{Label: "Retry", Small: true, Form: "source.retry/retry?source=b"}},
				w.MenuRefusal{Mark: w.Mark{Colour: "#6b4e7a"}, Source: "radicale · vps · Проекты", Answer: "REPORT /p/ → 503 Service Unavailable",
					Retry: w.Button{Label: "Retry", Small: true, Form: "source.retry/retry?source=r"}},
				w.MenuFoot{Items: []w.Button{{Label: "Retry all", Small: true, Form: "poll.retry/retry"}}},
			}},
		},
	}})
	same(t, "their forms", len(bind.PollForms(two)), 3)
}

func TestBadgeAvatar(t *testing.T) {
	same(t, "a linked badge", bind.Badge(view.Badge{Label: "linked ×2", Role: view.BadgeLinked}), w.Badge{Label: "linked ×2", Tone: w.BadgeLinked})
	same(t, "a plain badge", bind.Badge(view.Badge{Label: "x", Role: view.BadgePlan}), w.Badge{Label: "x"})
	same(t, "a photo", bind.Avatar(view.Avatar{Href: "/p", Initials: "АБ"}), w.Avatar{Initials: "АБ", Src: "/p"})
}

func TestFieldsItem(t *testing.T) {
	same(t, "a phone", bind.FieldsItem(view.Field{Label: "Phone", Value: "+7 912", Href: "tel:+7912", Mono: true}, "· "),
		w.FieldsItem{Term: "Phone", Value: []w.ValuePart{w.Text("+7 912")}, Face: w.ValueMono, Href: "tel:+7912"})
	bday := view.Field{Label: "Birthday", Value: "1987-03-14", Mono: true, Aside: "in 7 months"}
	same(t, "the birthday", bind.FieldsItem(bday, "· "),
		w.FieldsItem{Term: "Birthday", Value: []w.ValuePart{w.Text("1987-03-14 "), w.Dim{Parts: []w.Inline{w.Text("· in 7 months")}}}, Face: w.ValueMono})
	same(t, "no aside where the frame draws none", bind.FieldsItem(bday, ""),
		w.FieldsItem{Term: "Birthday", Value: []w.ValuePart{w.Text("1987-03-14")}, Face: w.ValueMono})
	same(t, "the tags", bind.FieldsItem(view.Field{Label: "Tags", Tags: []view.Tag{{Label: "a", Href: "/a"}, {Label: "b"}}}, "· "),
		w.FieldsItem{Term: "Tags", Value: []w.ValuePart{w.Tag{Label: "a", Href: "/a"}, w.Text(" "), w.Tag{Label: "b"}}})
	// A value in its pieces (Runs: the time monospaced, s-agenda-2) and the
	// badge after it (Badge: «created 12:00 Asia/Yekaterinburg», «until
	// 2026-12-28»), a space before the badge.
	when := view.Field{Label: "When", Runs: view.Runs{{Text: "10:00 – 11:00", Mark: view.RunMono}, {Text: ", Monday 10 August"}}, Badge: view.Badge{Label: "created 12:00 Asia/Yekaterinburg"}}
	same(t, "a value in pieces with its badge", bind.FieldsItem(when, "· "), w.FieldsItem{Term: "When", Value: []w.ValuePart{
		w.Mono{Text: "10:00 – 11:00"}, w.Text(", Monday 10 August "), w.Badge{Label: "created 12:00 Asia/Yekaterinburg"},
	}})
	// A value of one monospaced piece is the line's monospaced face, as the
	// panel of a task draws its due date (s-tasks-1); with more pieces —
	// overdue in alarm after it (s-tasks-2) — each piece in its face.
	due := view.Field{Label: "Due", Runs: view.Runs{{Text: "12 August 2026, 18:00", Mark: view.RunMono}}}
	same(t, "a value of one monospaced piece", bind.FieldsItem(due, "· "), w.FieldsItem{Term: "Due", Value: []w.ValuePart{w.Text("12 August 2026, 18:00")}, Face: w.ValueMono})
	due.Runs = append(due.Runs, view.Run{Text: " · overdue by 3 days", Mark: view.RunDanger})
	same(t, "a value of pieces", bind.FieldsItem(due, "· "), w.FieldsItem{Term: "Due", Value: []w.ValuePart{
		w.Mono{Text: "12 August 2026, 18:00"}, w.Span{Face: w.FaceDanger, Text: " · overdue by 3 days"},
	}})
	same(t, "a value with its badge", bind.FieldsItem(view.Field{Label: "Repeats", Value: "Weekly on Mon", Badge: view.Badge{Label: "until 2026-12-28"}}, "· "),
		w.FieldsItem{Term: "Repeats", Value: []w.ValuePart{w.Text("Weekly on Mon "), w.Badge{Label: "until 2026-12-28"}}})
	same(t, "a book", bind.Book(view.Field{Label: "radicale · Команда", Value: "Phone differs", Href: "/c", Colour: "#6b4e7a"}, "· "), []w.SecItemPart{
		w.Mark{Colour: "#6b4e7a", Size: w.MarkH11Inline}, w.Text(" "), w.Link{Text: "radicale · Команда", Href: "/c"}, w.Text(" "), w.Dim{Parts: []w.Inline{w.Text("· Phone differs")}},
	})
}

func TestActions(t *testing.T) {
	same(t, "a segment", bind.SegOption(view.Action{Label: "All", Tally: "24", Href: "/t", On: true}), w.SegOption{Label: "All", Count: "24", Href: "/t", On: true})
	same(t, "a submit", bind.Submit(view.Action{Label: "Show", Href: "/t", Method: "GET", Primary: true}), w.Button{Label: "Show", Kind: w.ButtonPrimary, Submit: true})
	same(t, "a form of an action", bind.ActionForm(view.Action{Href: "/d?etag=e1", Method: "POST"}, "delete"), w.PostForm{ID: "delete", Action: "/d?etag=e1"})
	same(t, "a form of an action with the session's token", bind.ActionForm(view.Action{Href: "/d?etag=e1", Method: "POST", CSRF: "c"}, "delete"),
		w.PostForm{ID: "delete", Action: "/d?etag=e1", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}})
	// An action that carries a file (Action.File: Attach, «From this
	// computer») posts multipart with its file field.
	same(t, "a form of an action with a file", bind.ActionForm(view.Action{Href: "/e?op=attach", Method: "POST", CSRF: "c", File: "file"}, "attach.add"),
		w.PostForm{ID: "attach.add", Action: "/e?op=attach", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}, File: "file"})
	poll := &view.PollBar{Failed: "1 failed", Sources: []view.SourceProgress{{Answer: "REPORT /o → 503",
		Retry: &view.Action{ID: "source.retry", Href: "/retry?source=o", Method: "POST", Enabled: true, CSRF: "c"}}}}
	same(t, "a retry's form, with the token", bind.PollForms(poll), []w.PostForm{{ID: "source.retry/retry?source=o", Action: "/retry?source=o", Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}}})
	same(t, "a technical block", bind.Diag(bind.DiagLine{Name: "UID", Value: "4f2b"}, bind.DiagLine{Name: "Read", Value: "2 min ago"}),
		w.Diag{Text: "UID   4f2b\nRead  2 min ago"})
}

func TestRepeating(t *testing.T) {
	remove := view.FormRow{Kind: view.InputRemove, Name: "remove", Label: "Remove", Value: "tel.0"}
	same(t, "Remove", bind.Remove(remove), w.Button{Label: "Remove", Kind: w.ButtonDanger, Submit: true, Name: "remove", Value: "tel.0"})
	add := view.FormRow{Kind: view.InputAdd, Name: "add", Label: "Add phone", Value: "tel", Template: []view.FormRow{{Kind: view.InputText, Name: "tel.{n}"}, remove}}
	btn, tmpl := bind.Add(add, func(r view.FormRow) []w.FieldSetPart { return []w.FieldSetPart{w.Field{Label: r.Name}} })
	same(t, "Add", btn, w.Button{Label: "Add phone", Adds: "add.tel"})
	same(t, "its template", tmpl, w.Template{ID: "add.tel", Items: []w.FieldSetPart{w.Field{Label: "tel.{n}"}, w.Field{Label: "remove"}}})
	loss := view.LossReport{Note: "The change was saved, but:", Properties: []view.Text{"X-A", "X-B"}}
	// A notice in the plain tone, not an alert: the record is written (states-12).
	same(t, "a loss", bind.Loss(loss, w.MsgFlush), w.Msg{Parts: []w.MsgPart{w.Text("The change was saved, but:"), w.Text(" "), w.Mono{Text: "X-A"}, w.Text(", "), w.Mono{Text: "X-B"}}, Space: w.MsgFlush})
}

func TestNoticeDiagAndEmpty(t *testing.T) {
	n := view.Notice{Role: view.NoticeAlert, Title: "Контакты is unavailable", Body: "Nothing comes from it.",
		Details: &view.Diagnostic{Summary: "REPORT /a/", Lines: []view.Text{"nextcloud", "dav: HTTP 503"}},
		Actions: []view.Action{{ID: "source.retry", Label: "Try again", Href: "/k", Method: "GET", Enabled: true}}}
	same(t, "the technical block", bind.NoticeDiag(n), &w.MainDiag{Diag: w.Diag{Text: "REPORT /a/\nnextcloud\ndav: HTTP 503"}})
	same(t, "none", bind.NoticeDiag(view.Notice{}), (*w.MainDiag)(nil))
	msg := w.Msg{Parts: []w.MsgPart{w.Strong("Контакты is unavailable"), w.Text(" "), w.Text("Nothing comes from it."), w.Text(" "),
		w.Button{Label: "Try again", Small: true, Href: "/k"}}, Tone: w.MsgAlert}
	same(t, "a notice, its button small", bind.Notice(n, w.MsgSpacePage), msg)
	same(t, "the notices, each with its technical block", bind.Notices([]view.Notice{n, {Body: "Saved."}}, w.MsgSpacePage), []w.MainPart{
		msg, w.MainDiag{Diag: w.Diag{Text: "REPORT /a/\nnextcloud\ndav: HTTP 503"}}, w.Msg{Parts: []w.MsgPart{w.Text("Saved.")}},
	})
	empty := view.Notice{Title: "This book is empty.", Body: "Add a contact, or import a file of them.", Actions: []view.Action{
		{ID: "record.new", Label: "New contact", Icon: "i-plus", Href: "/new", Method: "GET", Enabled: true, Primary: true},
		{ID: "collection.import", Label: "Import", Href: "/import", Method: "GET", Enabled: true},
	}}
	same(t, "an empty state", bind.EmptyParts(empty), []w.EmptyPart{
		w.Strong("This book is empty."), w.Text(" Add a contact, or import a file of them. "),
		w.Button{Label: "New contact", Icon: "plus", Kind: w.ButtonPrimary, Href: "/new"}, w.Text(" "), w.Button{Label: "Import", Href: "/import"},
	})
	same(t, "words only", bind.EmptyParts(view.Notice{Body: "Nothing here."}), []w.EmptyPart{w.Text("Nothing here.")})
}
