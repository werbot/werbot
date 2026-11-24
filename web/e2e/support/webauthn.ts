import type { Page } from '@playwright/test';

// A virtual CTAP2 platform authenticator that holds resident keys and approves every
// prompt (user verified), so passkey registration and sign-in run without a person.
export async function addVirtualAuthenticator(page: Page): Promise<void> {
	const cdp = await page.context().newCDPSession(page);
	await cdp.send('WebAuthn.enable');
	await cdp.send('WebAuthn.addVirtualAuthenticator', {
		options: {
			protocol: 'ctap2',
			transport: 'internal',
			hasResidentKey: true,
			hasUserVerification: true,
			isUserVerified: true,
			automaticPresenceSimulation: true
		}
	});
}
