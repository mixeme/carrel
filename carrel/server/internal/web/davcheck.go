package web

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
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
	res, trace, _ := s.checkOf(r)
	return res, trace
}

// checkOf is traceOf with the error discovery ended on: whether the server
// refused the login and password (discover.Refused).
func (s *Server) checkOf(r *http.Request) (*discover.Result, *view.DiagnosticTrace, error) {
	server := strings.TrimSpace(r.FormValue("server"))
	if !validServerURL(server) {
		return nil, &view.DiagnosticTrace{Steps: []view.TraceStep{{Name: "address", State: view.CheckBad,
			Detail: "Enter the full address, starting with https:// or http://."}}, Stopped: "Discovery stopped at the address."}, nil
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
	stopped(trace)
	return res, trace, err
}

// stopped names the step discovery did not get past (s-admin-28): the last
// one that failed.
func stopped(trace *view.DiagnosticTrace) {
	if trace.OK {
		return
	}
	for i := len(trace.Steps) - 1; i >= 0; i-- {
		if trace.Steps[i].State == view.CheckBad {
			trace.Stopped = "Discovery stopped at the " + trace.Steps[i].Name + "."
			return
		}
	}
}

// postSettingsConnectionsTest is Test only beside the connection form: the
// connections page with the dialog as it was typed — the password is not
// given back — and the trace under it, nothing saved. A trace is no zone of
// C-7.2, so the answer is the page, with scripts or without (the lead's
// decision П2, 02.10.2026).
func (s *Server) postSettingsConnectionsTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	f := connectionForm{
		Label:           connectionLabel(r.FormValue("label")),
		Server:          strings.TrimSpace(r.FormValue("server")),
		Login:           strings.TrimSpace(r.FormValue("login")),
		CertFingerprint: strings.TrimSpace(r.FormValue("certFingerprint")),
		NoPassword:      noPasswordMark(r),
		Open:            true,
	}
	// No password and no mark: said at the field, no server asked. Answered
	// into the dialog's zone (the live layer), the field is not drawn again:
	// the zone says it.
	if !f.NoPassword && r.FormValue("password") == "" && f.Login != "" {
		f.PasswordErr = missingPassword
		page := s.connectionsPage(sess, f, nil)
		if render.WantsFragment(r) && render.FragmentTarget(r) == connectAnswer {
			page.Shell.Notices = append(page.Shell.Notices, view.Notice{Role: view.NoticeAlert, Title: "Test not run:", Body: text(missingPassword)})
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	res, trace, err := s.checkOf(r)
	f.Tested = true
	page := s.connectionsPage(sess, f, nil)
	page.Shell.Notices = append([]view.Notice{testOutcome(res, trace, err, primaryLabel(page.Add))}, page.Shell.Notices...)
	page.Trace = trace
	s.writePage(w, r, http.StatusOK, page)
}

// connectAnswer is the zone of the connection's dialog under its head that
// Connect and Test only answer into (C-7.2): the outcome and the trace under
// it.
const connectAnswer = render.TargetConnectAnswer

// testOutcome is what Test only found, over its trace (s-profile-39, -40):
// passed — what was found, that nothing is kept until the dialog's own
// button (Connect, Save), and the steps that had a warning, by name;
// failed — the step discovery stopped at, what to do, that nothing was
// saved.
func testOutcome(res *discover.Result, trace *view.DiagnosticTrace, err error, save string) view.Notice {
	if res != nil && trace != nil && trace.OK {
		found := "Carrel can sign in"
		if n := len(res.Collections); n > 0 {
			found += " and found " + plural(n, "collection")
		}
		body := found + ". Nothing is saved until you press " + save + "."
		// A step by its name, once: .well-known asked for CalDAV and for
		// CardDAV is one step of the trace's.
		var warned []string
		for _, st := range trace.Steps {
			if st.State == view.CheckBad && !slices.Contains(warned, string(st.Name)) {
				warned = append(warned, string(st.Name))
			}
		}
		switch n := len(warned); n {
		case 0:
		case 1:
			body += " One step had a warning: see " + warned[0] + " below."
		default:
			body += fmt.Sprintf(" %d steps had warnings: see %s and %s below.", n, strings.Join(warned[:n-1], ", "), warned[n-1])
		}
		return view.Notice{Role: view.NoticeInfo, Title: "Test passed:", Body: text(body)}
	}
	step := "check"
	if trace != nil {
		for i := len(trace.Steps) - 1; i >= 0; i-- {
			if trace.Steps[i].State == view.CheckBad {
				step = string(trace.Steps[i].Name)
				break
			}
		}
	}
	what := "Check the address and test again."
	if discover.Refused(err) {
		what = "the server refused the login and password. Check them and test again."
		return view.Notice{Role: view.NoticeAlert, Title: "Test failed:", Body: text("discovery stopped at the " + step + ": " + what + " Nothing was saved.")}
	}
	return view.Notice{Role: view.NoticeAlert, Title: "Test failed:", Body: text("discovery stopped at the " + step + ". " + what + " Nothing was saved.")}
}

// primaryLabel is the label of the dialog's own button — Connect, or Save of
// a connection edited —, «Connect» where the page has no dialog.
func primaryLabel(f *view.Form) string {
	if f != nil {
		for _, a := range f.Actions {
			if a.Primary {
				return string(a.Label)
			}
		}
	}
	return "Connect"
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
		{Name: "server", Label: "Server URL", Kind: view.InputURL, Value: text(strings.TrimSpace(r.FormValue("server"))), Required: true, Mono: true,
			Placeholder: "https://dav.example.org/"},
		{Name: "login", Label: "Username", Kind: view.InputText, Value: text(strings.TrimSpace(r.FormValue("login")))},
		// The password is a password field, empty after the check (Typecase's
		// note 80).
		{Name: "password", Label: "Password", Kind: view.InputPassword, Hint: "Asked each time, never kept."},
		// As at the connection (s-profile-2): for a certificate the system
		// does not trust (201).
		{Name: "certFingerprint", Label: "Certificate fingerprint", LabelAside: "SHA-256 · optional", Kind: view.InputText,
			Value: text(strings.TrimSpace(r.FormValue("certFingerprint"))), Mono: true,
			Placeholder: "only for a certificate the system does not trust"},
	}
	page.Actions = []view.Action{{ID: "admin.dav.test", Label: "Test discovery", Method: "POST", Primary: true, Enabled: true,
		Href: s.postQuery("/admin/dav", url.Values{"op": {"test"}})}}
	// The second stage stands under the trace, out of the form and naming it
	// (s-admin-5): only after discovery went through.
	if trace != nil && trace.OK {
		page.Agree = &view.FormRow{Name: "agree", Label: "Create test objects, read them back and delete them", Kind: view.InputCheck,
			Error: text(agreeErr),
			Hint:  "The second stage writes to this server: one event, task, contact and file, each deleted after it is read. It is written to the audit log."}
		page.Actions = append(page.Actions, view.Action{ID: "admin.dav.exercise", Label: "Run the full check", Method: "POST", Enabled: true, Danger: true,
			Href: s.postQuery("/admin/dav", url.Values{"op": {"exercise"}})})
	}
	page.Form = view.Form{Action: s.route("/admin/dav"), Method: "POST",
		Sets: []view.FieldSet{{Legend: "Discovery without saving credentials", Rows: rows}}}
	return page
}
