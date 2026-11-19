import { describe, expect, it } from 'vitest';
import { parsePubKeyFile, readPrivateKeyFile } from './sshkey.js';

describe('parsePubKeyFile', () => {
	it('uses the full comment as name', () => {
		expect(parsePubKeyFile('ssh-ed25519 AAAA my work laptop\n', 'id.pub')).toEqual({ key: 'ssh-ed25519 AAAA my work laptop', name: 'my work laptop' });
	});
	it('falls back to the file name without .pub', () => {
		expect(parsePubKeyFile('ssh-ed25519 AAAA\n', 'id_ed25519.pub')).toEqual({ key: 'ssh-ed25519 AAAA', name: 'id_ed25519' });
	});
	it('rejects a private key', () => {
		expect(parsePubKeyFile('-----BEGIN OPENSSH PRIVATE KEY-----\nx', 'id')).toEqual({ error: 'private_key' });
	});
	it('rejects a too large file', () => {
		expect(parsePubKeyFile('a'.repeat(11), 'id', 10)).toEqual({ error: 'too_large' });
	});
});

describe('readPrivateKeyFile', () => {
	it('returns the file text', async () => {
		const pem = '-----BEGIN OPENSSH PRIVATE KEY-----\nx\n-----END OPENSSH PRIVATE KEY-----\n';
		await expect(readPrivateKeyFile(new Blob([pem]))).resolves.toEqual({ key: pem });
	});
	it('rejects a too large file', async () => {
		await expect(readPrivateKeyFile(new Blob(['a'.repeat(11)]), 10)).resolves.toEqual({ error: 'too_large' });
	});
});
