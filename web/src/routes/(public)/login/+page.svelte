<script lang="ts">
	import { goto } from '$app/navigation';
	import { Banner, Button, FilledField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let password = $state('');
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await auth.login({ email, password });
			await goto('/');
		} catch (err) {
			error = errorMessage(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_login_title()}</h1>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
	<FilledField type="password" label={m.field_password()} bind:value={password} required autocomplete="current-password" />
	<Button type="submit" variant="solid" size={48}>{m.auth_login_submit()}</Button>
	<a class="forgot" href="/forgot-password">{m.auth_forgot_link()}</a>
</form>
