import { describe, expect, it } from 'vitest';
import { address } from './address';

describe('address', () => {
	it('shows host and port of an ssh or telnet config', () => {
		expect(address('{"host":"10.0.0.7","port":22,"login":"root"}')).toBe('10.0.0.7:22');
	});

	it('shows host and port ranges of a tcp or udp config', () => {
		expect(address('{"host":"db","ports":[{"port":5432},{"port":8000,"endPort":8100}]}')).toBe('db:5432, 8000-8100');
	});

	it('shows the host alone without a port', () => {
		expect(address('{"host":"db"}')).toBe('db');
	});

	it('is empty for no host or unreadable json', () => {
		expect(address('')).toBe('');
		expect(address('{"url":"x"}')).toBe('');
		expect(address('{')).toBe('');
	});
});
