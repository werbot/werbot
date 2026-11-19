import { expect, test } from 'vitest';
import type { Gateway, GatewayInstance } from '#lib/gen/werbot/v1/gateway_pb.js';
import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
import { ACTIVE_PAGE, activeLabel, onlineCount, unhealthy } from './dashboard';

const res = (name: string, error?: string) => ({ name, health: error === undefined ? undefined : { error } }) as Resource;
const pool = (online: boolean) => ({ online }) as Gateway;
const instance = (online: boolean) => ({ online }) as GatewayInstance;

test('active count caps at one page', () => {
	expect(ACTIVE_PAGE).toBe(200);
	expect(activeLabel(0)).toBe('0');
	expect(activeLabel(199)).toBe('199');
	expect(activeLabel(200)).toBe('200+');
});

test('unhealthy keeps resources with a health error, by name', () => {
	const list = [res('b', 'down'), res('a', ''), res('c'), res('a2', 'timeout')];
	expect(unhealthy(list).map((r) => r.name)).toEqual(['a2', 'b']);
});

test('online count counts online pools and instances', () => {
	expect(onlineCount([pool(true), pool(false), pool(true)])).toEqual({ online: 2, total: 3 });
	expect(onlineCount([instance(false), instance(true)])).toEqual({ online: 1, total: 2 });
	expect(onlineCount([])).toEqual({ online: 0, total: 0 });
});
