<script lang="ts">
	import * as AsciinemaPlayer from 'asciinema-player';
	import 'asciinema-player/dist/bundle/asciinema-player.css';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		src,
		onverified,
		onerror,
		onretry
	}: { src: string; onverified: (verified: boolean) => void; onerror?: () => void; onretry?: () => void } = $props();

	let failed = $state(false);

	function mount(el: HTMLElement) {
		const player = AsciinemaPlayer.create(
			{
				// the load error is caught here: 3.17 has no typed player "error" event
				data: async () => {
					try {
						const r = await fetch(src, { credentials: 'same-origin' });
						if (!r.ok) throw new Error(String(r.status));
						const text = await r.text(); // a broken body must not show "verified"
						onverified(r.headers.get('X-Werbot-Recording-Verified') === 'true');
						return text;
					} catch (e) {
						failed = true;
						failToast(m.sessions_recording_load_error());
						onerror?.();
						throw e;
					}
				}
			},
			el,
			{ preload: true, fit: 'width' }
		);
		return () => player.dispose();
	}
</script>

<!-- Retry mounts a fresh player -->
{#if failed}
	<LoadFailed onretry={() => ((failed = false), onretry?.())} />
{:else}
	<div data-testid="cast-player" {@attach mount}></div>
{/if}
