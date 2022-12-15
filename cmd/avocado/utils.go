package main

import (
	"bytes"

	gossh "golang.org/x/crypto/ssh"
)

// Отправляет сообщение в канал
func sendMessageInChannel(ch gossh.Channel, msg string) {
	p := []byte(msg)
	p = bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\r', '\n'})
	p = bytes.ReplaceAll(p, []byte{'\r', '\r', '\n'}, []byte{'\r', '\n'})
	if _, err := ch.Write(p); err != nil {
		app.log.Error(err).Msg("sendMessageInChannel Write")
	}
}
