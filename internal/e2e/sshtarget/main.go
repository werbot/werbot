// Command sshtarget is the SSH target of the Playwright e2e suite: a password
// SSH server with an ephemeral host key. Exec "echo X" prints "X"; a shell
// echoes its input. It is not part of the werbot binary.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"flag"
	"io"
	"log"
	"strings"

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

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("sshtarget: host key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		log.Fatalf("sshtarget: host signer: %v", err)
	}
	srv := &ssh.Server{
		Addr:        *addr,
		Handler:     handle,
		HostSigners: []ssh.Signer{signer},
		PasswordHandler: func(_ ssh.Context, p string) bool {
			return subtle.ConstantTimeCompare([]byte(p), []byte(*password)) == 1
		},
	}
	log.Fatal(srv.ListenAndServe())
}

func handle(s ssh.Session) {
	if s.RawCommand() == "" {
		_, _ = io.Copy(s, s)
		return
	}
	_, _ = io.WriteString(s, strings.TrimPrefix(s.RawCommand(), "echo ")+"\n")
	_ = s.Exit(0)
}
