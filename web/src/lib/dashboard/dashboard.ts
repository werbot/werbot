import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';

// a card is pending until its call settles; a failed call only breaks its own card,
// which then offers to retry that call
export type Slot<T> = { state: 'loading' } | { state: 'error'; retry: () => void } | { state: 'ok'; value: T };

// one page of active sessions is the most the counter can show
export const ACTIVE_PAGE = 200;

export function activeLabel(count: number): string {
	return count >= ACTIVE_PAGE ? `${ACTIVE_PAGE}+` : String(count);
}

export function unhealthy(list: Resource[]): Resource[] {
	return list.filter((r) => r.health?.error).sort((a, b) => a.name.localeCompare(b.name));
}

/** How many of the pools (or instances) are online. */
export function onlineCount(list: { online: boolean }[]): { online: number; total: number } {
	return { online: list.filter((g) => g.online).length, total: list.length };
}
