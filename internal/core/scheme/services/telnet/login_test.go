package telnet

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func readLine(t *testing.T, c net.Conn) string {
	t.Helper()
	var line []byte
	buf := make([]byte, 1)
	for {
		if _, err := io.ReadFull(c, buf); err != nil {
			t.Fatalf("read line: %v", err)
		}
		line = append(line, buf[0])
		if bytes.HasSuffix(line, []byte("\r\n")) || bytes.HasSuffix(line, []byte("\n")) {
			return string(line)
		}
	}
}

func loginPipe(t *testing.T) (*Conn, net.Conn) {
	t.Helper()
	local, peer := pipePair(t)
	return NewClient(local, "xterm"), peer
}

func TestLoginSuccess(t *testing.T) {
	t.Parallel()

	c, target := loginPipe(t)
	go func() {
		writeAll(t, target, []byte("Ubuntu\r\nlogin: "))
		if got := readLine(t, target); got != "ubuntu1\r\n" {
			t.Errorf("login = %q, want %q", got, "ubuntu1\r\n")
			return
		}
		writeAll(t, target, []byte("Password: "))
		if got := readLine(t, target); got != "secret12\r\n" {
			t.Errorf("password = %q, want %q", got, "secret12\r\n")
			return
		}
		writeAll(t, target, []byte("Welcome\r\n$ "))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	out, err := Login(ctx, c, "ubuntu1", "secret12")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !bytes.Contains(out, []byte("Welcome")) {
		t.Fatalf("output = %q, want Welcome", out)
	}
	if !bytes.HasSuffix(out, []byte("$ ")) {
		t.Fatalf("output = %q, want shell prompt suffix", out)
	}
}

func TestLoginUsernamePrompt(t *testing.T) {
	t.Parallel()

	c, target := loginPipe(t)
	go func() {
		writeAll(t, target, []byte("Username:"))
		_ = readLine(t, target)
		writeAll(t, target, []byte("Password: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("$ "))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	if _, err := Login(ctx, c, "ubuntu1", "secret12"); err != nil {
		t.Fatalf("Login: %v", err)
	}
}

func TestLoginQuietSuccess(t *testing.T) {
	t.Parallel()

	c, target := loginPipe(t)
	go func() {
		writeAll(t, target, []byte("login: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("Password: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("motd\r\n"))
		time.Sleep(2 * time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	out, err := Login(ctx, c, "ubuntu1", "secret12")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if string(out) != "motd\r\n" {
		t.Fatalf("output = %q, want %q", out, "motd\r\n")
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("Login took %v, want about 1s", elapsed)
	}
}

func TestLoginIncorrect(t *testing.T) {
	t.Parallel()

	c, target := loginPipe(t)
	go func() {
		writeAll(t, target, []byte("login: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("Password: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("Login incorrect\r\nlogin: "))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	if _, err := Login(ctx, c, "ubuntu1", "wrongpass"); !errors.Is(err, ErrLoginFailed) {
		t.Fatalf("Login error = %v, want ErrLoginFailed", err)
	}
}

func TestLoginPromptAgain(t *testing.T) {
	t.Parallel()

	c, target := loginPipe(t)
	go func() {
		writeAll(t, target, []byte("login: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("Password: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("login: "))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	if _, err := Login(ctx, c, "ubuntu1", "secret12"); !errors.Is(err, ErrLoginFailed) {
		t.Fatalf("Login error = %v, want ErrLoginFailed", err)
	}
}

func TestLoginTimeout(t *testing.T) {
	t.Parallel()

	c, _ := loginPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if _, err := Login(ctx, c, "ubuntu1", "secret12"); !errors.Is(err, ErrLoginTimeout) {
		t.Fatalf("Login error = %v, want ErrLoginTimeout", err)
	}
}

func TestLoginOutputExcludesPassword(t *testing.T) {
	t.Parallel()

	c, target := loginPipe(t)
	go func() {
		writeAll(t, target, []byte("login: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("Password: "))
		_ = readLine(t, target)
		writeAll(t, target, []byte("secret12\r\n$ "))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	out, err := Login(ctx, c, "ubuntu1", "secret12")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if bytes.Contains(out, []byte("secret12")) {
		t.Fatalf("output leaked password: %q", out)
	}
	if !strings.HasSuffix(string(out), "$ ") {
		t.Fatalf("output = %q, want shell prompt suffix", out)
	}
}
