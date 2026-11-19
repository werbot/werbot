export const KEY_FILE_MAX_BYTES = 16 * 1024;

export type PubKeyFile = { key: string; name: string } | { error: 'too_large' | 'private_key' };

// A .pub file is "<type> <base64> [comment]": the comment names the key, else the file name does.
export function parsePubKeyFile(text: string, fileName: string, maxBytes = KEY_FILE_MAX_BYTES): PubKeyFile {
	if (new TextEncoder().encode(text).length > maxBytes) return { error: 'too_large' };
	if (text.includes('PRIVATE KEY')) return { error: 'private_key' };
	const key = text.trim();
	const comment = key.split(/\s+/).slice(2).join(' ');
	return { key, name: comment || fileName.replace(/\.pub$/, '') };
}

// Rejects on size before reading, so a huge file never loads.
export async function readPubKeyFile(file: File, maxBytes = KEY_FILE_MAX_BYTES): Promise<PubKeyFile> {
	if (file.size > maxBytes) return { error: 'too_large' };
	return parsePubKeyFile(await file.text(), file.name, maxBytes);
}

export type PrivateKeyFile = { key: string } | { error: 'too_large' };

// A private key credential file: only the size is checked here, the server parses the key.
export async function readPrivateKeyFile(file: Blob, maxBytes = KEY_FILE_MAX_BYTES): Promise<PrivateKeyFile> {
	if (file.size > maxBytes) return { error: 'too_large' };
	return { key: await file.text() };
}
