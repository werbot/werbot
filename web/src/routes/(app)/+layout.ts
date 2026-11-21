import { Code, ConnectError } from '@connectrpc/connect';
import { redirect } from '@sveltejs/kit';
import { auth } from '#lib/api.js';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ url, untrack }) => {
	try {
		return { session: await auth.getSession({}) };
	} catch (e) {
		// untracked: reading url would rerun this load on every navigation
		const next = untrack(() => url.pathname + url.search);
		if (ConnectError.from(e).code === Code.Unauthenticated) redirect(307, '/login?next=' + encodeURIComponent(next));
		throw e;
	}
};
