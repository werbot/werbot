import type { Page } from '@playwright/test';

// rpc calls a Connect method with the JSON codec and the page's session cookie.
export async function rpc<T = any>(page: Page, path: string, body: object): Promise<T> {
	const res = await page.request.post(`/${path}`, { data: body });
	if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
	return res.json();
}

// removeNamed deletes every item called name that `<service>/List<kind>s`
// returns under field, through `<service>/Delete<kind>`: the cleanup a failed
// step would skip.
export async function removeNamed(page: Page, service: string, kind: string, field: string, name: string): Promise<void> {
	const items: { id: string; name: string }[] = (await rpc(page, `werbot.v1.${service}/List${kind}s`, {}))[field] ?? [];
	for (const item of items.filter((i) => i.name === name)) await rpc(page, `werbot.v1.${service}/Delete${kind}`, { id: item.id });
}
