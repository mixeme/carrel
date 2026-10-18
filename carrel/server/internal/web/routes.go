package web

import "net/http"

func (s *Server) registerRoutes() {
	// C-5.1 — outside shell
	s.handle("GET /login", (*Server).getLogin)
	s.handle("POST /login", (*Server).postLogin)
	s.handle("POST /logout", (*Server).postLogout)
	s.handle("GET /setup", (*Server).getSetup)
	s.handle("POST /setup", (*Server).postSetup)
	s.handle("GET /register", (*Server).getRegister)
	s.handle("POST /register", (*Server).postRegister)
	s.handle("GET /invite/{token}", (*Server).getInvite)
	s.handle("GET /password-history/{token}", (*Server).getPasswordHistory)
	s.handle("POST /invite/{token}", (*Server).postInvite)
	s.handle("GET /confirm-email/{token}", (*Server).getConfirmEmail)
	s.handle("GET /forgot", (*Server).getForgot)
	s.handle("POST /forgot", (*Server).postForgot)
	s.handle("GET /forgot/{token}", (*Server).getForgotWipe)
	s.handle("POST /forgot/{token}", (*Server).postForgotWipe)
	s.handle("GET /about", (*Server).getAbout)
	s.handle("GET /healthz", (*Server).getHealthz)

	// C-5.2 — public by secret
	s.handle("GET /p/{token}", (*Server).getPublished)
	s.handle("GET /p/{token}/download", (*Server).getPublishedDownload)
	s.handle("GET /p/{token}/file/{path...}", (*Server).getPublishedFile)
	s.handle("/p/{token}/dav/{path...}", (*Server).publishedDAV)
	s.handle("POST /api/backup/run", (*Server).postBackupRun)
	s.handle("GET /api/backup/status", (*Server).getBackupStatus)
	s.handle("GET /api/backup/{id}/download", (*Server).getBackupDownload)
	s.handle("GET /api/backup/download", (*Server).getBackupDownloadLegacy)

	// C-5.3
	s.handle("GET /app/", (*Server).getAppRoot)
	s.handle("GET /app/search", (*Server).getSearch)
	s.handle("GET /app/search/progress/{task}", (*Server).getSearchProgressSSE)
	s.handle("GET /app/search/progress/{task}/poll", (*Server).getSearchProgressPoll)
	s.handle("POST /app/search/progress/{task}/retry", (*Server).postSearchProgressRetry)
	s.handle("POST /app/search/progress/{task}/cancel", (*Server).postSearchProgressCancel)
	s.handle("GET /app/duplicates", (*Server).getDuplicates)
	s.handle("POST /app/duplicates/decide", (*Server).postDuplicatesDecide)
	s.handle("GET /app/duplicates/merge", (*Server).getDuplicatesMerge)
	s.handle("POST /app/duplicates/merge", (*Server).postDuplicatesMerge)
	s.handle("POST /app/sources/{kind}", (*Server).postSources)
	s.handle("GET /app/publish", (*Server).getPublish)
	s.handle("POST /app/publish", (*Server).postPublish)
	s.handle("GET /app/mail/compose", (*Server).getMailCompose)
	s.handle("POST /app/mail/compose", (*Server).postMailCompose)
	s.handle("GET /app/mail/attach", (*Server).getMailAttach)
	s.handle("POST /app/mail/attach", (*Server).postMailAttach)
	s.handle("GET /app/password", (*Server).getPassword)
	s.handle("POST /app/password", (*Server).postPassword)

	registerKindRoutes(s, "contacts")
	registerKindRoutes(s, "calendar")
	registerKindRoutes(s, "tasks")
	registerKindRoutes(s, "notes")

	// The Columns menu of a list (S-9.6): the sections of records, a
	// person's timeline; files, users and the audit log below.
	for _, segment := range recordSections {
		s.handle("POST /app/"+segment+"/columns", combinedColumnsHandler(segment))
		s.handle("POST /app/"+segment+"/{account}/{col}/columns", collectionColumnsHandler(segment))
	}
	s.handle("POST /app/contacts/{account}/{col}/{uid}/timeline/columns", (*Server).postTimelineColumns)

	s.handle("GET /app/contacts/suggest", (*Server).getContactsSuggest)
	// The suggestions of Related to (s-tasks-10, s-notes-20): the record's
	// own address, «new» for one not yet made.
	for _, kind := range []string{"tasks", "notes"} {
		s.handle("GET /app/"+kind+"/{account}/{col}/{uid}/related", relatedSuggestHandler(kind))
	}
	s.handle("GET /app/contacts/{account}/{col}/{uid}/timeline", (*Server).getContactTimeline)
	s.handle("GET /app/tasks/{account}/{col}/{uid}/done", (*Server).getTaskDone)
	s.handle("POST /app/tasks/{account}/{col}/{uid}/done", (*Server).postTaskDone)
	s.handle("GET /app/contacts/{account}/{col}/{uid}/rewrite", (*Server).getContactRewrite)
	s.handle("POST /app/contacts/{account}/{col}/{uid}/rewrite", (*Server).postContactRewrite)
	s.handle("GET /app/contacts/{account}/{col}/new/photo-preview", (*Server).getNewPhotoPreview)
	s.handle("POST /app/contacts/{account}/{col}/new/photo-preview", (*Server).postNewPhotoPreview)
	s.handle("GET /app/contacts/{account}/{col}/{uid}/photo-preview", (*Server).getPhotoPreview)
	s.handle("POST /app/contacts/{account}/{col}/{uid}/photo-preview", (*Server).postPhotoPreview)
	s.handle("GET /c/{account}/{col}/{uid}/photo", (*Server).getContactPhoto)

	// C-5.5
	s.handle("GET /app/files", (*Server).getFilesRoot)
	s.handle("GET /app/files/{account}/{col}/{path...}", (*Server).getFiles)
	s.handle("GET /app/files/{account}/{col}/download/{path...}", (*Server).getFilesDownload)
	s.handle("POST /app/files/{account}/{col}/download/{path...}", (*Server).postFilesDownload)
	s.handle("POST /app/files/{account}/{col}/upload/{path...}", (*Server).postFilesUpload)
	s.handle("POST /app/files/{account}/{col}/mkcol/{path...}", (*Server).postFilesMkcol)
	s.handle("POST /app/files/{account}/{col}/op/{path...}", (*Server).postFilesOp)
	s.handle("GET /app/files/{account}/{col}/op/{path...}", (*Server).postFilesOp)
	s.handle("POST /app/files/{account}/{col}/columns/{path...}", (*Server).postFilesColumns)
	s.handle("GET /app/files/picker", (*Server).getFilesPicker)
	s.handle("POST /app/files/picker", (*Server).postFilesPicker)
	s.handle("GET /app/files/published", (*Server).getFilesPublished)
	s.handle("GET /app/attach/{account}/{col}/{path...}", (*Server).getAttach)

	// C-5.6
	s.handle("GET /app/collections/new", (*Server).getCollectionNew)
	s.handle("POST /app/collections/new", (*Server).postCollectionNew)
	s.handle("GET /app/collections/rename", (*Server).getCollectionRename)
	s.handle("POST /app/collections/rename", (*Server).postCollectionRename)
	s.handle("GET /app/collections/move", (*Server).getCollectionMove)
	s.handle("POST /app/collections/move", (*Server).postCollectionMove)
	s.handle("GET /app/collections/delete", (*Server).getCollectionDelete)
	s.handle("POST /app/collections/delete", (*Server).postCollectionDelete)

	// C-5.7
	s.handle("GET /app/settings/account", (*Server).getSettingsAccount)
	s.handle("POST /app/settings/account", (*Server).postSettingsAccount)
	s.handle("GET /app/settings/escrow", (*Server).getSettingsEscrow)
	s.handle("POST /app/settings/escrow", (*Server).postSettingsEscrow)
	s.handle("GET /app/settings/escrow/deposit", (*Server).getEscrowDeposit)
	s.handle("GET /app/settings/escrow/withdraw", (*Server).getEscrowWithdraw)
	s.handle("GET /app/settings/connections", (*Server).getSettingsConnections)
	s.handle("POST /app/settings/connections", (*Server).postSettingsConnections)
	s.handle("POST /app/settings/connections/test", (*Server).postSettingsConnectionsTest)
	s.handle("GET /app/settings/connections/disconnect", (*Server).getConnectionDisconnect)
	s.handle("POST /app/settings/connections/disconnect", (*Server).postConnectionDisconnect)
	s.handle("GET /app/settings/attachments", (*Server).getSettingsAttachments)
	s.handle("POST /app/settings/attachments", (*Server).postSettingsAttachments)
	s.handle("GET /app/settings/devices", (*Server).getSettingsDevices)
	s.handle("POST /app/settings/devices", (*Server).postSettingsDevices)
	s.handle("GET /app/settings/devices/profile", (*Server).getSettingsDevicesProfile)
	s.handle("GET /app/settings/davloom", (*Server).getSettingsDavloom)
	s.handle("POST /app/settings/davloom", (*Server).postSettingsDavloom)
	s.handle("GET /app/settings/backup", (*Server).getSettingsBackup)
	s.handle("POST /app/settings/backup", (*Server).postSettingsBackup)
	s.handle("GET /app/settings/appearance", (*Server).getSettingsAppearance)
	s.handle("POST /app/settings/appearance", (*Server).postSettingsAppearance)
	s.handle("GET /app/settings/unlock", (*Server).getSettingsUnlock)
	s.handle("POST /app/settings/unlock", (*Server).postSettingsUnlock)
	s.handle("POST /app/settings/lock", (*Server).postSettingsLock)

	// C-5.8
	s.handle("GET /admin/", (*Server).getAdminUsers)
	s.handle("POST /admin/", (*Server).postAdminUsers)
	s.handle("GET /admin/invites", (*Server).getAdminInvites)
	s.handle("POST /admin/invites", (*Server).postAdminInvites)
	s.handle("GET /admin/delete", (*Server).getAdminDelete)
	s.handle("GET /admin/reset", (*Server).getAdminReset)
	s.handle("POST /admin/reset", (*Server).postAdminReset)
	s.handle("GET /admin/settings", (*Server).getAdminSettings)
	s.handle("POST /admin/settings", (*Server).postAdminSettings)
	s.handle("POST /admin/settings/mail-test", (*Server).postAdminMailTest)
	s.handle("GET /admin/escrow", (*Server).getAdminEscrow)
	s.handle("POST /admin/escrow", (*Server).postAdminEscrow)
	s.handle("GET /admin/audit", (*Server).getAdminAudit)
	s.handle("POST /admin/audit/columns", (*Server).postAuditColumns)
	s.handle("POST /admin/columns", (*Server).postAdminColumns)
	s.handle("GET /admin/dav", (*Server).getAdminDav)
	s.handle("POST /admin/dav", (*Server).postAdminDav)
	s.handle("GET /admin/install", (*Server).getAdminInstall)
	s.handle("POST /admin/install", (*Server).postAdminInstall)
}

func registerKindRoutes(s *Server, kind string) {
	s.handle("GET /app/"+kind, kindRootHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}", kindListHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/{uid}", kindRecordHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/{uid}/edit", kindEditGetHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/{uid}/edit", kindEditPostHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/{uid}/conflict", kindConflictGetHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/{uid}/conflict", kindConflictPostHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/{uid}/move", kindMoveGetHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/{uid}/copies", kindCopiesHandler(kind))
	s.handle("GET /app/"+kind+"/linked/{uid}/copies", kindLinkedCopiesHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/{uid}/move", kindMovePostHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/{uid}/delete", kindDeleteHandler(kind))
	s.handle("GET /app/"+kind+"/import", kindImportEntryHandler(kind))
	s.handle("POST /app/"+kind+"/import", kindImportEntryPostHandler(kind))
	s.handle("GET /app/"+kind+"/export", kindExportEntryHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/import", kindImportGetHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/import", kindImportPostHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/export", kindExportHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/new", kindNewGetHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/new", kindNewPostHandler(kind))
	s.handle("GET /app/"+kind+"/quick", kindQuickEntryHandler(kind))
	s.handle("GET /app/"+kind+"/{account}/{col}/new/quick", kindQuickGetHandler(kind))
	s.handle("POST /app/"+kind+"/{account}/{col}/new/quick", kindQuickPostHandler(kind))
}

func relatedSuggestHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getRelatedSuggest(w, r, kind)
	}
}

func kindRootHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindRoot(w, r, kind)
	}
}

func kindListHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindList(w, r, kind)
	}
}

func kindRecordHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindRecord(w, r, kind)
	}
}

func kindEditGetHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindEdit(w, r, kind)
	}
}

func kindEditPostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindEdit(w, r, kind)
	}
}

func kindConflictGetHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindConflict(w, r, kind)
	}
}

func kindConflictPostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindConflict(w, r, kind)
	}
}

func kindCopiesHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindCopies(w, r, kind)
	}
}

func kindLinkedCopiesHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getLinkedCopies(w, r, kind)
	}
}

func kindMoveGetHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindMove(w, r, kind)
	}
}

func kindMovePostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindMove(w, r, kind)
	}
}

func kindDeleteHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindDelete(w, r, kind)
	}
}

func kindImportEntryHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindImportEntry(w, r, kind)
	}
}

func kindImportEntryPostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindImportEntry(w, r, kind)
	}
}

func kindExportEntryHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindExportEntry(w, r, kind)
	}
}

func kindImportGetHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindImport(w, r, kind)
	}
}

func kindImportPostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindImport(w, r, kind)
	}
}

func kindExportHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindExport(w, r, kind)
	}
}

func kindNewGetHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindNew(w, r, kind)
	}
}

func kindNewPostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindNew(w, r, kind)
	}
}

func kindQuickEntryHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindQuickEntry(w, r, kind)
	}
}

func kindQuickGetHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.getKindQuick(w, r, kind)
	}
}

func kindQuickPostHandler(kind string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postKindQuick(w, r, kind)
	}
}
