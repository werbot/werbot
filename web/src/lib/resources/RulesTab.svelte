<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Button, InfoBlock, SettingsCard, toast } from '#lib/ui/index.js';
	import ToneSegmented from '#lib/ToneSegmented.svelte';
	import { policy } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import CountryRules from './CountryRules.svelte';
	import { modeOf, modeOptions, type Mode } from './effects.js';
	import NetworkRules from './NetworkRules.svelte';
	import ScheduleGrid from './ScheduleGrid.svelte';

	let { resourceId }: { resourceId: string } = $props();

	// each list has one effect for all its entries, so an entry cannot be allowed and denied at once
	let cidrs = $state<string[]>([]);
	let netMode = $state<Mode>('allow');
	let netMixed = $state(false);
	let codes = $state<string[]>([]);
	let countryMode = $state<Mode>('allow');
	let countryMixed = $state(false);
	let slots = $state(new SvelteSet<number>());
	// a save after a failed load would wipe the rules: LoadFailed replaces the form
	let failed = $state(false);

	async function load() {
		failed = false;
		try {
			const rules = await policy.getResourceRules({ resourceId });
			cidrs = rules.networks.map((n) => n.cidr);
			({ mode: netMode, mixed: netMixed } = modeOf(rules.networks.map((n) => n.effect)));
			codes = rules.countries.map((c) => c.country);
			({ mode: countryMode, mixed: countryMixed } = modeOf(rules.countries.map((c) => c.effect)));
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
				networks: cidrs.map((cidr) => ({ cidr, effect: netMode })),
				countries: codes.map((country) => ({ country, effect: countryMode })),
				schedule: [...slots].sort((a, b) => a - b).map((k) => ({ weekday: Math.floor(k / 24), hour: k % 24 }))
			});
			netMixed = countryMixed = false;
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
		<SettingsCard icon="security_ip" title={m.rules_networks()}>
			{#snippet actions()}<ToneSegmented size={28} options={modeOptions()} bind:value={netMode} />{/snippet}
			<div class="body">
				<p class="hint">{netMode === 'allow' ? m.rules_networks_allow_hint() : m.rules_networks_deny_hint()}</p>
				{#if netMixed}<InfoBlock tone="warning">{m.rules_mixed()}</InfoBlock>{/if}
				<NetworkRules bind:cidrs onerror={failToast} />
			</div>
		</SettingsCard>
		<SettingsCard icon="globe" title={m.rules_countries()}>
			{#snippet actions()}<ToneSegmented size={28} options={modeOptions()} bind:value={countryMode} />{/snippet}
			<div class="body">
				<p class="hint">{countryMode === 'allow' ? m.rules_countries_allow_hint() : m.rules_countries_deny_hint()}</p>
				{#if countryMixed}<InfoBlock tone="warning">{m.rules_mixed()}</InfoBlock>{/if}
				<CountryRules bind:codes onerror={failToast} />
			</div>
		</SettingsCard>
		<SettingsCard icon="calendar_clock" title={m.rules_schedule()}>
			<ScheduleGrid bind:slots />
		</SettingsCard>
		<div><Button size={40} type="submit" variant="solid">{m.rules_save()}</Button></div>
	</form>
{/if}

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-24);
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
