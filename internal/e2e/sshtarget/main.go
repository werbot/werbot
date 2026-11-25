// Command sshtarget is the SSH target of the Playwright e2e suite and of the
// red-team stand: a password SSH server with an ephemeral host key. Exec
// "echo X" prints "X"; a shell echoes its input. The secret commands print the
// password the client logged in with: "cat /var/tmp/secret" in one write,
// "cat /var/tmp/secret.split" in two writes 50 ms apart cut in the middle of
// the value, "cat /var/tmp/secret.err" on stderr (masking, stage 16 scenario
// 4). It is not part of the werbot binary.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"flag"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:2022", "listen address")
	password := flag.String("password", "", "accepted password")
	flag.Parse()
	if *password == "" {
		log.Fatal("sshtarget: -password is required")
	}
	srv, err := newServer(*addr, *password)
	if err != nil {
		log.Fatalf("sshtarget: %v", err)
	}
	log.Fatal(srv.ListenAndServe())
}

// newServer is the server on addr that accepts password.
func newServer(addr, password string) (*ssh.Server, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	return &ssh.Server{
		Addr:        addr,
		Handler:     handle,
		HostSigners: []ssh.Signer{signer},
		PasswordHandler: func(ctx ssh.Context, p string) bool {
			ok := subtle.ConstantTimeCompare([]byte(p), []byte(password)) == 1
			if ok {
				ctx.SetValue(passwordKey{}, p) // read by the secret commands
			}
			return ok
		},
	}, nil
}

// passwordKey holds the login password in the session context.
type passwordKey struct{}

// splitPause is the pause between the two writes of the split secret.
const splitPause = 50 * time.Millisecond

func handle(s ssh.Session) {
	if s.RawCommand() == "" {
		_, _ = io.Copy(s, s)
		return
	}
	respond(s)
	_ = s.Exit(0)
}

// respond writes the output of the exec: a secret command prints the login
// password, any other command is echoed without a leading "echo ".
func respond(s ssh.Session) {
	pass, _ := s.Context().Value(passwordKey{}).(string)
	switch cmd := s.RawCommand(); cmd {
	case "cat /var/tmp/secret":
		_, _ = io.WriteString(s, pass+"\n")
	case "cat /var/tmp/secret.split":
		half := len(pass) / 2
		_, _ = io.WriteString(s, pass[:half])
		time.Sleep(splitPause)
		_, _ = io.WriteString(s, pass[half:]+"\n")
	case "cat /var/tmp/secret.err":
		_, _ = io.WriteString(s.Stderr(), pass+"\n")
	default:
		_, _ = io.WriteString(s, strings.TrimPrefix(cmd, "echo ")+"\n")
	}
}
