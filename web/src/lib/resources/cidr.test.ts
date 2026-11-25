import { describe, expect, it } from 'vitest';
import { cidrOf } from './cidr';

describe('cidrOf', () => {
	it('masks an IPv4 CIDR to its network', () => {
		expect(cidrOf('10.1.2.3/8')).toBe('10.0.0.0/8');
		expect(cidrOf(' 192.168.1.77/24 ')).toBe('192.168.1.0/24');
		expect(cidrOf('1.2.3.4/0')).toBe('0.0.0.0/0');
	});

	it('gives a bare address a single-host prefix', () => {
		expect(cidrOf('203.0.113.5')).toBe('203.0.113.5/32');
		expect(cidrOf('2001:DB8:0:0::1')).toBe('2001:db8::1/128');
	});

	it('keeps an IPv6 prefix in canonical form', () => {
		expect(cidrOf('2001:0db8::/32')).toBe('2001:db8::/32');
	});

	it('rejects what is not an address', () => {
		for (const bad of ['', 'abc', '10.0.0/8', '10.0.0.256', '10.0.0.0/33', '10.0.0.0/x', '1.2.3.4/8/9', '2001:db8::/129', 'fe80::zz'])
			expect(cidrOf(bad), bad).toBe('');
	});
});
