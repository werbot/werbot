// Command echotarget is the TCP and UDP echo target of the network e2e
// (deploy/e2e): it sends every TCP stream and UDP datagram back unchanged.
// It is not part of the werbot binary.
package main

import (
	"io"
	"log"
	"net"
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
