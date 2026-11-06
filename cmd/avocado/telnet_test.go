package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	telnetpkg "github.com/werbot/werbot/internal/core/scheme/services/telnet"
)

func testHostKey(t *testing.T) gossh.Signer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer
}

type sshPipe struct {
	client     gossh.Channel
	server     gossh.Channel
	serverReqs <-chan *gossh.Request
	clientConn *gossh.Client
	serverConn *gossh.ServerConn
}

func newSSHPipe(t *testing.T) *sshPipe {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	signer := testHostKey(t)

	type serverResult struct {
		conn *gossh.ServerConn
		ch   gossh.Channel
		reqs <-chan *gossh.Request
		err  error
	}
	serverReady := make(chan serverResult, 1)

	go func() {
		raw, err := listener.Accept()
		if err != nil {
			serverReady <- serverResult{err: err}
			return
		}
		_ = raw.SetDeadline(time.Now().Add(5 * time.Second))

		config := &gossh.ServerConfig{NoClientAuth: true}
		config.AddHostKey(signer)
		conn, chans, _, err := gossh.NewServerConn(raw, config)
		if err != nil {
			serverReady <- serverResult{err: err}
			return
		}
		newChan, ok := <-chans
		if !ok {
			serverReady <- serverResult{err: io.EOF}
			return
		}
		ch, reqs, err := newChan.Accept()
		serverReady <- serverResult{conn: conn, ch: ch, reqs: reqs, err: err}
	}()

	clientNet, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = clientNet.SetDeadline(time.Now().Add(5 * time.Second))

	clientConn, chans, reqs, err := gossh.NewClientConn(clientNet, "test", &gossh.ClientConfig{
		User:            "test",
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test-only loopback SSH server
	})
	if err != nil {
		t.Fatalf("NewClientConn: %v", err)
	}
	client := gossh.NewClient(clientConn, chans, reqs)
	clientCh, _, err := client.OpenChannel("session", nil)
	if err != nil {
		t.Fatalf("OpenChannel: %v", err)
	}

	server := <-serverReady
	if server.err != nil {
		t.Fatalf("server: %v", server.err)
	}

	pipe := &sshPipe{
		client:     clientCh,
		server:     server.ch,
		serverReqs: server.reqs,
		clientConn: client,
		serverConn: server.conn,
	}
	t.Cleanup(func() {
		_ = clientCh.Close()
		_ = server.ch.Close()
		_ = client.Close()
		_ = server.conn.Close()
	})
	return pipe
}

func ptyRequestPayload(term string, width, height int) []byte {
	return gossh.Marshal(struct {
		Term              string
		Width, Height     uint32
		WidthPx, HeightPx uint32
	}{term, uint32(width), uint32(height), 0, 0}) //nolint:gosec // test payload uses small positive dimensions
}

func windowChangePayload(width, height int) []byte {
	return gossh.Marshal(struct {
		Width, Height, WidthPx, HeightPx uint32
	}{uint32(width), uint32(height), 0, 0}) //nolint:gosec // test payload uses small positive dimensions
}

func startShell(t *testing.T, pipe *sshPipe) ptyInfo {
	t.Helper()

	type result struct {
		info ptyInfo
		ok   bool
	}
	done := make(chan result, 1)
	go func() {
		info, ok := waitShell(pipe.serverReqs)
		done <- result{info: info, ok: ok}
	}()

	for _, request := range []struct {
		name    string
		payload []byte
	}{
		{"pty-req", ptyRequestPayload("xterm", 100, 30)},
		{"env", gossh.Marshal(struct{ Name, Value string }{"LANG", "C"})},
		{"shell", nil},
	} {
		ok, err := pipe.client.SendRequest(request.name, true, request.payload)
		if err != nil {
			t.Fatalf("SendRequest(%s): %v", request.name, err)
		}
		if !ok {
			t.Fatalf("SendRequest(%s) rejected", request.name)
		}
	}

	res := <-done
	if !res.ok {
		t.Fatal("waitShell() = false, want true")
	}
	return res.info
}

func TestWaitShell(t *testing.T) {
	pipe := newSSHPipe(t)
	info := startShell(t, pipe)

	if info != (ptyInfo{Width: 100, Height: 30, Term: "xterm"}) {
		t.Fatalf("ptyInfo = %+v", info)
	}
}

func TestWaitShellRejectsExec(t *testing.T) {
	pipe := newSSHPipe(t)

	done := make(chan bool, 1)
	go func() {
		_, ok := waitShell(pipe.serverReqs)
		done <- ok
	}()

	ok, err := pipe.client.SendRequest("exec", true, gossh.Marshal(struct{ Command string }{"ls"}))
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if ok {
		t.Fatal("exec request was accepted")
	}
	if <-done {
		t.Fatal("waitShell() = true, want false")
	}
}

func TestPipeTelnet(t *testing.T) {
	pipe := newSSHPipe(t)
	startShell(t, pipe)

	targetLocal, targetPeer := net.Pipe()
	t.Cleanup(func() {
		_ = targetLocal.Close()
		_ = targetPeer.Close()
	})
	target := telnetpkg.NewClient(targetLocal, "xterm")

	var recorded bytes.Buffer
	out := io.MultiWriter(pipe.server, &recorded)

	pipeDone := make(chan error, 1)
	go func() {
		pipeDone <- pipeTelnet(pipe.server, pipe.serverReqs, target, out)
	}()

	if _, err := pipe.client.Write([]byte("ls\n")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	got := make([]byte, 3)
	if _, err := io.ReadFull(targetPeer, got); err != nil {
		t.Fatalf("target read: %v", err)
	}
	if string(got) != "ls\n" {
		t.Fatalf("target received %q, want %q", got, "ls\n")
	}

	if _, err := targetPeer.Write([]byte("file\r\n")); err != nil {
		t.Fatalf("target write: %v", err)
	}
	clientGot := make([]byte, len("file\r\n"))
	if _, err := io.ReadFull(pipe.client, clientGot); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(clientGot) != "file\r\n" {
		t.Fatalf("client received %q, want %q", clientGot, "file\r\n")
	}

	if _, err := targetPeer.Write([]byte{telnetpkg.IAC, telnetpkg.DO, 31}); err != nil {
		t.Fatalf("target write NAWS: %v", err)
	}
	wantWill := []byte{telnetpkg.IAC, telnetpkg.WILL, 31}
	if got := make([]byte, len(wantWill)); true {
		if _, err := io.ReadFull(targetPeer, got); err != nil {
			t.Fatalf("target read WILL NAWS: %v", err)
		}
		if !bytes.Equal(got, wantWill) {
			t.Fatalf("target received %v, want %v", got, wantWill)
		}
	}

	requestDone := make(chan bool, 1)
	go func() {
		ok, _ := pipe.client.SendRequest("window-change", true, windowChangePayload(132, 50))
		requestDone <- ok
	}()
	wantNAWS := []byte{telnetpkg.IAC, telnetpkg.SB, 31, 0, 132, 0, 50, telnetpkg.IAC, telnetpkg.SE}
	gotNAWS := make([]byte, len(wantNAWS))
	if _, err := io.ReadFull(targetPeer, gotNAWS); err != nil {
		t.Fatalf("target read NAWS: %v", err)
	}
	if !bytes.Equal(gotNAWS, wantNAWS) {
		t.Fatalf("target received %v, want %v", gotNAWS, wantNAWS)
	}
	if !<-requestDone {
		t.Fatal("window-change request was rejected")
	}

	execDone := make(chan bool, 1)
	go func() {
		ok, _ := pipe.client.SendRequest("exec", true, gossh.Marshal(struct{ Command string }{"id"}))
		execDone <- ok
	}()
	if <-execDone {
		t.Fatal("exec request was accepted after shell")
	}

	_ = targetPeer.Close()
	select {
	case err := <-pipeDone:
		if err != nil {
			t.Fatalf("pipeTelnet() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pipeTelnet did not return after target close")
	}
	if !bytes.Contains(recorded.Bytes(), []byte("file\r\n")) {
		t.Fatalf("recorded output = %q, want file data", recorded.String())
	}
}

func TestSetInitialWindowSize(t *testing.T) {
	local, peer := net.Pipe()
	t.Cleanup(func() {
		_ = local.Close()
		_ = peer.Close()
	})
	target := telnetpkg.NewClient(local, "xterm")

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 64)
		for {
			if _, err := target.Read(buf); err != nil {
				return
			}
		}
	}()

	if _, err := peer.Write([]byte{telnetpkg.IAC, telnetpkg.DO, 31}); err != nil {
		t.Fatalf("write DO NAWS: %v", err)
	}
	wantWill := []byte{telnetpkg.IAC, telnetpkg.WILL, 31}
	if got := readExactBytes(t, peer, len(wantWill)); !bytes.Equal(got, wantWill) {
		t.Fatalf("reply = %v, want %v", got, wantWill)
	}

	setDone := make(chan struct{})
	go func() {
		setInitialWindowSize(target, 100, 30)
		close(setDone)
	}()

	wantNAWS := []byte{telnetpkg.IAC, telnetpkg.SB, 31, 0, 100, 0, 30, telnetpkg.IAC, telnetpkg.SE}
	if got := readExactBytes(t, peer, len(wantNAWS)); !bytes.Equal(got, wantNAWS) {
		t.Fatalf("NAWS = %v, want %v", got, wantNAWS)
	}
	<-setDone

	_ = peer.Close()
	<-readDone
}
