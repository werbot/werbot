<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Banner, Button, TextField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let password = $state('');
	let repeat = $state('');
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (password !== repeat) {
			error = m.auth_password_mismatch();
			return;
		}
		try {
			await auth.resetPassword({ token: page.params.token, newPassword: password });
			await goto('/login');
		} catch (err) {
			error = errorMessage(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_reset_title()}</h1>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<TextField type="password" label={m.field_password()} bind:value={password} required autocomplete="new-password" />
	<TextField type="password" label={m.field_password_repeat()} bind:value={repeat} required autocomplete="new-password" />
	<Button type="submit" variant="solid">{m.auth_reset_submit()}</Button>
</form>
