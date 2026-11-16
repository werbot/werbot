<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Banner, Button, toast } from '#lib/ui/index.js';
	import { policy } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { CountryRule, NetworkRule } from '#lib/gen/werbot/v1/policy_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import CountryRules from './CountryRules.svelte';
	import NetworkRules from './NetworkRules.svelte';
	import ScheduleGrid from './ScheduleGrid.svelte';

	let { resourceId }: { resourceId: string } = $props();

	let networks = $state<NetworkRule[]>([]);
	let countries = $state<CountryRule[]>([]);
	let slots = $state(new SvelteSet<number>());
	let error = $state('');

	const fail = (e: unknown) => (error = errorMessage(e));

	onMount(async () => {
		try {
			const rules = await policy.getResourceRules({ resourceId });
			networks = rules.networks;
			countries = rules.countries;
			slots = new SvelteSet(rules.schedule.map((s) => s.weekday * 24 + s.hour));
		} catch (e) {
			fail(e);
		}
	});

	async function save(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await policy.setResourceRules({
				resourceId,
				networks,
				countries: countries.filter((c) => c.country),
				schedule: [...slots].sort((a, b) => a - b).map((k) => ({ weekday: Math.floor(k / 24), hour: k % 24 }))
			});
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			fail(e);
		}
	}
</script>

{#if error}<Banner tone="alert">{error}</Banner>{/if}
<form onsubmit={save}>
	<NetworkRules bind:rows={networks} onerror={fail} />
	<CountryRules bind:rows={countries} onerror={fail} />
	<ScheduleGrid bind:slots />
	<div><Button type="submit" variant="solid">{m.rules_save()}</Button></div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
</style>
