<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Banner, Button, FilledField, InfoBlock, SettingsCard, toast } from '#lib/ui/index.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { profile } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let name = $state('');
	let email = $state('');
	let savedEmail = $state('');
	let emailPassword = $state('');
	const emailChanged = $derived(email.trim().toLowerCase() !== savedEmail);
	let current = $state('');
	let next = $state('');
	let error = $state('');

	async function run(action: () => Promise<unknown>, message: string) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: message });
			return true;
		} catch (e) {
			error = errorMessage(e);
			return false;
		}
	}

	// A stale tab may hold an old email: re-read it so the password field shows.
	async function refreshSavedEmail() {
		try {
			savedEmail = (await profile.getProfile({})).user?.email ?? savedEmail;
		} catch {
			// keep savedEmail: the save error is already shown
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			const { user } = await profile.updateProfile({ name, email, currentPassword: emailChanged ? emailPassword : '' });
			savedEmail = user?.email ?? '';
			emailPassword = '';
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			error = errorMessage(e);
			if (ConnectError.from(e).code === Code.InvalidArgument) await refreshSavedEmail();
		}
	}

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => profile.changePassword({ currentPassword: current, newPassword: next }), m.common_saved())) {
			current = next = '';
		}
	}

	onMount(async () => {
		try {
			const { user } = await profile.getProfile({});
			name = user?.name ?? '';
			email = savedEmail = user?.email ?? '';
		} catch (e) {
			error = errorMessage(e);
		}
	});

	async function requestDeletion() {
		error = '';
		try {
			await profile.requestAccountDeletion({});
			toast({ tone: 'success', text: m.profile_delete_sent() });
		} catch (e) {
			error = ConnectError.from(e).code === Code.FailedPrecondition ? m.profile_delete_last_owner() : errorMessage(e);
		}
	}
</script>

<PageHeader title={m.profile_title()} />
<PageBody width={680}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}

	<SettingsCard icon="person" title={m.profile_title()}>
		<form onsubmit={save}>
			<FilledField label={m.field_name()} bind:value={name} required autocomplete="name" />
			<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
			{#if emailChanged}
				<FilledField type="password" label={m.profile_email_password()} bind:value={emailPassword} required autocomplete="current-password" />
			{/if}
			<div><Button type="submit" variant="solid">{m.profile_save()}</Button></div>
		</form>
	</SettingsCard>

	<SettingsCard icon="lock" title={m.profile_password_title()}>
		<form onsubmit={changePassword}>
			<FilledField type="password" label={m.profile_password_current()} bind:value={current} required autocomplete="current-password" />
			<FilledField type="password" label={m.profile_password_new()} bind:value={next} required minlength={12} autocomplete="new-password" />
			<div><Button type="submit" variant="solid">{m.profile_password_submit()}</Button></div>
		</form>
	</SettingsCard>

	<SettingsCard icon="trash_can" title={m.profile_delete_title()}>
		<div class="danger">
			<InfoBlock tone="warning" icon="exclam_triangle-line" title={m.common_warning()}>{m.profile_delete_warning()}</InfoBlock>
			<div><Button scheme="w" variant="solid" onclick={requestDeletion}>{m.profile_delete_submit()}</Button></div>
		</div>
	</SettingsCard>
</PageBody>

<style>
	form,
	.danger {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}
</style>
