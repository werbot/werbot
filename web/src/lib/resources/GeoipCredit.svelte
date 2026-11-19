<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { serverInfo } from '#lib/version.js';

	// DB-IP Lite requires the credit on the pages that use its data; other GeoIP databases do not.
	const info = serverInfo();
</script>

{#if info.geoipAttribution}
	<a class="attribution" href="https://db-ip.com">{m.geoip_attribution()}</a>
{/if}

<style>
	.attribution {
		align-self: flex-start;
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-tertiary);
		text-decoration: none;
		white-space: nowrap;
	}

	.attribution:hover {
		color: var(--miso-txt-primary);
	}
</style>
