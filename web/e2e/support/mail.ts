import { readFile } from 'node:fs/promises';
import { expect } from '@playwright/test';

const LOG = 'e2e/.server.log';

// Reads the newest LogMailer record for `to` from the server log and returns its first link to `path`.
export async function latestEmailLink(to: string, path: 'invite' | 'reset-password' | 'profile/delete'): Promise<string> {
	const re = new RegExp(`http://127\\.0\\.0\\.1:18080/${path}/[A-Za-z0-9_-]{43}`);
	let link = '';
	await expect
		.poll(
			async () => {
				const lines = (await readFile(LOG, 'utf8').catch(() => '')).split('\n').reverse();
				for (const line of lines) {
					if (!line.startsWith('{')) continue;
					const rec = JSON.parse(line) as { msg?: string; to?: string; body?: string };
					if (rec.msg === 'email (log mailer)' && rec.to === to) {
						link = rec.body?.match(re)?.[0] ?? '';
						return link;
					}
				}
				return '';
			},
			{ timeout: 5_000 }
		)
		.not.toBe('');
	return link;
}
