package pgconnector

import (
	"strings"

	"github.com/jackc/pgx/v5/pgproto3"
)

// masks reports whether the session's replies are masked.
func (d *dbSide) masks() bool { return d.lim != nil && d.lim.mask != nil }

// text forwards a reply that carries texts (see replyText) read whole
// within maxParsed; a longer one ends the session. The queue sees the
// database's reply.
func (d *dbSide) text(typ byte, n int) error {
	if n > maxParsed {
		_, _ = d.w.Write(limitMessage(errTooLarge))
		return errTooLarge
	}
	body, err := d.read(n)
	if err != nil {
		return err
	}
	var mask func([]byte) []byte
	if d.masks() {
		mask = d.lim.mask
	}
	out, err := replyText(nil, typ, body, mask)
	if err != nil {
		return err
	}
	if _, err := d.w.Write(out); err != nil {
		return err
	}
	d.reply(typ, body)
	return nil
}

// replyText appends to b the reply typ with body as the client may read it:
// nothing for a database notice in the gateway's name (gatewayPrefix); with
// mask set, the message, detail and hint of an ErrorResponse or
// NoticeResponse, the payload of a NotificationResponse and the result of a
// FunctionCallResponse masked; any other reply as it is.
func replyText(b []byte, typ byte, body []byte, mask func([]byte) []byte) ([]byte, error) {
	switch {
	case typ == 'E' || typ == 'N':
		return errorText(b, typ, body, mask)
	case mask == nil:
	case typ == 'A':
		var m pgproto3.NotificationResponse
		if m.Decode(body) != nil {
			return nil, errMalformed
		}
		m.Payload = maskText(mask, m.Payload)
		return m.Encode(b)
	case typ == 'V':
		var m pgproto3.FunctionCallResponse
		if m.Decode(body) != nil {
			return nil, errMalformed
		}
		if m.Result != nil {
			m.Result = mask(m.Result)
		}
		return m.Encode(b)
	}
	return append(appendHeader(b, typ, len(body)), body...), nil
}

// errorText is replyText of an ErrorResponse or NoticeResponse.
func errorText(b []byte, typ byte, body []byte, mask func([]byte) []byte) ([]byte, error) {
	var e pgproto3.ErrorResponse
	if e.Decode(body) != nil {
		return nil, errMalformed
	}
	switch {
	case typ == 'N' && strings.HasPrefix(e.Message, gatewayPrefix):
		return b, nil
	case mask == nil:
		return append(appendHeader(b, typ, len(body)), body...), nil
	}
	e.Message, e.Detail, e.Hint = maskText(mask, e.Message), maskText(mask, e.Detail), maskText(mask, e.Hint)
	if typ == 'N' {
		return (*pgproto3.NoticeResponse)(&e).Encode(b)
	}
	return e.Encode(b)
}

// maskText is s masked.
func maskText(mask func([]byte) []byte, s string) string {
	if s == "" {
		return s
	}
	return string(mask([]byte(s)))
}
