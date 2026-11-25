import { describe, expect, it } from 'vitest';
import { Role } from '#lib/gen/werbot/v1/user_pb.js';
import { canControlSession } from './access.js';

describe('canControlSession', () => {
	it('lets owner and admin control any session', () => {
		expect(canControlSession(Role.OWNER, 'root', 'alice')).toBe(true);
		expect(canControlSession(Role.ADMIN, 'root', 'alice')).toBe(true);
	});
	it('lets a member control only their own session', () => {
		expect(canControlSession(Role.MEMBER, 'alice', 'alice')).toBe(true);
		expect(canControlSession(Role.MEMBER, 'alice', 'bob')).toBe(false);
	});
	it('refuses without a known viewer or session user', () => {
		expect(canControlSession(Role.MEMBER, undefined, 'alice')).toBe(false);
		expect(canControlSession(Role.MEMBER, '', '')).toBe(false);
		expect(canControlSession(undefined, 'alice', 'alice')).toBe(true);
	});
});
