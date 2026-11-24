import { ConnectError } from '@connectrpc/connect';
import { failToast } from '#lib/errors.js';
import { m } from '#lib/paraglide/messages.js';

const text = new TextEncoder();

// The server sends go-webauthn's `{"publicKey": …}` JSON; the browser parses the
// JSON form itself (base64url → buffers) and gives the credential back as JSON.
const publicKey = (options: Uint8Array): unknown => JSON.parse(new TextDecoder().decode(options)).publicKey;

function credentialJSON(cred: Credential | null): Uint8Array {
	if (!cred) throw new Error('no credential');
	return text.encode(JSON.stringify((cred as PublicKeyCredential).toJSON()));
}

/** Creation options JSON → navigator.credentials.create → the attestation JSON as bytes. */
export async function createCredential(options: Uint8Array, creds: CredentialsContainer = navigator.credentials): Promise<Uint8Array> {
	const parsed = PublicKeyCredential.parseCreationOptionsFromJSON(publicKey(options) as PublicKeyCredentialCreationOptionsJSON);
	return credentialJSON(await creds.create({ publicKey: parsed }));
}

/** Request options JSON → navigator.credentials.get → the assertion JSON as bytes. */
export async function getAssertion(options: Uint8Array, creds: CredentialsContainer = navigator.credentials): Promise<Uint8Array> {
	const parsed = PublicKeyCredential.parseRequestOptionsFromJSON(publicKey(options) as PublicKeyCredentialRequestOptionsJSON);
	return credentialJSON(await creds.get({ publicKey: parsed }));
}

/** A server error shows its own text; a dismissed or failed browser prompt shows the passkey message. */
export function failPasskey(e: unknown): void {
	failToast(e instanceof ConnectError ? e : m.mfa_passkey_failed());
}

export const passkeysSupported = (): boolean => typeof PublicKeyCredential !== 'undefined';
