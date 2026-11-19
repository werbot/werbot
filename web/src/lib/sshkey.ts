export const PUB_KEY_MAX_BYTES = 16 * 1024;

export type PubKeyFile = { key: string; name: string } | { error: 'too_large' | 'private_key' };

// A .pub file is "<type> <base64> [comment]": the comment names the key, else the file name does.
export function parsePubKeyFile(text: string, fileName: string, maxBytes = PUB_KEY_MAX_BYTES): PubKeyFile {
	if (new TextEncoder().encode(text).length > maxBytes) return { error: 'too_large' };
	if (text.includes('PRIVATE KEY')) return { error: 'private_key' };
	const key = text.trim();
	const comment = key.split(/\s+/).slice(2).join(' ');
	return { key, name: comment || fileName.replace(/\.pub$/, '') };
}
