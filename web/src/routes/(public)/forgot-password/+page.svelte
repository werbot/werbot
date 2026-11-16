<script lang="ts">
	import { Banner, Button, TextField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let sent = $state(false);
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await auth.requestPasswordReset({ email });
			sent = true;
		} catch (err) {
			error = errorMessage(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_forgot_title()}</h1>
	{#if sent}<Banner tone="success">{m.auth_reset_sent()}</Banner>{/if}
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<TextField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
	<Button type="submit" variant="solid">{m.auth_forgot_submit()}</Button>
</form>
