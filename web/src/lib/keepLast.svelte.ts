/** Last non-empty value of `get`: keeps a drawer's content on screen while its close animation plays. */
export function keepLast<T>(get: () => T | null | undefined) {
	let last = $state.raw<T | null>(null);
	$effect.pre(() => {
		const v = get();
		if (v) last = v;
	});
	return {
		get value() {
			return last;
		}
	};
}
