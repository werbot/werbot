import { m } from '#lib/paraglide/messages.js';

/** Close reason the server sends with 1011 when no gateway instance is online. */
const GATEWAY_DOWN = 'Gateway is unavailable';

/** WebSocket URL on the page's own origin: ws for http, wss for https. */
export function wsURL(path: string, params: Record<string, string | number> = {}, loc: Pick<Location, 'protocol' | 'host'> = location): string {
	const query = new URLSearchParams(Object.entries(params).map(([k, v]) => [k, String(v)])).toString();
	return `${loc.protocol === 'https:' ? 'wss:' : 'ws:'}//${loc.host}${path}${query && `?${query}`}`;
}

/** The browser terminal page of resource name. */
export const terminalHref = (name: string): string => `/terminal/${encodeURIComponent(name)}`;

/** Whether a resource of type opens in the browser terminal: its type among types has the terminal kind. */
export function hasTerminal(types: readonly { name: string; kinds: readonly string[] }[], type: string): boolean {
	return types.find((t) => t.name === type)?.kinds.includes('terminal') ?? false;
}

export const resizeFrame = (cols: number, rows: number): string => JSON.stringify({ type: 'resize', cols, rows });

/** The window of a server resize frame; anything else (not JSON, other type, bad numbers) is null. */
export function parseServerText(data: string): { cols: number; rows: number } | null {
	try {
		const v = JSON.parse(data);
		if (v?.type === 'resize' && Number.isInteger(v.cols) && Number.isInteger(v.rows) && v.cols > 0 && v.rows > 0) return { cols: v.cols, rows: v.rows };
	} catch {
		// not JSON: ignored
	}
	return null;
}

export function closeMessage(code: number, reason = ''): string {
	switch (code) {
		case 1000:
			return m.terminal_closed();
		case 4001:
			return m.terminal_mfa_required();
		case 4003:
			return m.terminal_denied();
		case 4004:
			return m.terminal_not_found();
		case 4008:
			return m.terminal_limit();
		case 1011:
			return reason === GATEWAY_DOWN ? m.terminal_unavailable() : m.terminal_error();
		default:
			return m.terminal_error();
	}
}

export interface SocketHandlers {
	ondata: (d: Uint8Array) => void;
	onresize: (cols: number, rows: number) => void;
	onclose: (code: number, reason: string) => void;
}

export interface Socket {
	send: (d: Uint8Array<ArrayBuffer>) => void;
	resize: (cols: number, rows: number) => void;
	close: () => void;
}

/** Opens the stream: binary frames carry terminal bytes, text frames carry resize. No auto reconnect: the page offers a button. */
export function connect(url: string, h: SocketHandlers, WS: typeof WebSocket = WebSocket): Socket {
	const ws = new WS(url);
	ws.binaryType = 'arraybuffer';
	ws.onmessage = (e) => {
		if (typeof e.data === 'string') {
			const w = parseServerText(e.data);
			if (w) h.onresize(w.cols, w.rows);
		} else {
			h.ondata(new Uint8Array(e.data as ArrayBuffer));
		}
	};
	ws.onclose = (e) => h.onclose(e.code, e.reason);
	const open = () => ws.readyState === WS.OPEN;
	return {
		send: (d) => open() && ws.send(d),
		resize: (cols, rows) => open() && ws.send(resizeFrame(cols, rows)),
		close: () => ws.close()
	};
}
