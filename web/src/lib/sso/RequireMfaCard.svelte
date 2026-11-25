<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Segmented, SettingsCard, Skeleton, toast } from '#lib/ui/index.js';
	import { settings } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const modes = [
		{ value: 'off', label: m.sso_require_mfa_off() },
		{ value: 'admins', label: m.sso_require_mfa_admins() },
		{ value: 'all', label: m.sso_require_mfa_all() }
	];

	let saved = $state('');
	let value = $state('off');
	let loaded = $state(false);
	let failed = $state(false);

	function show(mode: string) {
		saved = value = mode || 'off';
	}

	async function load() {
		failed = false;
		try {
			show((await settings.getSettings({})).settings?.requireMfa ?? '');
			loaded = true;
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		try {
			const res = await settings.updateSettings({ settings: { requireMfa: value }, updateMask: { paths: ['require_mfa'] } });
			show(res.settings?.requireMfa ?? '');
			toast({ tone: 'success', text: m.settings_saved() });
		} catch (err) {
			failToast(err);
		}
	}

	onMount(load);
</script>

<SettingsCard icon="lock" title={m.sso_require_mfa()}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={2} />
	{:else}
		<form onsubmit={save}>
			<Segmented size={32} options={modes} bind:value />
			<p class="hint">{m.sso_require_mfa_hint()}</p>
			<div><Button size={40} type="submit" variant="solid" disabled={value === saved}>{m.settings_save()}</Button></div>
		</form>
	{/if}
</SettingsCard>

<style>
	form {
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
