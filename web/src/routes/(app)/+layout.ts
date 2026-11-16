import { Code, ConnectError } from '@connectrpc/connect';
import { redirect } from '@sveltejs/kit';
import { auth } from '#lib/api.js';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async () => {
	try {
		return { session: await auth.getSession({}) };
	} catch (e) {
		if (ConnectError.from(e).code === Code.Unauthenticated) redirect(307, '/login');
		throw e;
	}
};
