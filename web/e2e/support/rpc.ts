import type { Page } from '@playwright/test';

// rpc calls a Connect method with the JSON codec and the page's session cookie.
export async function rpc<T = any>(page: Page, path: string, body: object): Promise<T> {
	const res = await page.request.post(`/${path}`, { data: body });
	if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
	return res.json();
}
