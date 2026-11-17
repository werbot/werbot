<script lang="ts">
	import * as AsciinemaPlayer from 'asciinema-player';
	import 'asciinema-player/dist/bundle/asciinema-player.css';
	import { Banner } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	let { src, onverified }: { src: string; onverified: (verified: boolean) => void } = $props();

	let failed = $state(false);

	function mount(el: HTMLElement) {
		const player = AsciinemaPlayer.create(
			{
				// the load error is caught here: 3.17 has no typed player "error" event
				data: async () => {
					try {
						const r = await fetch(src, { credentials: 'same-origin' });
						if (!r.ok) throw new Error(String(r.status));
						onverified(r.headers.get('X-Werbot-Recording-Verified') === 'true');
						return await r.text();
					} catch (e) {
						failed = true;
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

{#if failed}<Banner tone="alert">{m.sessions_recording_load_error()}</Banner>{/if}
<div data-testid="cast-player" {@attach mount}></div>
