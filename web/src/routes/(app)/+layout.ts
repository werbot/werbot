import { Code, ConnectError } from '@connectrpc/connect';
import { redirect } from '@sveltejs/kit';
import { auth } from '#lib/api.js';
import type { GetSessionResponse } from '#lib/gen/werbot/v1/auth_pb.js';
import type { LayoutLoad } from './$types';

const ENROLL = '/profile/mfa';

export const load: LayoutLoad = async ({ url, untrack }) => {
	let session: GetSessionResponse;
	try {
		session = await auth.getSession({});
	} catch (e) {
		// untracked: reading url would rerun this load on every navigation
		const next = untrack(() => url.pathname + url.search);
		if (ConnectError.from(e).code === Code.Unauthenticated) redirect(307, '/login?next=' + encodeURIComponent(next));
		throw e;
	}
	// The organization requires a second factor and the user has none: only the enrollment page is usable.
	// url is read (tracked) only here, so while this holds every navigation is checked and otherwise none reruns the load.
	if (session.mfaEnrollmentRequired && url.pathname !== ENROLL) redirect(307, ENROLL);
	return { session };
};
