<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Segmented, Select, type SelectOption } from '#lib/ui/index.js';
	import { system } from '#lib/api.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { CountryRule } from '#lib/gen/werbot/v1/policy_pb.js';
	import { effectOptions } from './effects.js';
	import GeoipCredit from './GeoipCredit.svelte';

	let { rows = $bindable(), onerror }: { rows: CountryRule[]; onerror: (e: unknown) => void } = $props();

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

<fieldset>
	<legend>{m.rules_countries()}</legend>
	{#each rows as row, i (i)}
		<div class="row">
			<Select {options} placeholder={m.rules_country()} bind:value={row.country} />
			<Segmented options={effectOptions()} bind:value={row.effect} />
			<Button type="button" scheme="w" variant="plain" onclick={() => rows.splice(i, 1)}>{m.rules_remove()}</Button>
		</div>
	{/each}
	<div>
		<Button type="button" variant="plain" onclick={() => rows.push({ $typeName: 'werbot.v1.CountryRule', country: '', effect: 'allow' })}>
			{m.rules_add_country()}
		</Button>
	</div>
	<GeoipCredit />
</fieldset>

<style>
	fieldset {
		display: flex;
		flex-direction: column;
		gap: 8px;
		border: 0;
		padding: 0;
	}

	.row {
		display: flex;
		gap: 8px;
		align-items: flex-end;
	}
</style>
