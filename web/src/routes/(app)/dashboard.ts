import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';

// one page of active sessions is the most the counter can show
export const ACTIVE_PAGE = 200;

export function activeLabel(count: number): string {
	return count >= ACTIVE_PAGE ? `${ACTIVE_PAGE}+` : String(count);
}

export function unhealthy(list: Resource[]): Resource[] {
	return list.filter((r) => r.health?.error).sort((a, b) => a.name.localeCompare(b.name));
}

export function poolsOnline(list: Gateway[]): { online: number; total: number } {
	return { online: list.filter((g) => g.online).length, total: list.length };
}
