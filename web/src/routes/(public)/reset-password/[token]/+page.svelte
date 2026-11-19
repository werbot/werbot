<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button, FilledField } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let password = $state('');
	let repeat = $state('');
	let mismatch = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		mismatch = '';
		if (password !== repeat) {
			mismatch = m.auth_password_mismatch();
			return;
		}
		try {
			await auth.resetPassword({ token: page.params.token, newPassword: password });
			await goto('/login');
		} catch (err) {
			failToast(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_reset_title()}</h1>
	<FilledField type="password" label={m.field_password()} bind:value={password} required autocomplete="new-password" />
	<FormField type="password" label={m.field_password_repeat()} bind:value={repeat} required autocomplete="new-password" error={mismatch} />
	<Button type="submit" variant="solid" size={48}>{m.auth_reset_submit()}</Button>
</form>
