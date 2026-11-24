import { describe, expect, it } from 'vitest';
import { m } from '#lib/paraglide/messages.js';
import { closeMessage, connect, hasTerminal, parseServerText, resizeFrame, terminalHref, wsURL } from './socket.js';

describe('wsURL', () => {
	it('uses ws for http and wss for https, same host', () => {
		expect(wsURL('/api/terminal', {}, { protocol: 'http:', host: 'localhost:8080' })).toBe('ws://localhost:8080/api/terminal');
		expect(wsURL('/api/terminal', {}, { protocol: 'https:', host: 'w.example.com' })).toBe('wss://w.example.com/api/terminal');
	});
	it('encodes query params', () => {
		const u = wsURL('/api/terminal', { resource: 'a b&c', cols: 100, rows: 30 }, { protocol: 'https:', host: 'h' });
		expect(u).toBe('wss://h/api/terminal?resource=a+b%26c&cols=100&rows=30');
	});
});

describe('terminalHref', () => {
	it('encodes the resource name as one path segment', () => {
		expect(terminalHref('db')).toBe('/terminal/db');
		expect(terminalHref('a b/c?d')).toBe('/terminal/a%20b%2Fc%3Fd');
	});
});

describe('hasTerminal', () => {
	const types = [
		{ name: 'ssh', kinds: ['terminal', 'exec', 'stream'] },
		{ name: 'tcp', kinds: ['stream'] }
	];
	it('is true only for a known type with the terminal kind', () => {
		expect(hasTerminal(types, 'ssh')).toBe(true);
		expect(hasTerminal(types, 'tcp')).toBe(false);
		expect(hasTerminal(types, 'unknown')).toBe(false);
		expect(hasTerminal([], 'ssh')).toBe(false);
	});
});

describe('resize framing', () => {
	it('builds the contract frame', () => {
		expect(resizeFrame(100, 30)).toBe('{"type":"resize","cols":100,"rows":30}');
	});
	it('parses only resize with positive integers', () => {
		expect(parseServerText('{"type":"resize","cols":120,"rows":40}')).toEqual({ cols: 120, rows: 40 });
		for (const bad of ['', 'x', '{}', '{"type":"data"}', '{"type":"resize","cols":0,"rows":4}', '{"type":"resize","cols":1.5,"rows":4}', '{"type":"resize","cols":"9","rows":4}', '{"type":"resize","cols":9}', 'null']) {
			expect(parseServerText(bad), bad).toBeNull();
		}
	});
});

describe('closeMessage', () => {
	it('maps contract codes', () => {
		expect(closeMessage(1000)).toBe(m.terminal_closed());
		expect(closeMessage(4001)).toBe(m.terminal_mfa_required());
		expect(closeMessage(4003)).toBe(m.terminal_denied());
		expect(closeMessage(4004)).toBe(m.terminal_not_found());
		expect(closeMessage(4008)).toBe(m.terminal_limit());
	});
	it('tells a down gateway from other 1011 errors', () => {
		expect(closeMessage(1011, 'Gateway is unavailable')).toBe(m.terminal_unavailable());
		expect(closeMessage(1011, 'boom')).toBe(m.terminal_error());
		expect(closeMessage(1011)).toBe(m.terminal_error());
	});
	it('falls back to a generic error', () => {
		expect(closeMessage(1006)).toBe(m.terminal_error());
	});
});

class FakeSocket {
	static last: FakeSocket;
	static OPEN = 1;
	readyState = 1;
	binaryType = '';
	sent: (string | Uint8Array)[] = [];
	closed = false;
	onmessage: ((e: { data: unknown }) => void) | null = null;
	onclose: ((e: { code: number; reason: string }) => void) | null = null;
	constructor(public url: string) {
		FakeSocket.last = this;
	}
	send(d: string | Uint8Array) {
		this.sent.push(d);
	}
	close() {
		this.closed = true;
	}
}

describe('connect', () => {
	const setup = () => {
		const got = { data: [] as Uint8Array[], resize: [] as [number, number][], close: [] as [number, string][] };
		const sock = connect('ws://x/api/terminal', {
			ondata: (d) => got.data.push(d),
			onresize: (c, r) => got.resize.push([c, r]),
			onclose: (c, r) => got.close.push([c, r])
		}, FakeSocket as unknown as typeof WebSocket);
		return { got, sock, ws: FakeSocket.last };
	};

	it('requests arraybuffer frames and routes binary to ondata', () => {
		const { got, ws } = setup();
		expect(ws.url).toBe('ws://x/api/terminal');
		expect(ws.binaryType).toBe('arraybuffer');
		ws.onmessage?.({ data: new Uint8Array([104, 105]).buffer });
		expect(Array.from(got.data[0])).toEqual([104, 105]);
	});
	it('routes text resize to onresize and ignores other text', () => {
		const { got, ws } = setup();
		ws.onmessage?.({ data: resizeFrame(90, 20) });
		ws.onmessage?.({ data: 'noise' });
		expect(got.resize).toEqual([[90, 20]]);
		expect(got.data).toEqual([]);
	});
	it('sends input as binary and resize as text', () => {
		const { sock, ws } = setup();
		const d = new Uint8Array([1, 2]);
		sock.send(d);
		sock.resize(80, 24);
		expect(ws.sent).toEqual([d, resizeFrame(80, 24)]);
	});
	it('does not send when the socket is not open', () => {
		const { sock, ws } = setup();
		ws.readyState = 3;
		sock.send(new Uint8Array([1]));
		sock.resize(80, 24);
		expect(ws.sent).toEqual([]);
	});
	it('reports the close code and reason, and closes on demand', () => {
		const { got, sock, ws } = setup();
		ws.onclose?.({ code: 4003, reason: 'denied' });
		expect(got.close).toEqual([[4003, 'denied']]);
		sock.close();
		expect(ws.closed).toBe(true);
	});
});
