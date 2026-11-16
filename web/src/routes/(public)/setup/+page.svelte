<script lang="ts">
	import { goto } from '$app/navigation';
	import { Banner, Button, TextField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let login = $state('');
	let name = $state('');
	let password = $state('');
	let organizationName = $state('');
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await auth.setup({ email, login, name, password, organizationName });
			await goto('/');
		} catch (err) {
			error = errorMessage(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_setup_title()}</h1>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<TextField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
	<TextField label={m.field_login()} bind:value={login} required autocomplete="username" />
	<TextField label={m.field_name()} bind:value={name} required autocomplete="name" />
	<TextField type="password" label={m.field_password()} bind:value={password} required autocomplete="new-password" />
	<TextField label={m.field_org_name()} bind:value={organizationName} required />
	<Button type="submit" variant="solid">{m.auth_setup_submit()}</Button>
</form>
