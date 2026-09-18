package web

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

const (
	cookieName     = "carrel_sid"
	csrfFormField  = "_csrf"
	csrfHeaderName = "X-CSRF-Token"
)

// Users handles accounts and secrets (S-10). Implemented by internal/user.
type Users interface {
	FirstRun() bool
	SetupFirstAdmin(ctx context.Context, login, password string) error
	Authenticate(ctx context.Context, login, password string) (accountID string, dataKey [32]byte, mustChange bool, ok bool)
	LoginExists(login string) bool
	RegisterOpen() bool

	InviteValid(ctx context.Context, token string) (valid bool)
	CompleteInvite(ctx context.Context, token, password string) error

	ConfirmEmail(ctx context.Context, token string) (ok bool)
	// Register and ForgotRequest return the raw link token for the mail
	// layer; the request layer never puts it in a response.
	Register(ctx context.Context, login, email, password string) (token string, err error)

	ForgotRequest(ctx context.Context, loginOrEmail string) (token string, sent, asked, hasMail bool, err error)
	ForgotWipeState(ctx context.Context, token string) (login string, step string, valid bool)
	ForgotConfirmWipe(ctx context.Context, token string) error
	ForgotSetPassword(ctx context.Context, token, password string) error

	IsAdmin(accountID string) bool
	AccountRole(accountID string) store.Role
	// OwnsAccount opens the viewer's sealed blob with the session's data key
	// and looks for the connection: {account} is a connection (C-4), never
	// the user ID.
	OwnsAccount(viewerID string, dataKey [32]byte, accountParam string) bool
	// OwnsCollection looks {col} up in the catalog of the sealed blob: the
	// collection address, decoded from its one segment (C-4.1).
	OwnsCollection(viewerID string, dataKey [32]byte, accountParam, col string) bool
	OwnsRecord(viewerID string, dataKey [32]byte, accountParam, col, uid string) bool

	// Catalog opens the viewer's sealed blob; AddConnection appends a
	// discovered connection with its collections and returns its ID.
	Catalog(viewerID string, dataKey [32]byte) (sealed.Blob, bool)
	AddConnection(viewerID string, dataKey [32]byte, conn sealed.Connection, cols []sealed.Collection) (string, error)
	// ApplyRediscovery writes a re-discovery the person confirmed (S-6.1):
	// only what they were shown and the server still confirms.
	ApplyRediscovery(viewerID string, dataKey [32]byte, accountID string, rd user.Rediscovery) error
	// RevealConnectionPassword returns one stored password and journals the
	// release; the caller checks the unlock window (S-10.7).
	RevealConnectionPassword(viewerID string, dataKey [32]byte, accountID string) (string, error)
	// AddCollection writes a collection the server has just created into the
	// catalog; RenameCollection writes the name and colour it accepted (S-12).
	AddCollection(viewerID string, dataKey [32]byte, accountID string, col sealed.Collection) error
	RenameCollection(viewerID string, dataKey [32]byte, accountID, path, name, colour string) error
	PlaceMovedCollection(viewerID string, dataKey [32]byte, fromAccount, fromPath, toAccount string, col sealed.Collection, move bool) error
	// RemoveCollection takes a deleted collection and its ticks out (S-12.4).
	RemoveCollection(viewerID string, dataKey [32]byte, accountID, path string) error
	// SetSelection replaces the ticks of one kind (S-14); other kinds keep
	// theirs.
	SetSelection(viewerID string, dataKey [32]byte, kind view.Kind, picks []sealed.Selection) error
	// SetAttachmentFolder stores the folder large attachments go to (S-20).
	SetAttachmentFolder(viewerID string, dataKey [32]byte, folder sealed.AttachmentFolder) error
	// SetLastUsed remembers where a record of a kind was last created, so
	// the sheet opened from the shell starts there (U-6.16). No journal entry.
	SetLastUsed(viewerID string, dataKey [32]byte, sel sealed.Selection) error
	// EditDuplicates changes the decisions about duplicates (S-16); an
	// empty what journals nothing.
	EditDuplicates(viewerID string, dataKey [32]byte, what, detail string, edit func(*sealed.Duplicates) error) error
	// A device profile (S-23): the one-time link, followed once by the same
	// person; the connections left out of every device set-up.
	IssueDeviceProfile(viewerID string, accounts []string) (string, time.Time, error)
	TakeDeviceProfile(viewerID, token string) ([]string, bool)
	RevokeDeviceProfiles(viewerID string) error
	SetDeviceExcluded(viewerID string, dataKey [32]byte, accountID string, excluded bool) error
	// Published links (S-25): made with the one connection's credentials in
	// a sealed envelope, opened without a session, listed and revoked by
	// their owner.
	Publish(viewerID string, conn sealed.Connection, target user.PublishTarget, expires *time.Time) (string, error)
	OpenPublished(token string) (user.OpenLink, bool)
	PublishedLinks(viewerID string) []user.PublishedLink
	RevokePublished(viewerID, id string) error
	// Subscriptions (S-25.2): feeds in the blob, renamed only here.
	Subscribe(viewerID string, dataKey [32]byte, f sealed.Subscription) (string, error)
	Unsubscribe(viewerID string, dataKey [32]byte, id string) error
	EditFeed(viewerID string, dataKey [32]byte, id, name, colour string, kinds []view.Kind) error
	// Backup (S-24): jobs in the blob, runs in the state.
	SaveBackupJob(viewerID string, dataKey [32]byte, job sealed.BackupJob) (string, error)
	DeleteBackupJob(viewerID string, dataKey [32]byte, id string) error
	BackupRuns(viewerID string) map[string][]store.BackupRun
	StartBackupRun(viewerID, jobID, trigger, mode string, running func(runID string) bool) (string, error)
	FinishBackupRun(viewerID, jobID, runID string, finish func(r *store.BackupRun)) error
	// The external trigger (S-24.4): a secret per job and an envelope of the
	// data key under server.key.
	EnableBackupTrigger(viewerID string, dataKey [32]byte, jobID string) (string, error)
	DisableBackupTrigger(viewerID, jobID string) error
	BackupTriggerOn(viewerID, jobID string) bool
	OpenBackupTrigger(token string) (userID, jobID string, dataKey [32]byte, ok bool)

	// SetTimePrefs stores the account's first day of the week and 12-hour
	// clock (C-10).
	SetTimePrefs(accountID, weekStart string, clock12 bool) error

	PublicLinkValid(ctx context.Context, token string) bool

	// Unlock confirms the password of the session's own account for the
	// S-10.7 sheet and journals it.
	Unlock(ctx context.Context, accountID, password string) bool

	// ChangePassword re-wraps the data key under a new password and ends
	// every other session of the account.
	ChangePassword(ctx context.Context, accountID, exceptSID, current, newPW, newPW2 string) error

	CreateInvite(actorID, login, email string) (secret string, err error)
	RevokeInvite(actorID, inviteID string) error
	ExtendInvite(actorID, inviteID string) error
	DisableUser(actorID, targetID string) error
	DeleteUser(actorID, targetID, loginConfirm string) error
	SetUserRole(actorID, targetID string, role store.Role) error
	EndUserSessions(actorID, targetID string) error
	AdminResetPassword(ctx context.Context, actorID, targetID string) (token string, mailed, escrowAsk, handLink bool, err error)
	SetSelfRegEnabled(actorID string, enabled bool) error

	EnableEscrow(ctx context.Context, actorID, master, master2 string) error
	SetEscrowCoverNew(actorID string, cover bool) error
	ChangeEscrowMaster(ctx context.Context, actorID, current, newPW, newPW2 string) error
	DepositEscrow(accountID string, dataKey [32]byte) error
	WithdrawEscrow(accountID string) error
	RecoverEscrow(ctx context.Context, actorID, login, masterPassword string) (tempPassword string, err error)
	ConsumeEscrowNotice(accountID string) bool
	EscrowEnabled() bool
}

// Server is the HTTP front door (S-9).
type Server struct {
	cfg      *conf.Config
	basePath string // normalised CARREL_BASE_PATH
	store    *store.Store
	sessions *session.Manager
	users    Users
	render   Renderer
	limit    *Limiter
	guard    *dav.Guard // every outbound DAV request of this server (S-5.1)
	version  string
	commit   string
	mux      *http.ServeMux
	proxies  []netip.Prefix // CARREL_TRUSTED_PROXIES
}

// New builds a Server with all C-5 routes registered.
func New(cfg *conf.Config, st *store.Store, mgr *session.Manager, users Users, render Renderer, version, commit string) *Server {
	if render == nil {
		render = JSONRenderer{}
	}
	s := &Server{
		cfg:      cfg,
		basePath: conf.NormalizeBasePath(cfg.BasePath),
		store:    st,
		sessions: mgr,
		users:    users,
		render:   render,
		limit:    NewLimiter(),
		guard:    dav.NewGuard(cfg),
		version:  version,
		commit:   commit,
		proxies:  parseProxies(cfg.TrustedProxies),
	}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	if s.basePath == "" {
		return s.mux
	}
	return http.StripPrefix(s.basePath, s.mux)
}

func (s *Server) sessionFrom(r *http.Request) (*session.Session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, false
	}
	return s.sessions.Get(c.Value)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sid,
		Path:     s.cookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(r),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     s.cookiePath(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(r),
	})
}

func (s *Server) secureCookies(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// A trusted proxy may terminate TLS (S-28.1): its protocol header counts,
	// and so does an https CARREL_PUBLIC_URL.
	if s.fromTrustedProxy(r) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(s.cfg.PublicURL), "https://")
}

// parseProxies reads CARREL_TRUSTED_PROXIES: comma- or space-separated IPs
// and CIDRs. Anything unparsable is ignored rather than trusted.
func parseProxies(list string) []netip.Prefix {
	var out []netip.Prefix
	for _, item := range strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if p, err := netip.ParsePrefix(item); err == nil {
			out = append(out, p.Masked())
			continue
		}
		if a, err := netip.ParseAddr(item); err == nil {
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
		}
	}
	return out
}

func (s *Server) trustedAddr(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range s.proxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func (s *Server) fromTrustedProxy(r *http.Request) bool {
	a, ok := remoteAddr(r)
	return ok && s.trustedAddr(a)
}

func remoteAddr(r *http.Request) (netip.Addr, bool) {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

// clientIP is the address the rate buckets key on. Directly connected, it is
// the peer. Behind a proxy listed in CARREL_TRUSTED_PROXIES it is the
// rightmost X-Forwarded-For entry that is not itself a trusted proxy.
// Otherwise every client would share the proxy's bucket, and twenty login
// attempts from one attacker would lock the whole instance out (S-28.1: the
// service believes address headers only from trusted proxies).
func (s *Server) clientIP(r *http.Request) string {
	peer, ok := remoteAddr(r)
	if !ok {
		return r.RemoteAddr
	}
	if !s.trustedAddr(peer) {
		return peer.String()
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		a = a.Unmap()
		if !s.trustedAddr(a) {
			return a.String()
		}
	}
	return peer.String()
}

func checkCSRF(sess *session.Session, r *http.Request) bool {
	if sess == nil {
		return false
	}
	token := r.Header.Get(csrfHeaderName)
	if token == "" {
		token = r.FormValue(csrfFormField)
	}
	return sess.CheckCSRF(token)
}

func normalizeLogin(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func constantTimeDelay(start time.Time, min time.Duration) {
	if d := min - time.Since(start); d > 0 {
		time.Sleep(d)
	}
}

func tokensEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
