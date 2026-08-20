// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// taskFilter is which tasks the list shows.
const (
	taskFilterOpen = "open"
	taskFilterAll  = "all"
	taskFilterDone = "done"
)

// taskSort is the order of 2.6.B2. The default, "due", is the sort the list
// already had; the others are additional orders over the same loaded set —
// nothing here asks the server for anything a different sort didn't already
// have.
const (
	taskSortDue      = "due"
	taskSortPriority = "priority"
	taskSortChanged  = "changed"
)

type tasksView struct {
	Sources      []sourceRow
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	Filter       string
	Sort         string
	Rows         []taskRow
	Counts       taskCounts
	ReadOnly     bool
	Empty        bool
	NoLists      bool
	PrintDate    string
	SectionRail  sectionRail
	Mode         findMode
}

type taskCounts struct{ Open, Done, Overdue int }

// All is the total already-loaded count behind the "All n" segment of §16 —
// the mockup names it, and it was the one number in Open/Done/All that was
// never computed (2.6.G10).
func (c taskCounts) All() int { return c.Open + c.Done }

type taskRow struct {
	UID      string
	Title    string
	DueLabel string
	Status   string
	Tags     []string
	Percent  int
	Done     bool
	Overdue  bool
	ETag     string
}

// TasksHome shows every ticked task list at once, or the empty state.
func (s *Server) TasksHome(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	rows, err := s.collectionsOfKind(sess, discovery.KindCalendar, account.ViewTasks, dav.CompTodo)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		v := s.View(r, "Tasks")
		v.Data = tasksView{NoLists: true, SectionRail: s.emptyHomeRail(sess, modeTasks)}
		s.Render(w, "tasks.html", v)
		return
	}
	s.sectionFind(w, r, findRequest{Mode: modeTasks}, "tasks.html")
}

// TasksList renders the VTODOs of one collection.
func (s *Server) TasksList(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	filter := taskFilterFrom(r.URL.Query().Get("filter"))
	sortBy := taskSortFrom(r.URL.Query().Get("sort"))
	view, err := s.buildTasks(r.Context(), sess, accountID, collection, colEnc, filter, sortBy)
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}
	if rail, railErr := s.buildSectionRail(sess, findRequest{Mode: modeTasks}, accountID, colEnc); railErr == nil {
		view.SectionRail = rail
	}
	v := s.View(r, "Tasks")
	v.Notice = strings.TrimSpace(r.URL.Query().Get("notice"))
	v.Data = view
	s.Render(w, "tasks.html", v)
}

func taskFilterFrom(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case taskFilterAll:
		return taskFilterAll
	case taskFilterDone:
		return taskFilterDone
	default:
		return taskFilterOpen
	}
}

func taskSortFrom(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case taskSortPriority:
		return taskSortPriority
	case taskSortChanged:
		return taskSortChanged
	default:
		return taskSortDue
	}
}

func (s *Server) buildTasks(ctx context.Context, sess *session.Session, accountID, collection, colEnc, filter, sortBy string) (tasksView, error) {
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		return tasksView{}, err
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		return tasksView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	set, err := p.QueryComponent(ctx, normalizeCollectionPath(col.Path), dav.CompTodo, time.Time{}, time.Time{})
	if err != nil {
		return tasksView{}, err
	}
	loc, now := s.timezone(), time.Now()
	tasks := make([]model.Todo, 0, len(set.Objects))
	etags := make(map[string]string, len(set.Objects))
	for _, obj := range set.Objects {
		task, err := obj.Todo(loc)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
		etags[task.UID] = obj.ETag
	}
	switch sortBy {
	case taskSortPriority:
		sort.SliceStable(tasks, func(i, j int) bool { return lessTaskByPriority(tasks[i], tasks[j]) })
	case taskSortChanged:
		sort.SliceStable(tasks, func(i, j int) bool { return lessTaskByChanged(tasks[i], tasks[j]) })
	default:
		sort.SliceStable(tasks, func(i, j int) bool { return lessTask(tasks[i], tasks[j]) })
	}

	view := tasksView{
		AccountID: accountID, ColEnc: colEnc, Collection: col,
		AccountLabel: accountLabel(*acc), Filter: filter, Sort: sortBy, ReadOnly: col.ReadOnly,
		PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	if rows, listErr := s.collectionsOfKind(sess, discovery.KindCalendar, account.ViewTasks, dav.CompTodo); listErr == nil {
		view.Sources = rows
	}
	for _, task := range tasks {
		if task.Done() {
			view.Counts.Done++
		} else {
			view.Counts.Open++
			if task.Overdue(now) {
				view.Counts.Overdue++
			}
		}
		if !taskMatchesFilter(task, filter) {
			continue
		}
		view.Rows = append(view.Rows, taskRow{
			UID: task.UID, Title: task.DisplayTitle(),
			DueLabel: dueLabel(task, loc, now), Status: taskStatusLabel(task),
			Tags: task.Categories, Percent: task.PercentComplete,
			Done: task.Done(), Overdue: task.Overdue(now), ETag: etags[task.UID],
		})
	}
	view.Empty = len(view.Rows) == 0
	return view, nil
}

func taskMatchesFilter(task model.Todo, filter string) bool {
	switch filter {
	case taskFilterAll:
		return true
	case taskFilterDone:
		return task.Done()
	default:
		return !task.Done()
	}
}

// lessTask puts what still needs doing first, soonest due at the top, and what
// is finished at the bottom.
func lessTask(a, b model.Todo) bool {
	bucketA, dueA := a.SortKey()
	bucketB, dueB := b.SortKey()
	if bucketA != bucketB {
		return bucketA < bucketB
	}
	if !dueA.Equal(dueB) {
		if dueA.IsZero() {
			return false
		}
		if dueB.IsZero() {
			return true
		}
		return dueA.Before(dueB)
	}
	return strings.ToLower(a.DisplayTitle()) < strings.ToLower(b.DisplayTitle())
}

// lessTaskByPriority is the "By priority" order of 2.6.B2: RFC 5545 §3.8.1.9
// numbers 1 (highest) through 9 (lowest), with 0 — no priority set — sorted
// after every task that has one, same as a due date of zero already is.
func lessTaskByPriority(a, b model.Todo) bool {
	pa, pb := a.Priority, b.Priority
	if pa != pb {
		if pa == 0 {
			return false
		}
		if pb == 0 {
			return true
		}
		return pa < pb
	}
	return strings.ToLower(a.DisplayTitle()) < strings.ToLower(b.DisplayTitle())
}

// lessTaskByChanged is the "Recently changed" order of 2.6.B2: LAST-MODIFIED
// descending, with a task that never carried one sorted after every task
// that did.
func lessTaskByChanged(a, b model.Todo) bool {
	ma, mb := a.Modified, b.Modified
	if !ma.Equal(mb) {
		if ma.IsZero() {
			return false
		}
		if mb.IsZero() {
			return true
		}
		return ma.After(mb)
	}
	return strings.ToLower(a.DisplayTitle()) < strings.ToLower(b.DisplayTitle())
}

func dueLabel(task model.Todo, loc *time.Location, now time.Time) string {
	if task.Due.IsZero() {
		return ""
	}
	due := task.Due.In(loc)
	if task.DueDateOnly {
		return due.Format("2 Jan 2006")
	}
	if due.Year() == now.In(loc).Year() {
		return due.Format("2 Jan 15:04")
	}
	return due.Format("2 Jan 2006 15:04")
}

func taskStatusLabel(task model.Todo) string {
	switch task.Status {
	case model.TaskCompleted:
		return "Completed"
	case model.TaskInProcess:
		return "In progress"
	case model.TaskCancelled:
		return "Cancelled"
	case model.TaskNeedsAction, "":
		if task.Done() {
			return "Completed"
		}
		return ""
	default:
		return capitalize(strings.ToLower(task.Status))
	}
}

type taskCardView struct {
	Sources      []sourceRow
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	UID          string
	ETag         string
	Task         model.Todo
	Form         taskForm
	Related      []relatedRow
	Attachments  []attachmentRow
	CanAttach    bool
	Section      string
	ReadOnly     bool
	IsNew        bool
	Source       sourceBlockView
}

type taskForm struct {
	Summary, Description, Location string
	Status, Categories             string
	DueDate, DueTime               string
	StartDate                      string
	Priority, PercentComplete      string
	Related                        string
}

// TaskNew shows and takes the create form.
func (s *Server) TaskNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.taskSave(w, r, true)
		return
	}
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	_, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this task list is read-only", http.StatusForbidden)
		return
	}
	v := s.View(r, "New task")
	v.Data = taskCardView{
		Sources: s.taskSources(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), IsNew: true,
		Form: taskForm{Status: model.TaskNeedsAction, Priority: "0", PercentComplete: "0"},
	}
	s.Render(w, "task.html", v)
}

// TaskCard shows one task and takes its edits.
func (s *Server) TaskCard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.taskSave(w, r, false)
		return
	}
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadTaskCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}
	v := s.View(r, "Task")
	v.Notice = strings.TrimSpace(r.URL.Query().Get("notice"))
	v.Data = card
	s.Render(w, "task.html", v)
}

func (s *Server) taskSources(sess *session.Session) []sourceRow {
	rows, err := s.collectionsOfKind(sess, discovery.KindCalendar, account.ViewTasks, dav.CompTodo)
	if err != nil {
		return nil
	}
	return rows
}

func (s *Server) taskSave(w http.ResponseWriter, r *http.Request, isNew bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this task list is read-only", http.StatusForbidden)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	action := r.PostFormValue(fieldAction)
	if action == "delete" {
		err = p.Delete(ctx, collection, calendarObjectPath(collection, uid), strings.TrimSpace(r.PostFormValue("etag")))
		if err != nil {
			s.handleTaskWriteError(w, r, sess, err, accountID, collection, colEnc, uid)
			return
		}
		http.Redirect(w, r, s.Path("/app/tasks/"+accountID+"/"+colEnc), http.StatusSeeOther)
		return
	}

	var obj *model.Object
	if isNew {
		uid, err = model.NewUID()
		if err == nil {
			obj, err = model.NewTodo(uid)
		}
	} else {
		obj, err = p.Get(ctx, collection, calendarObjectPath(collection, uid))
		if err == nil && strings.TrimSpace(r.PostFormValue("etag")) != "" {
			obj.ETag = strings.TrimSpace(r.PostFormValue("etag"))
		}
	}
	if err != nil {
		s.renderTasksError(w, r, err, accountID, colEnc)
		return
	}

	var patch *model.Patch
	if action == "toggle" {
		// The tick on a list row is a one-property edit, not a rebuilt object.
		patch, err = toggleTaskPatch(obj, s.timezone())
	} else {
		form := parseTaskForm(r)
		patch, err = form.toPatch(s.timezone())
		if err == nil {
			err = obj.Apply(patch)
			patch = nil
		}
		if err != nil {
			v := s.View(r, map[bool]string{true: "New task", false: "Task"}[isNew])
			v.Error = capitalize(err.Error())
			task, _ := obj.Todo(s.timezone())
			v.Data = taskCardView{
				Sources: s.taskSources(sess), AccountID: accountID, ColEnc: colEnc,
				Collection: col, AccountLabel: accountLabel(*acc), UID: task.UID,
				ETag: obj.ETag, Task: task, Form: form, IsNew: isNew,
			}
			s.RenderStatus(w, http.StatusBadRequest, "task.html", v)
			return
		}
	}
	if patch != nil {
		if err = obj.Apply(patch); err != nil {
			s.renderTasksError(w, r, err, accountID, colEnc)
			return
		}
	}

	var result *calendar.WriteResult
	if isNew {
		obj.Path = calendarObjectPath(collection, uid)
		result, err = p.Create(ctx, collection, obj)
	} else {
		result, err = p.Update(ctx, collection, obj)
	}
	if err != nil {
		s.handleTaskWriteError(w, r, sess, err, accountID, collection, colEnc, uid)
		return
	}
	s.rememberDefault(sess, account.ViewTasks, accountID, collection)
	notice := "Task saved."
	if result != nil && result.ReportLoss && !result.Loss.Empty() {
		notice = "Task saved, but " + result.Loss.String() + "."
	}
	if action == "toggle" {
		s.redirectNotice(w, r, s.Path("/app/tasks/"+accountID+"/"+colEnc+"?filter="+taskFilterFrom(r.PostFormValue("filter"))), notice)
		return
	}
	s.redirectNotice(w, r, s.Path("/app/tasks/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), notice)
}

func (s *Server) handleTaskWriteError(w http.ResponseWriter, r *http.Request, sess *session.Session, err error, accountID, collection, colEnc, uid string) {
	if calendar.IsConflict(err) {
		s.showICalConflict(w, r, sess, sectionTasks, accountID, collection, colEnc, uid, err)
		return
	}
	s.renderTasksError(w, r, err, accountID, colEnc)
}

// toggleTaskPatch flips a task between done and not done, touching STATUS,
// COMPLETED and PERCENT-COMPLETE and nothing else.
func toggleTaskPatch(obj *model.Object, loc *time.Location) (*model.Patch, error) {
	task, err := obj.Todo(loc)
	if err != nil {
		return nil, err
	}
	p := &model.Patch{}
	if task.Done() {
		p.SetText(ical.PropStatus, model.TaskNeedsAction)
		p.Remove(ical.PropCompleted)
		p.SetText(ical.PropPercentComplete, "0")
	} else {
		p.SetText(ical.PropStatus, model.TaskCompleted)
		p.SetText(ical.PropCompleted, time.Now().UTC().Format("20060102T150405Z"))
		p.SetText(ical.PropPercentComplete, "100")
	}
	p.SetText(ical.PropLastModified, time.Now().UTC().Format("20060102T150405Z"))
	return p, nil
}

func parseTaskForm(r *http.Request) taskForm {
	return taskForm{
		Summary:         strings.TrimSpace(r.PostFormValue("summary")),
		Description:     strings.TrimSpace(r.PostFormValue("description")),
		Location:        strings.TrimSpace(r.PostFormValue("location")),
		Status:          strings.ToUpper(strings.TrimSpace(r.PostFormValue("status"))),
		Categories:      strings.TrimSpace(r.PostFormValue("categories")),
		DueDate:         strings.TrimSpace(r.PostFormValue("due_date")),
		DueTime:         strings.TrimSpace(r.PostFormValue("due_time")),
		StartDate:       strings.TrimSpace(r.PostFormValue("start_date")),
		Priority:        strings.TrimSpace(r.PostFormValue("priority")),
		PercentComplete: strings.TrimSpace(r.PostFormValue("percent_complete")),
		Related:         strings.TrimSpace(r.PostFormValue("related")),
	}
}

var taskStatuses = map[string]bool{
	model.TaskNeedsAction: true, model.TaskInProcess: true,
	model.TaskCompleted: true, model.TaskCancelled: true,
}

func (f taskForm) toPatch(loc *time.Location) (*model.Patch, error) {
	if loc == nil {
		loc = time.Local
	}
	if strings.TrimSpace(f.Summary) == "" {
		return nil, fmt.Errorf("a task needs a summary")
	}
	p := &model.Patch{}
	p.SetText(ical.PropSummary, f.Summary)
	setTextOrRemove(p, ical.PropDescription, f.Description)
	setTextOrRemove(p, ical.PropLocation, f.Location)
	setTextOrRemove(p, ical.PropCategories, cleanCommaList(f.Categories))

	status := f.Status
	if status == "" {
		status = model.TaskNeedsAction
	}
	if !taskStatuses[status] {
		return nil, fmt.Errorf("unknown task status %q", f.Status)
	}
	p.SetText(ical.PropStatus, status)

	if f.StartDate != "" {
		start, err := time.ParseInLocation("2006-01-02", f.StartDate, loc)
		if err != nil {
			return nil, fmt.Errorf("start date must use YYYY-MM-DD")
		}
		p.Set(ical.PropDateTimeStart, dateValue(start))
	} else {
		p.Remove(ical.PropDateTimeStart)
	}

	switch {
	case f.DueDate == "" && f.DueTime != "":
		return nil, fmt.Errorf("a due time needs a due date")
	case f.DueDate == "":
		p.Remove(ical.PropDue)
	case f.DueTime == "":
		due, err := time.ParseInLocation("2006-01-02", f.DueDate, loc)
		if err != nil {
			return nil, fmt.Errorf("due date must use YYYY-MM-DD")
		}
		p.Set(ical.PropDue, dateValue(due))
	default:
		due, err := time.ParseInLocation("2006-01-02 15:04", f.DueDate+" "+f.DueTime, loc)
		if err != nil {
			return nil, fmt.Errorf("due date and time are invalid")
		}
		p.Set(ical.PropDue, model.Value{
			Text:   due.Format("20060102T150405"),
			Params: map[string][]string{"TZID": {loc.String()}},
		})
	}

	percent, err := boundedNumber(f.PercentComplete, 0, 100)
	if err != nil {
		return nil, fmt.Errorf("percent complete must be between 0 and 100")
	}
	if percent == 0 && status != model.TaskCompleted {
		p.Remove(ical.PropPercentComplete)
	} else {
		p.SetText(ical.PropPercentComplete, strconv.Itoa(percent))
	}
	priority, err := boundedNumber(f.Priority, 0, 9)
	if err != nil {
		return nil, fmt.Errorf("priority must be between 0 and 9")
	}
	if priority == 0 {
		p.Remove(ical.PropPriority)
	} else {
		p.SetText(ical.PropPriority, strconv.Itoa(priority))
	}

	if status == model.TaskCompleted {
		p.SetText(ical.PropCompleted, time.Now().UTC().Format("20060102T150405Z"))
		p.SetText(ical.PropPercentComplete, "100")
	} else {
		p.Remove(ical.PropCompleted)
	}

	if values := model.RelationValues(model.ParseRelations(f.Related)); len(values) > 0 {
		p.Set(ical.PropRelatedTo, values...)
	} else {
		p.Remove(ical.PropRelatedTo)
	}
	p.SetText(ical.PropDateTimeStamp, time.Now().UTC().Format("20060102T150405Z"))
	p.SetText(ical.PropLastModified, time.Now().UTC().Format("20060102T150405Z"))
	return p, nil
}

func formFromTask(task model.Todo, loc *time.Location) taskForm {
	f := taskForm{
		Summary: task.Summary, Description: task.Description, Location: task.Location,
		Status: task.Status, Categories: strings.Join(task.Categories, ", "),
		Priority: strconv.Itoa(task.Priority), PercentComplete: strconv.Itoa(task.PercentComplete),
		Related: strings.Join(model.RelationUIDs(task.Related), ", "),
	}
	if f.Status == "" {
		f.Status = model.TaskNeedsAction
	}
	if !task.Start.IsZero() {
		f.StartDate = task.Start.In(loc).Format("2006-01-02")
	}
	if !task.Due.IsZero() {
		due := task.Due.In(loc)
		f.DueDate = due.Format("2006-01-02")
		if !task.DueDateOnly {
			f.DueTime = due.Format("15:04")
		}
	}
	return f
}

func dateValue(t time.Time) model.Value {
	return model.Value{Text: t.Format("20060102"), Params: map[string][]string{"VALUE": {"DATE"}}}
}

func setTextOrRemove(p *model.Patch, name, value string) {
	if strings.TrimSpace(value) == "" {
		p.Remove(name)
	} else {
		p.SetText(name, value)
	}
}

func boundedNumber(value string, min, max int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return min, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("out of range")
	}
	return n, nil
}

func (s *Server) renderTasksError(w http.ResponseWriter, r *http.Request, err error, accountID, colEnc string) {
	v := s.View(r, "Tasks")
	v.Error = userFacingDAVError(err)
	v.Data = tasksView{Sources: s.taskSources(SessionFrom(r)), AccountID: accountID, ColEnc: colEnc, Filter: taskFilterOpen}
	s.RenderStatus(w, http.StatusBadRequest, "tasks.html", v)
}
