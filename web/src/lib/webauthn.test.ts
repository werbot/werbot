import { afterEach, expect, test, vi } from 'vitest';
import { createCredential, getAssertion } from './webauthn.js';

const bytes = (v: unknown) => new TextEncoder().encode(JSON.stringify(v));
const json = (b: Uint8Array) => JSON.parse(new TextDecoder().decode(b));

afterEach(() => vi.unstubAllGlobals());

function stub(result: unknown) {
	vi.stubGlobal('PublicKeyCredential', {
		parseCreationOptionsFromJSON: (o: unknown) => ({ parsedCreation: o }),
		parseRequestOptionsFromJSON: (o: unknown) => ({ parsedRequest: o })
	});
	const seen: unknown[] = [];
	const creds = {
		create: async (o: unknown) => (seen.push(o), result),
		get: async (o: unknown) => (seen.push(o), result)
	} as unknown as CredentialsContainer;
	return { seen, creds };
}

test('createCredential passes the parsed publicKey and returns the credential JSON as bytes', async () => {
	const { seen, creds } = stub({ toJSON: () => ({ id: 'abc', type: 'public-key' }) });
	const out = await createCredential(bytes({ publicKey: { challenge: 'c' } }), creds);
	expect(seen).toEqual([{ publicKey: { parsedCreation: { challenge: 'c' } } }]);
	expect(json(out)).toEqual({ id: 'abc', type: 'public-key' });
});

test('getAssertion passes the parsed request options', async () => {
	const { seen, creds } = stub({ toJSON: () => ({ id: 'xyz' }) });
	const out = await getAssertion(bytes({ publicKey: { challenge: 'r' } }), creds);
	expect(seen).toEqual([{ publicKey: { parsedRequest: { challenge: 'r' } } }]);
	expect(json(out)).toEqual({ id: 'xyz' });
});

test('a dismissed prompt (no credential) is an error', async () => {
	const { creds } = stub(null);
	await expect(getAssertion(bytes({ publicKey: {} }), creds)).rejects.toThrow();
});
