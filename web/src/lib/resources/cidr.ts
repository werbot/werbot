// cidrOf turns an address or a CIDR into the form the server stores: an IPv4
// CIDR masked to its network, an IPv6 one in canonical form; a bare address
// gets /32 or /128. It returns '' when the input is not an address.
// ponytail: an IPv6 CIDR is not masked here, the server masks it on save.
export function cidrOf(input: string): string {
	const [addr, bits, extra] = input.trim().split('/');
	if (extra !== undefined || !addr) return '';
	return addr.includes(':') ? v6(addr, bits) : v4(addr, bits);
}

function prefix(bits: string | undefined, max: number): number {
	if (bits === undefined) return max;
	if (!/^\d{1,3}$/.test(bits)) return -1;
	const n = Number(bits);
	return n <= max ? n : -1;
}

function v4(addr: string, bits: string | undefined): string {
	const parts = addr.split('.');
	const n = prefix(bits, 32);
	if (parts.length !== 4 || n < 0 || !parts.every((p) => /^\d{1,3}$/.test(p) && Number(p) <= 255)) return '';
	const ip = parts.reduce((acc, p) => acc * 256 + Number(p), 0);
	const net = n === 0 ? 0 : ip - (ip % 2 ** (32 - n));
	const octets = [24, 16, 8, 0].map((s) => Math.floor(net / 2 ** s) % 256);
	return `${octets.join('.')}/${n}`;
}

function v6(addr: string, bits: string | undefined): string {
	const n = prefix(bits, 128);
	if (n < 0) return '';
	try {
		// the URL parser validates the address and writes it in canonical form
		const host = new URL(`http://[${addr}]/`).hostname;
		return `${host.slice(1, -1)}/${n}`;
	} catch {
		return '';
	}
}
