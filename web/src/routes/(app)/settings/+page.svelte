<script lang="ts">
	import { onMount } from 'svelte';
	import { durationFromMs, durationMs, type Duration } from '@bufbuild/protobuf/wkt';
	import { Button, SettingsCard, Skeleton, toast } from '#lib/ui/index.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import FormField from '#lib/FormField.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { settings } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { OrgSettings } from '#lib/gen/werbot/v1/settings_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	const HOUR = 3_600_000;
	const DAY = 24 * HOUR;

	// the form edits whole hours and days; saved holds what the server last returned
	let saved = $state({ ttl: '', max: '' });
	let ttl = $state('');
	let max = $state('');
	let loaded = $state(false);
	let failed = $state(false);

	// only the fields the person changed go into update_mask; a number input binds a number
	const paths = $derived(
		[String(ttl) !== saved.ttl && 'cli_session_ttl', String(max) !== saved.max && 'cli_session_max'].filter((p) => p !== false)
	);

	const whole = (d: Duration | undefined, unit: number) => String(Math.round((d ? durationMs(d) : 0) / unit));

	function show(s?: OrgSettings) {
		saved = { ttl: whole(s?.cliSessionTtl, HOUR), max: whole(s?.cliSessionMax, DAY) };
		({ ttl, max } = saved);
	}

	async function load() {
		failed = false;
		try {
			show((await settings.getSettings({})).settings);
			loaded = true;
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		try {
			const res = await settings.updateSettings({
				settings: { cliSessionTtl: durationFromMs(Number(ttl) * HOUR), cliSessionMax: durationFromMs(Number(max) * DAY) },
				updateMask: { paths }
			});
			show(res.settings);
			toast({ tone: 'success', text: m.settings_saved() });
		} catch (err) {
			failToast(err);
		}
	}

	onMount(load);
</script>

<PageHeader title={m.settings_title()} />
<PageBody width={680}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else}
		<SettingsCard icon="laptop" title={m.settings_cli_title()}>
			<form onsubmit={save}>
				<FieldRow even>
					<FormField type="number" step={1} label={m.settings_cli_session_ttl()} hint={m.settings_cli_session_ttl_hint()} bind:value={ttl} required />
					<FormField type="number" step={1} label={m.settings_cli_session_max()} hint={m.settings_cli_session_max_hint()} bind:value={max} required />
				</FieldRow>
				<div><Button size={40} type="submit" variant="solid" disabled={paths.length === 0}>{m.settings_save()}</Button></div>
			</form>
		</SettingsCard>
	{/if}
</PageBody>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}
</style>
