<script lang="ts">
	import { onMount } from 'svelte';
	import type { MessageInitShape } from '@bufbuild/protobuf';
	import { Button, InfoBlock, Segmented, SettingsCard, Skeleton, toast } from '#lib/ui/index.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import FormField from '#lib/FormField.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { settings } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration, parseDuration } from '#lib/duration.js';
	import type { OrgSettings, OrgSettingsSchema } from '#lib/gen/werbot/v1/settings_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	type DurationKey = 'cliSessionTtl' | 'cliSessionMax' | 'grantDefaultTtl' | 'grantMaxTtl' | 'requestPendingTtl' | 'commandApprovalTtl';
	type Key = DurationKey | 'lockMode';
	type Field = { key: DurationKey; label: () => string; hint: () => string };

	const cliFields: Field[] = [
		{ key: 'cliSessionTtl', label: m.settings_cli_session_ttl, hint: m.settings_cli_session_ttl_hint },
		{ key: 'cliSessionMax', label: m.settings_cli_session_max, hint: m.settings_cli_session_max_hint }
	];
	const accessFields: Field[] = [
		{ key: 'grantDefaultTtl', label: m.settings_grant_default_ttl, hint: m.settings_grant_default_ttl_hint },
		{ key: 'grantMaxTtl', label: m.settings_grant_max_ttl, hint: m.settings_grant_max_ttl_hint },
		{ key: 'requestPendingTtl', label: m.settings_request_pending_ttl, hint: m.settings_request_pending_ttl_hint },
		{ key: 'commandApprovalTtl', label: m.settings_command_approval_ttl, hint: m.settings_command_approval_ttl_hint }
	];
	const durationKeys = [...cliFields, ...accessFields].map((f) => f.key);
	const lockModes = [
		{ value: 'best_effort', label: m.settings_lock_mode_best_effort() },
		{ value: 'strict', label: m.settings_lock_mode_strict() }
	];

	// durations are edited as text ("12h", "30d"); saved holds what the server last returned
	let saved = $state({} as Record<Key, string>);
	let form = $state({} as Record<Key, string>);
	let errors = $state<Partial<Record<Key, string>>>({});
	let loaded = $state(false);
	let failed = $state(false);

	// only the fields the person changed go into update_mask
	const changed = (keys: Key[]) => keys.filter((k) => form[k] !== saved[k]);
	const snake = (k: string) => k.replace(/[A-Z]/g, (c) => '_' + c.toLowerCase());

	function show(s?: OrgSettings) {
		saved = { ...Object.fromEntries(durationKeys.map((k) => [k, formatDuration(s?.[k])])), lockMode: s?.lockMode || 'best_effort' } as Record<Key, string>;
		form = { ...saved };
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

	async function save(e: SubmitEvent, keys: Key[]) {
		e.preventDefault();
		const paths = changed(keys);
		const out: MessageInitShape<typeof OrgSettingsSchema> = {};
		errors = {};
		for (const k of paths) {
			if (k === 'lockMode') {
				out.lockMode = form.lockMode;
				continue;
			}
			const d = parseDuration(form[k]);
			if (d) out[k] = d;
			else errors[k] = m.common_duration_invalid();
		}
		if (Object.keys(errors).length > 0) return;
		try {
			const res = await settings.updateSettings({ settings: out, updateMask: { paths: paths.map(snake) } });
			show(res.settings);
			toast({ tone: 'success', text: m.settings_saved() });
		} catch (err) {
			failToast(err);
		}
	}

	onMount(load);
</script>

<!-- two fields per row: the even grid wraps the rest -->
{#snippet durations(fields: Field[])}
	<FieldRow even>
		{#each fields as f (f.key)}
			<FormField label={f.label()} hint={f.hint()} error={errors[f.key]} bind:value={form[f.key]} required />
		{/each}
	</FieldRow>
{/snippet}

<PageHeader title={m.settings_title()} />
<PageBody width={680}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else}
		{@const cliKeys = cliFields.map((f) => f.key)}
		{@const accessKeys = [...accessFields.map((f) => f.key), 'lockMode' as const]}
		<div class="cards">
			<SettingsCard icon="laptop" title={m.settings_cli_title()}>
				<form onsubmit={(e) => save(e, cliKeys)}>
					{@render durations(cliFields)}
					<div><Button size={40} type="submit" variant="solid" disabled={changed(cliKeys).length === 0}>{m.settings_save()}</Button></div>
				</form>
			</SettingsCard>
			<SettingsCard icon="hourglass" title={m.settings_access_title()}>
				<form onsubmit={(e) => save(e, accessKeys)}>
					{@render durations(accessFields)}
					<div class="mode">
						<span>{m.settings_lock_mode()}</span>
						<Segmented size={32} options={lockModes} bind:value={form.lockMode} />
					</div>
					<InfoBlock title={m.settings_lock_mode_strict()}>{m.settings_lock_mode_strict_hint()}</InfoBlock>
					<div><Button size={40} type="submit" variant="solid" disabled={changed(accessKeys).length === 0}>{m.settings_save()}</Button></div>
				</form>
			</SettingsCard>
		</div>
	{/if}
</PageBody>

<style>
	.cards,
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.cards {
		gap: var(--miso-sp-24);
	}

	.mode {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--miso-sp-12);
		font: var(--miso-font-ui-13-semi);
	}
</style>
