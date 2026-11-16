import { Code, ConnectError } from '@connectrpc/connect';
import { auth } from '#lib/api.js';
import type { PageLoad } from './$types';

// Session present → one-click accept; Unauthenticated → registration form.
export const load: PageLoad = async () => {
	try {
		await auth.getSession({});
		return { hasSession: true };
	} catch (e) {
		if (ConnectError.from(e).code === Code.Unauthenticated) return { hasSession: false };
		throw e;
	}
};
