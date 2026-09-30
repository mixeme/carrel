package web

import (
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The check of a DAV server, in two places and one code: Test beside the
// connection form, and the administrator's DAV validator (U-6.13). Both run
// discovery through the guard (S-6, F-13) and save nothing; what each step
// asked and what the server answered is the trace (C-9.1). The validator's
// second stage writes: only by explicit consent, and into the journal.

// traceOf runs discovery for a form, or says why it cannot start.
func (s *Server) traceOf(r *http.Request) (*discover.Result, *view.DiagnosticTrace) {
	server := strings.TrimSpace(r.FormValue("server"))
	if !validServerURL(server) {
		return nil, &view.DiagnosticTrace{Steps: []view.TraceStep{{Name: "address", State: view.CheckBad,
			Detail: "Enter the full address, starting with https:// or http://."}}}
	}
	pin := dav.NormalizeFingerprint(strings.TrimSpace(r.FormValue("certFingerprint")))
	password := r.FormValue("password")
	if noPasswordMark(r) {
		password = "" // the field is off under the mark (s-profile-14)
	}
	res, trace, err := discover.Run(r.Context(), s.guard, server, strings.TrimSpace(r.FormValue("login")), password, pin)
	if trace == nil {
		trace = &view.DiagnosticTrace{}
	}
	if err != nil && len(trace.Steps) == 0 {
		trace.Steps = []view.TraceStep{{Name: "connect", State: view.CheckBad, Detail: text(err.Error())}}
	}
	return res, trace
}

// postSettingsConnectionsTest is Test beside the connection form: the
// trace, nothing saved.
func (s *Server) postSettingsConnectionsTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	if !render.WantsFragment(r) {
		s.notFound(w, r)
		return
	}
	_, trace := s.traceOf(r)
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetMain, *trace, nil)
}

func (s *Server) getAdminDav(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.adminDavPage(sess, r, nil, nil, ""))
}

// postAdminDav tests discovery, or with op=exercise and consent runs the
// mutating stage after it.
func (s *Server) postAdminDav(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	res, trace := s.traceOf(r)
	if r.FormValue("op") != "exercise" || res == nil || !trace.OK {
		s.writePage(w, r, http.StatusOK, s.adminDavPage(sess, r, trace, nil, ""))
		return
	}
	if v := r.FormValue("agree"); v != "on" && v != "true" {
		s.writePage(w, r, http.StatusOK, s.adminDavPage(sess, r, trace, nil, "Tick the box to agree: the check creates objects on this server and deletes them."))
		return
	}
	pin := dav.NormalizeFingerprint(strings.TrimSpace(r.FormValue("certFingerprint")))
	report := discover.Exercise(r.Context(), s.guard, res, strings.TrimSpace(r.FormValue("login")), r.FormValue("password"), pin)
	outcome := "passed"
	if !report.OK {
		outcome = "failed"
	}
	s.users.Journal(sess.AccountID(), "dav.exercise", serverHost(res.BaseURL)+" "+outcome)
	s.writePage(w, r, http.StatusOK, s.adminDavPage(sess, r, trace, &report, ""))
}

func (s *Server) adminDavPage(sess *session.Session, r *http.Request, trace *view.DiagnosticTrace, full *view.ExerciseReport, agreeErr string) view.AdminDavPage {
	page := view.AdminDavPage{Shell: s.buildShell(sess, "DAV validator"), Trace: trace, Full: full}
	page.Rail = s.adminRail(sess, "/admin/dav")
	rows := []view.FormRow{
		csrfField(sess),
		{Name: "server", Label: "Server URL", Kind: view.InputURL, Value: text(strings.TrimSpace(r.FormValue("server"))), Required: true, Mono: true},
		{Name: "login", Label: "Username", Kind: view.InputText, Value: text(strings.TrimSpace(r.FormValue("login")))},
		{Name: "password", Label: "Password", Kind: view.InputPassword,
			Hint: "Asked each time: nothing of this page is kept."},
		{Name: "certFingerprint", Label: "Certificate fingerprint", Kind: view.InputText, Value: text(strings.TrimSpace(r.FormValue("certFingerprint"))), Mono: true,
			Hint: "Only for a server whose certificate is not trusted: its SHA-256, as the trace shows it."},
	}
	page.Actions = []view.Action{{ID: "admin.dav.test", Label: "Test discovery", Method: "POST", Primary: true, Enabled: true,
		Href: s.postQuery("/admin/dav", url.Values{"op": {"test"}})}}
	if trace != nil && trace.OK {
		rows = append(rows, view.FormRow{Name: "agree", Label: "Create test objects, read them back and delete them", Kind: view.InputCheck,
			Error: text(agreeErr),
			Hint:  "The second stage writes to this server: one event, task, contact and file, each deleted after it is read. It is written to the audit log."})
		page.Actions = append(page.Actions, view.Action{ID: "admin.dav.exercise", Label: "Run the full check", Method: "POST", Enabled: true, Danger: true,
			Href: s.postQuery("/admin/dav", url.Values{"op": {"exercise"}})})
	}
	page.Form = view.Form{Action: s.route("/admin/dav"), Method: "POST",
		Sets: []view.FieldSet{{Legend: "Discovery without saving credentials", Rows: rows}}}
	return page
}
