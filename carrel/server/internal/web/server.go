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
	"carrel.dev/carrel/server/render"
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
	// SetupFirstAdminWith is the same with an address (s-auth-4: optional).
	SetupFirstAdminWith(ctx context.Context, login, password, email string) error
	Authenticate(ctx context.Context, login, password string) (accountID string, dataKey [32]byte, mustChange bool, ok bool)
	LoginExists(login string) bool
	RegisterOpen() bool

	InviteValid(ctx context.Context, token string) (valid bool)
	// InviteLogin is the login a live invitation names ("" — the person
	// chooses it) and whether the token is one.
	InviteFor(ctx context.Context, token string) (inv user.OpenInvite, valid bool)
	CompleteInvite(ctx context.Context, token, password string) error
	AcceptInvite(ctx context.Context, token, login, password string) error

	ConfirmEmail(ctx context.Context, token string) (ok bool)
	// Register and ForgotRequest return the raw link token for the mail
	// layer; the request layer never puts it in a response.
	Register(ctx context.Context, login, email, password string) (token string, err error)

	ForgotLookup(loginOrEmail string) (login string, hasMail, canAsk bool)
	ForgotRequest(ctx context.Context, loginOrEmail string) (token string, err error)
	ForgotAsk(ctx context.Context, loginOrEmail string) (asked bool, err error)
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
	UpdateConnection(viewerID string, dataKey [32]byte, conn sealed.Connection) error
	RemoveConnection(viewerID string, dataKey [32]byte, accountID string) (user.Disconnected, error)
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
	// SetAttachmentNames stores how large attachments are named (S-20):
	// one of sealed.AttachNames….
	SetAttachmentNames(viewerID string, dataKey [32]byte, names string) error
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
	Publish(viewerID string, dataKey [32]byte, conn sealed.Connection, target user.PublishTarget, expires *time.Time) (string, error)
	LinkOf(viewerID string, dataKey [32]byte, id string) string
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
	// Invite, ResendInvite and CreateUser are the ways of s-admin-1: an
	// invitation with its role and no login, a new secret for one that went
	// by letter, the temporary password. SuggestPassword is what a form
	// offers for a temporary password; nothing keeps it.
	Invite(actorID, email string, role store.Role) (secret string, err error)
	ResendInvite(actorID, inviteID string) (secret, email string, err error)
	CreateUser(ctx context.Context, actorID, login, email, password string, role store.Role) error
	SuggestPassword() string
	RevokeInvite(actorID, inviteID string) error
	ExtendInvite(actorID, inviteID string) error
	DisableUser(actorID, targetID string) error
	EnableUser(actorID, targetID string) error
	DeleteUser(actorID, targetID, loginConfirm string) error
	SetUserRole(actorID, targetID string, role store.Role) error
	EndUserSessions(actorID, targetID string) error
	AdminResetPassword(ctx context.Context, actorID, targetID, temp string) error
	SetSelfRegEnabled(actorID string, enabled bool) error

	// S-26: the relay and the letters' bookkeeping.
	SMTP() store.SMTPSettings
	SetSMTP(actorID string, in store.SMTPSettings) error
	MarkInviteMailed(secret string, sent bool, reply string) error
	ConfirmedAddress(loginOrEmail string) string
	SetDisplayName(userID, name string) error
	RequestEmail(userID, email string) (token string, err error)
	EmailToConfirm(userID string) string
	// ConfirmMailed records that the letter with a confirmation link went;
	// ConfirmationSent is that letter while its link lives (account-9).
	ConfirmMailed(token string) error
	ConfirmationSent(userID string) (to string, at time.Time, ok bool)
	LettersLeft(userID string, perDay int) int
	Journal(actorID, what, detail string)
	IssueDevice(ctx context.Context, userID string, dataKey [32]byte, name string, sc user.DeviceScope) (id, password string, err error)
	RevokeDevice(userID, id string) error
	SetDeviceScope(userID string, dataKey [32]byte, id string, sc user.DeviceScope) error
	SetDeviceAccess(userID, id string, readOnly bool) error
	Devices(userID string) []store.ProxyDevice
	DeviceLastSeen(id string) time.Time
	SetUploadFolder(viewerID string, dataKey [32]byte, folder sealed.AttachmentFolder) error
	Settings() store.InstanceSettings
	SetInstance(actorID string, in user.InstanceUpdate) error
	InviteTTL() time.Duration
	ReserveLetter(userID string, perDay int) error
	ReleaseLetter(userID string)
	LetterSent(userID string, to []string, attached string)

	EnableEscrow(ctx context.Context, actorID, master, master2 string) error
	SetEscrowCoverNew(actorID string, cover bool) error
	ChangeEscrowMaster(ctx context.Context, actorID, current, newPW, newPW2 string) error
	DepositEscrow(accountID string, dataKey [32]byte) error
	WithdrawEscrow(accountID string) error
	RecoverEscrow(ctx context.Context, actorID, login, masterPassword, temp string) (tempPassword string, err error)
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
	render   render.Renderer
	limit    *Limiter
	guard    *dav.Guard // every outbound DAV request of this server (S-5.1)
	probes   probes     // self-requests of a running installation check (S-28.1)
	version  string
	commit   string
	mux      *http.ServeMux
	proxies  []netip.Prefix // CARREL_TRUSTED_PROXIES
	// clock is what the screens call "now": «2 min ago», the day a week
	// starts from, the stamp under a printed page. Only the answer's own
	// sense of now goes through it — a record's dates are the record's.
	// Tests freeze it (SetClock) so a fixture is the same tomorrow.
	clock func() time.Time
}

// now is the instant this answer calls "now".
func (s *Server) now() time.Time {
	if s.clock == nil {
		return time.Now()
	}
	return s.clock()
}

// SetBuild names the version and commit the answers carry (the rail's
// «Carrel 0.0.1 · 786949a»). Nothing but a test calls it: an instance is
// built with them.
func (s *Server) SetBuild(version, commit string) {
	s.version, s.commit = version, commit
}

// SetPatience lifts the rate buckets for a test that does in a second what
// a person does in an hour. Nothing but a test calls it: the limits of
// architecture.md are what an instance runs with, and a fixture run is not
// the traffic they guard against.
func (s *Server) SetPatience() {
	s.limit = &Limiter{
		loginAccount: newBucketMap(1 << 20),
		loginIP:      newBucketMap(1 << 20),
		anonIP:       newBucketMap(1 << 20),
		session:      newBucketMap(1 << 20),
		link:         newBucketMap(1 << 20),
	}
}

// DAVCounts reports what this server's outbound guard has done: connections
// dialled, requests sent, of those the ones that write, and of those the
// ones that went out on a connection the pool had kept. It exists to settle
// a question about the pool with numbers (docs/plan/server.md, «Пул
// соединений»); nothing but a measurement reads it.
func (s *Server) DAVCounts() (clients, dialled, requests, writes, writesFromIdle, resent int64) {
	return s.guard.Counts()
}

// SetClock freezes what the screens call "now". Nothing but a test calls
// it: an instance reads the wall clock.
func (s *Server) SetClock(at time.Time) {
	s.clock = func() time.Time { return at }
}

// New builds a Server with all C-5 routes registered.
func New(cfg *conf.Config, st *store.Store, mgr *session.Manager, users Users, rd render.Renderer, version, commit string) *Server {
	if rd == nil {
		rd = render.JSONRenderer{}
	}
	s := &Server{
		cfg:      cfg,
		basePath: conf.NormalizeBasePath(cfg.BasePath),
		store:    st,
		sessions: mgr,
		users:    users,
		render:   rd,
		limit:    NewLimiter(),
		guard:    dav.NewGuard(cfg),
		version:  version,
		commit:   commit,
		proxies:  conf.ParseTrustedProxies(cfg.TrustedProxies),
	}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler: the security headers of every
// answer around the mux, so the mux's own 404 carries them too.
func (s *Server) Handler() http.Handler {
	if s.basePath == "" {
		return s.observe(secureHeaders(s.mux))
	}
	return s.observe(http.StripPrefix(s.basePath, secureHeaders(s.mux)))
}

// Static mounts files served from the binary (C-5.9) at prefix of the mux,
// which sees paths without the base path: the client's library files, which
// cmd/carrel alone can name. GET and HEAD only; the security headers of every
// answer are around it as around any route, and the handler sets its own
// caching over their no-store. It stands outside the routes' buckets and
// CSRF (serve): the same files for everyone, read from memory, and a page
// loads several of them at once.
func (s *Server) Static(prefix string, h http.Handler) {
	s.mux.Handle("GET "+strings.TrimSuffix(prefix, "/")+"/", h)
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

func (s *Server) trustedAddr(a netip.Addr) bool {
	return conf.TrustedAddr(s.proxies, a)
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
