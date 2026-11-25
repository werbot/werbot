<script lang="ts">
	import { onMount } from 'svelte';
	import type { SelectOption } from '#lib/ui/index.js';
	import PickList from '#lib/requests/PickList.svelte';
	import { system } from '#lib/api.js';
	import { m } from '#lib/paraglide/messages.js';
	import GeoipCredit from './GeoipCredit.svelte';

	/** The countries of one rule list as tags; the search offers only the ones not in it. */
	let { codes = $bindable(), onerror }: { codes: string[]; onerror: (e: unknown) => void } = $props();

	let options = $state<SelectOption[]>([]);

	onMount(async () => {
		try {
			const { countries } = await system.listCountries({});
			options = countries.map((c) => ({ value: c.code, label: `${c.name} (${c.code})` }));
		} catch (e) {
			onerror(e);
		}
	});
</script>

<div class="rules">
	<PickList label={m.rules_add_country()} {options} bind:ids={codes} />
	<GeoipCredit />
</div>

<style>
	.rules {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-08);
	}
</style>
