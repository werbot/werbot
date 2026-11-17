package recorder

import (
	"bytes"
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

// maxEventData bounds one event's data: the worst JSON escape (×6) plus the
// header still fits in MaxChunkRaw.
const maxEventData = 8 << 10

// castHeader is the asciicast v2 header line.
func castHeader(cols, rows int, unix int64) []byte {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	b := []byte(`{"version":2,"width":`)
	b = strconv.AppendInt(b, int64(cols), 10)
	b = append(b, `,"height":`...)
	b = strconv.AppendInt(b, int64(rows), 10)
	b = append(b, `,"timestamp":`...)
	b = strconv.AppendInt(b, unix, 10)
	return append(b, "}\n"...)
}

// incompleteTail returns the length of an unfinished UTF-8 sequence at the end of p.
func incompleteTail(p []byte) int {
	for i := 1; i < utf8.UTFMax && i <= len(p); i++ {
		if utf8.RuneStart(p[len(p)-i]) {
			if utf8.FullRune(p[len(p)-i:]) {
				return 0
			}
			return i
		}
	}
	return 0
}

// pieceLen returns how many bytes of p go into one event: at most
// maxEventData, cut at a rune start when one is within utf8.UTFMax bytes.
func pieceLen(p []byte) int {
	if len(p) <= maxEventData {
		return len(p)
	}
	for n := maxEventData; n > maxEventData-utf8.UTFMax; n-- {
		if utf8.RuneStart(p[n]) {
			return n
		}
	}
	return maxEventData
}

// eventLen is the size of the line appendEvent writes.
func eventLen(ts, js []byte) int { return len(ts) + len(js) + len(`[,"o",]`) + 1 }

// appendEvent writes `[<ts>,"<code>",<js>]\n`; js is the JSON-encoded data.
func appendEvent(buf *bytes.Buffer, ts []byte, code byte, js []byte) {
	buf.WriteByte('[')
	buf.Write(ts)
	buf.WriteString(`,"`)
	buf.WriteByte(code)
	buf.WriteString(`",`)
	buf.Write(js)
	buf.WriteString("]\n")
}

// encodeData JSON-encodes data as a string; invalid UTF-8 becomes �.
func encodeData(data []byte) []byte {
	js, _ := json.Marshal(string(data)) // a string always marshals
	return js
}
