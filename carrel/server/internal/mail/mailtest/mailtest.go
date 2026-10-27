// Package mailtest is an SMTP relay in memory for tests: it takes letters
// over a real socket and keeps them, and can be told to ask for a password
// or to refuse a recipient.
package mailtest

import (
	"bufio"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Letter is one letter the relay accepted.
type Letter struct {
	From string
	To   []string
	Data string
	Auth string // the user of AUTH PLAIN, if any
}

// Server is a running relay.
type Server struct {
	Host string
	Port int
	// Refuse makes RCPT of this address fail with RefuseReply, by default
	// «550 no such mailbox».
	Refuse      string
	RefuseReply string
	// Defer, when set, is the reply to every MAIL: the relay takes no
	// letter now («451 4.7.0 Try again later»).
	Defer string
	// NeedAuth makes MAIL fail with 530 until AUTH PLAIN with this user.
	NeedAuth string

	mu      sync.Mutex
	letters []Letter
	ln      net.Listener
}

// New starts a relay on 127.0.0.1 that stops with the test.
func New(t testing.TB) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{ln: ln, Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

// Letters are the letters taken so far.
func (s *Server) Letters() []Letter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Letter(nil), s.letters...)
}

func (s *Server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.session(c)
	}
}

func (s *Server) session(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(line string) { _, _ = c.Write([]byte(line + "\r\n")) }
	say("220 mailtest ready")
	var cur Letter
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch verb {
		case "EHLO", "HELO":
			_, _ = c.Write([]byte("250-mailtest\r\n250-AUTH PLAIN\r\n250 8BITMIME\r\n"))
		case "AUTH":
			parts := strings.Fields(line)
			if len(parts) == 3 {
				raw, _ := base64.StdEncoding.DecodeString(parts[2])
				fields := strings.Split(string(raw), "\x00")
				if len(fields) == 3 {
					cur.Auth = fields[1]
				}
			}
			say("235 ok")
		case "MAIL":
			if s.NeedAuth != "" && cur.Auth != s.NeedAuth {
				say("530 authentication required")
				continue
			}
			if s.Defer != "" {
				say(s.Defer)
				continue
			}
			cur.From = strings.Trim(strings.Fields(strings.TrimPrefix(line[5:], "FROM:"))[0], "<>")
			say("250 ok")
		case "RCPT":
			to := strings.Trim(strings.TrimPrefix(line[5:], "TO:"), "<> ")
			if s.Refuse != "" && to == s.Refuse {
				reply := s.RefuseReply
				if reply == "" {
					reply = "550 no such mailbox"
				}
				say(reply)
				continue
			}
			cur.To = append(cur.To, to)
			say("250 ok")
		case "DATA":
			say("354 go on")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			cur.Data = b.String()
			s.mu.Lock()
			s.letters = append(s.letters, cur)
			s.mu.Unlock()
			cur = Letter{Auth: cur.Auth}
			say("250 queued as " + strconv.Itoa(len(s.letters)))
		case "RSET", "NOOP":
			say("250 ok")
		case "QUIT":
			say("221 bye")
			return
		default:
			say("502 not implemented")
		}
	}
}
