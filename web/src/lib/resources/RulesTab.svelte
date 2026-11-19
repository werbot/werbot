<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Button, toast } from '#lib/ui/index.js';
	import { policy } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import type { CountryRule, NetworkRule } from '#lib/gen/werbot/v1/policy_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import CountryRules from './CountryRules.svelte';
	import NetworkRules from './NetworkRules.svelte';
	import ScheduleGrid from './ScheduleGrid.svelte';

	let { resourceId }: { resourceId: string } = $props();

	let networks = $state<NetworkRule[]>([]);
	let countries = $state<CountryRule[]>([]);
	let slots = $state(new SvelteSet<number>());
	// a save after a failed load would wipe the rules: LoadFailed replaces the form
	let failed = $state(false);

	async function load() {
		failed = false;
		try {
			const rules = await policy.getResourceRules({ resourceId });
			networks = rules.networks;
			countries = rules.countries;
			slots = new SvelteSet(rules.schedule.map((s) => s.weekday * 24 + s.hour));
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	onMount(load);

	async function save(e: SubmitEvent) {
		e.preventDefault();
		try {
			await policy.setResourceRules({
				resourceId,
				networks,
				countries: countries.filter((c) => c.country),
				schedule: [...slots].sort((a, b) => a - b).map((k) => ({ weekday: Math.floor(k / 24), hour: k % 24 }))
			});
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			failToast(e);
		}
	}
</script>

{#if failed}
	<LoadFailed onretry={load} />
{:else}
	<form onsubmit={save}>
		<NetworkRules bind:rows={networks} onerror={failToast} />
		<CountryRules bind:rows={countries} onerror={failToast} />
		<ScheduleGrid bind:slots />
		<div><Button size={40} type="submit" variant="solid">{m.rules_save()}</Button></div>
	</form>
{/if}

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
</style>
