<script lang="ts">
	import { Banner, Button, FilledField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let sent = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			await auth.requestPasswordReset({ email });
			sent = true;
		} catch (err) {
			failToast(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_forgot_title()}</h1>
	{#if sent}<Banner tone="success">{m.auth_reset_sent()}</Banner>{/if}
	<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
	<Button type="submit" variant="solid" size={48}>{m.auth_forgot_submit()}</Button>
</form>
