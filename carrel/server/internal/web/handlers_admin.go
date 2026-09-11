package web

import (
	"net/http"

	"carrel.dev/carrel/server/view"
)

func (s *Server) getAdminUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminUsersPage{
		Shell: s.buildShell(sess, "Users"),
		Rows:  nil,
		Add: []view.Action{
			{ID: "admin.invite", Label: "Invite", Href: "/admin/invites", Method: "GET", Enabled: true},
		},
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminUsers(w http.ResponseWriter, r *http.Request) {
	s.getAdminUsers(w, r)
}

func (s *Server) getAdminInvites(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminInvitesPage{
		Shell: s.buildShell(sess, "Invites"),
		Add: plainForm("/admin/invites", "POST",
			csrfField(sess),
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true},
			view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail},
		),
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminInvites(w http.ResponseWriter, r *http.Request) {
	s.getAdminInvites(w, r)
}

func (s *Server) getAdminSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminSettingsPage{Shell: s.buildShell(sess, "Instance settings")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminSettings(w http.ResponseWriter, r *http.Request) {
	s.getAdminSettings(w, r)
}

func (s *Server) postAdminMailTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	result := view.MailTestResult{OK: false, Reply: text("SMTP is not configured.")}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetMain, result, nil)
		return
	}
	s.notFound(w, r)
}

func (s *Server) getAdminAudit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminAuditPage{Shell: s.buildShell(sess, "Audit log")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getAdminDav(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminDavPage{Shell: s.buildShell(sess, "DAV allowlist")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminDav(w http.ResponseWriter, r *http.Request) {
	s.getAdminDav(w, r)
}

func (s *Server) getAdminInstall(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminInstallPage{Shell: s.buildShell(sess, "Install")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminInstall(w http.ResponseWriter, r *http.Request) {
	s.getAdminInstall(w, r)
}
