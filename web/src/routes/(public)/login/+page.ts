import { redirect } from '@sveltejs/kit';
import { auth } from '#lib/api.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	const { needsSetup } = await auth.getSetupState({});
	if (needsSetup) redirect(307, '/setup');
};
