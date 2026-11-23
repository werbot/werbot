// Command telnettarget is the telnet target of the Playwright e2e suite: a
// login prompt, a password prompt without echo, then a shell that echoes every
// input line. Telnet options are refused. It is not part of the werbot binary.
package main

import (
	"bufio"
	"crypto/subtle"
	"flag"
	"log"
	"net"
	"strings"
)

const (
	iac  = 255
	dont = 254
	do   = 253
	wont = 252
	will = 251
	sb   = 250
	se   = 240
)

func main() {
	addr := flag.String("addr", "127.0.0.1:2023", "listen address")
	login := flag.String("login", "", "accepted login")
	password := flag.String("password", "", "accepted password")
	flag.Parse()
	if *login == "" || *password == "" {
		log.Fatal("telnettarget: -login and -password are required")
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("telnettarget: %v", err)
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatalf("telnettarget: %v", err)
		}
		go serve(c, *login, *password)
	}
}

func serve(c net.Conn, login, password string) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(s string) { _, _ = c.Write([]byte(s)) }
	say("login: ")
	name, err := readLine(r, c)
	if err != nil {
		return
	}
	say("Password: ")
	pass, err := readLine(r, c)
	if err != nil {
		return
	}
	if name != login || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
		say("\r\nLogin incorrect\r\n")
		return
	}
	say("\r\nWelcome\r\n$ ")
	for {
		line, err := readLine(r, c)
		if err != nil {
			return
		}
		say(line + "\r\n$ ")
	}
}

// readLine returns the next line ended by CR or LF; it answers option
// negotiation with a refusal and skips subnegotiation.
func readLine(r *bufio.Reader, c net.Conn) (string, error) {
	var line strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		switch b {
		case iac:
			if err := command(r, c); err != nil {
				return "", err
			}
		case '\r', '\n':
			if line.Len() > 0 {
				return line.String(), nil
			}
		case 0:
		default:
			line.WriteByte(b)
		}
	}
}

func command(r *bufio.Reader, c net.Conn) error {
	cmd, err := r.ReadByte()
	if err != nil {
		return err
	}
	switch cmd {
	case do, will:
		opt, err := r.ReadByte()
		if err != nil {
			return err
		}
		reply := byte(wont)
		if cmd == will {
			reply = dont
		}
		_, err = c.Write([]byte{iac, reply, opt})
		return err
	case sb:
		prev := byte(0)
		for {
			b, err := r.ReadByte()
			if err != nil {
				return err
			}
			if prev == iac && b == se {
				return nil
			}
			prev = b
		}
	}
	return nil
}
