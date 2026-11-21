/**
 * The page to open after login: only a path from the root of this origin.
 * `//host`, `/\host`, a scheme or a control character (browsers drop tabs and
 * newlines, so `/\t/host` becomes `//host`) give `/`.
 */
export function safeNext(s: string | null): string {
	return s && /^\/(?![/\\])[^\x00-\x1f\x7f]*$/.test(s) ? s : '/';
}
