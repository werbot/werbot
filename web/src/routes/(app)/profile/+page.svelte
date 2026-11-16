<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Banner, Button, SettingsCard, TextField, toast } from '#lib/ui/index.js';
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

<h1>{m.profile_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}

<div class="cards">
	<SettingsCard title={m.profile_title()}>
		<form onsubmit={save}>
			<TextField label={m.field_name()} bind:value={name} required autocomplete="name" />
			<TextField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
			{#if emailChanged}
				<TextField type="password" label={m.profile_email_password()} bind:value={emailPassword} required autocomplete="current-password" />
			{/if}
			<Button type="submit" variant="solid">{m.profile_save()}</Button>
		</form>
	</SettingsCard>

	<SettingsCard title={m.profile_password_title()}>
		<form onsubmit={changePassword}>
			<TextField type="password" label={m.profile_password_current()} bind:value={current} required autocomplete="current-password" />
			<TextField type="password" label={m.profile_password_new()} bind:value={next} required minlength={12} autocomplete="new-password" />
			<Button type="submit" variant="solid">{m.profile_password_submit()}</Button>
		</form>
	</SettingsCard>

	<SettingsCard title={m.profile_delete_title()}>
		<Button scheme="w" variant="solid" onclick={requestDeletion}>{m.profile_delete_submit()}</Button>
	</SettingsCard>
</div>

<style>
	.cards {
		display: flex;
		flex-direction: column;
		gap: 16px;
		max-width: 480px;
	}

	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
</style>
