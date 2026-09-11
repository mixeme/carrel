package web

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
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
	Register(ctx context.Context, login, email, password string) error

	ForgotRequest(ctx context.Context, loginOrEmail string) (sent, asked, hasMail bool, err error)
	ForgotWipeState(ctx context.Context, token string) (login string, step string, valid bool)
	ForgotConfirmWipe(ctx context.Context, token string) error
	ForgotSetPassword(ctx context.Context, token, password string) error

	IsAdmin(accountID string) bool
	AccountRole(accountID string) store.Role
	OwnsAccount(viewerID, accountParam string) bool
	OwnsCollection(viewerID, accountParam, col string) bool
	OwnsRecord(viewerID, accountParam, col, uid string) bool

	PublicLinkValid(ctx context.Context, token string) bool

	UnlockUntil(accountID string) *time.Time
	SetUnlockUntil(accountID string, until *time.Time)
}

// Server is the HTTP front door (S-9).
type Server struct {
	cfg      *conf.Config
	store    *store.Store
	sessions *session.Manager
	users    Users
	render   Renderer
	limit    *Limiter
	version  string
	commit   string
	mux      *http.ServeMux
}

// New builds a Server with all C-5 routes registered.
func New(cfg *conf.Config, st *store.Store, mgr *session.Manager, users Users, render Renderer, version, commit string) *Server {
	if render == nil {
		render = JSONRenderer{}
	}
	s := &Server{
		cfg:      cfg,
		store:    st,
		sessions: mgr,
		users:    users,
		render:   render,
		limit:    NewLimiter(),
		version:  version,
		commit:   commit,
	}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
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
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(r),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
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
	// Trusted proxy may terminate TLS; CARREL_PUBLIC_URL https implies Secure cookies.
	return strings.HasPrefix(strings.ToLower(s.cfg.PublicURL), "https://")
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
