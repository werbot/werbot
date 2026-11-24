// Command echotarget is the TCP and UDP echo target of the network e2e
// (deploy/e2e): it sends every TCP stream and UDP datagram on port 7 back
// unchanged and answers every HTTP request on port 80 with "werbot-probe".
// It is not part of the werbot binary.
package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", ":7") //nolint:gosec // the e2e echo target serves every interface of its container
	if err != nil {
		log.Fatalf("echotarget: %v", err)
	}
	pc, err := net.ListenPacket("udp", ":7")
	if err != nil {
		log.Fatalf("echotarget: %v", err)
	}
	go echoUDP(pc)
	go serveHTTP()
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatalf("echotarget: %v", err)
		}
		go func() {
			defer c.Close()
			_, _ = io.Copy(c, c)
		}()
	}
}

func echoUDP(pc net.PacketConn) {
	buf := make([]byte, 64<<10)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			log.Fatalf("echotarget: %v", err)
		}
		_, _ = pc.WriteTo(buf[:n], from)
	}
}

func serveHTTP() {
	srv := &http.Server{
		Addr:              ":80",
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "werbot-probe")
		}),
	}
	log.Fatalf("echotarget: %v", srv.ListenAndServe())
}
