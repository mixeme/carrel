// Package zone cuts out of a page's view model the part one target of C-7.2
// shows.
//
// The server answers a fragment with the whole view model of the page and
// names the target (C-7.1); what that target shows is for the client to cut
// out (docs/plan/client.md, stage 1). Which part each target is — the table
// «Где зона во вью-модели» in C-7.2:
//
//   - a page inside the shell (it embeds view.Shell) has main, rail,
//     details, notices and topbar; poll only where the page carries a
//     PollBar, queue only where it carries a TransferQueue;
//   - a page outside the shell (it embeds view.Plain, C-5.1 and C-5.2) has
//     main only, and main is the whole page: its notices belong to its card;
//   - a sheet has sheet only, and sheet is the whole sheet.
//
// A target the page does not have is an error, not an empty answer: an empty
// fragment is a meaning of its own (C-7.2 — it closes the panel, removes the
// notices, closes the sheet), and a zone that is not on the page must not
// read as one that is empty now. A zone that is there and empty — no details
// panel, no poll bar on one collection, no transfers — comes back as a nil
// pointer.
//
// The package reads view models and returns parts of them; it draws nothing.
package zone

import (
	"errors"
	"fmt"
	"reflect"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Errors of Of. Each comes wrapped with the model's type and the target.
var (
	// ErrUnknownTarget: the name is not one of C-7.2. The list is closed.
	ErrUnknownTarget = errors.New("not a target of C-7.2")
	// ErrNoZone: the target is a real one, but this page has no such zone.
	ErrNoZone = errors.New("the page has no such zone")
	// ErrNotPage: the model is neither a page (in or outside the shell) nor
	// a sheet — a part already cut, or no view model at all.
	ErrNotPage = errors.New("not a page or sheet view model")
)

// Kind is what a view model is to the zones.
type Kind int

const (
	NotPage   Kind = iota
	ShellPage      // embeds view.Shell
	PlainPage      // embeds view.Plain: outside the shell, C-5.1 and C-5.2
	Sheet          // a sheet of C-5: lies over the screen
)

func (k Kind) String() string {
	switch k {
	case ShellPage:
		return "page"
	case PlainPage:
		return "plain page"
	case Sheet:
		return "sheet"
	}
	return "not a page"
}

var (
	shellType   = reflect.TypeFor[view.Shell]()
	plainType   = reflect.TypeFor[view.Plain]()
	detailsType = reflect.TypeFor[view.Details]()
	pollType    = reflect.TypeFor[view.PollBar]()
	queueType   = reflect.TypeFor[view.TransferQueue]()
)

// sheets are the view models C-5 answers as a sheet. Nothing in their shape
// tells a sheet from a fragment, so they are named; a test holds this list
// against the «лист» rows of C-5.
var sheets = map[reflect.Type]bool{
	reflect.TypeFor[view.PublishSheet]():          true,
	reflect.TypeFor[view.MailSheet]():             true,
	reflect.TypeFor[view.MailAttachSheet]():       true,
	reflect.TypeFor[view.MoveSheet]():             true,
	reflect.TypeFor[view.CopiesSheet]():           true,
	reflect.TypeFor[view.AskSheet]():              true,
	reflect.TypeFor[view.RewriteSheet]():          true,
	reflect.TypeFor[view.ExportSheet]():           true,
	reflect.TypeFor[view.NewSheet]():              true,
	reflect.TypeFor[view.FileOpSheet]():           true,
	reflect.TypeFor[view.FilePickerSheet]():       true,
	reflect.TypeFor[view.CollectionNewSheet]():    true,
	reflect.TypeFor[view.CollectionRenameSheet](): true,
	reflect.TypeFor[view.CollectionMoveSheet]():   true,
	reflect.TypeFor[view.CollectionDeleteSheet](): true,
	reflect.TypeFor[view.UnlockSheet]():           true,
	reflect.TypeFor[view.EscrowWithdrawSheet]():   true,
	reflect.TypeFor[view.ConnectionRemoveSheet](): true,
	reflect.TypeFor[view.PasswordResetSheet]():    true,
}

// KindOf tells what model is: a page in the shell, a page outside it, a
// sheet, or none of these. A pointer is looked through.
func KindOf(model any) Kind {
	t := reflect.TypeOf(model)
	if t == nil {
		return NotPage
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return NotPage
	}
	if sheets[t] {
		return Sheet
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous {
			continue
		}
		switch f.Type {
		case shellType:
			return ShellPage
		case plainType:
			return PlainPage
		}
	}
	return NotPage
}

// Targets lists the targets model has, in the order of C-7.2.
func Targets(model any) []string {
	var out []string
	for _, t := range render.Targets() {
		if _, err := Of(model, t); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// Of returns the part of model that target shows:
//
//	main     a page in the shell: a copy of it with a zero Shell, of the
//	         model's own type (pointer if model is one) — the zones nested in
//	         the page's own fields (poll, queue, the timeline's card) stay in
//	         it; a page outside the shell: the page itself
//	rail     view.Rail — Shell.Rail
//	details  *view.Details — Shell.Details, or the page's own Details field
//	         where it has one (the timeline's Card); nil: no panel
//	notices  []view.Notice — Shell.Notices
//	poll     *view.PollBar — the page's PollBar; nil: no bar now
//	queue    *view.TransferQueue — the page's queue; nil: none now
//	sheet    the sheet itself
//	topbar   view.TopBar — Shell.Top
func Of(model any, target string) (any, error) {
	if !known(target) {
		return nil, fmt.Errorf("zone: %q: %w", target, ErrUnknownTarget)
	}
	kind := KindOf(model)
	fail := func(err error) (any, error) {
		return nil, fmt.Errorf("zone: %T, target %q: %w", model, target, err)
	}
	switch kind {
	case NotPage:
		return fail(ErrNotPage)
	case Sheet:
		if target == render.TargetSheet {
			return model, nil
		}
		return fail(ErrNoZone)
	case PlainPage:
		if target == render.TargetMain {
			return model, nil
		}
		return fail(ErrNoZone)
	}

	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return fail(ErrNotPage)
		}
		v = v.Elem()
	}
	shell := v.FieldByIndex(shellIndex(v.Type())).Interface().(view.Shell)

	switch target {
	case render.TargetMain:
		out := reflect.New(v.Type())
		out.Elem().Set(v)
		out.Elem().FieldByIndex(shellIndex(v.Type())).SetZero()
		if reflect.TypeOf(model).Kind() == reflect.Pointer {
			return out.Interface(), nil
		}
		return out.Elem().Interface(), nil
	case render.TargetRail:
		return shell.Rail, nil
	case render.TargetNotices:
		return shell.Notices, nil
	case render.TargetTopbar:
		return shell.Top, nil
	case render.TargetDetails:
		own, err := ownField(v, detailsType)
		if err != nil {
			return fail(err)
		}
		if own != nil {
			return own.(*view.Details), nil
		}
		return shell.Details, nil
	case render.TargetPoll:
		own, err := ownField(v, pollType)
		if err != nil {
			return fail(err)
		}
		if own == nil {
			return fail(ErrNoZone)
		}
		return own.(*view.PollBar), nil
	case render.TargetQueue:
		own, err := ownField(v, queueType)
		if err != nil {
			return fail(err)
		}
		if own == nil {
			return fail(ErrNoZone)
		}
		return own.(*view.TransferQueue), nil
	}
	// sheet: a page is not a sheet.
	return fail(ErrNoZone)
}

func known(target string) bool {
	for _, t := range render.Targets() {
		if t == target {
			return true
		}
	}
	return false
}

// shellIndex is the index of the embedded Shell of a page KindOf called a
// ShellPage.
func shellIndex(t reflect.Type) []int {
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.Anonymous && f.Type == shellType {
			return f.Index
		}
	}
	panic(fmt.Sprintf("zone: %s embeds no view.Shell", t))
}

// ownField finds the page's own (not embedded) field of type want or *want
// and returns it as *want: a copy's address for a value field, the pointer
// itself — nil or not — for a pointer field. No such field: nil, nil. Two
// such fields: the zone is ambiguous, and that is an error of the view model.
func ownField(v reflect.Value, want reflect.Type) (any, error) {
	var found []reflect.Value
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous || !f.IsExported() {
			continue
		}
		if f.Type == want || (f.Type.Kind() == reflect.Pointer && f.Type.Elem() == want) {
			found = append(found, v.Field(i))
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
	default:
		return nil, fmt.Errorf("%d fields of type %s: which one is the zone is not said", len(found), want)
	}
	f := found[0]
	if f.Kind() == reflect.Pointer {
		return f.Interface(), nil
	}
	p := reflect.New(want)
	p.Elem().Set(f)
	return p.Interface(), nil
}
