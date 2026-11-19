<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, FilledField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let login = $state('');
	let name = $state('');
	let password = $state('');
	let organizationName = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			await auth.setup({ email, login, name, password, organizationName });
			await goto('/');
		} catch (err) {
			failToast(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_setup_title()}</h1>
	<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
	<FilledField label={m.field_login()} bind:value={login} required autocomplete="username" />
	<FilledField label={m.field_name()} bind:value={name} required autocomplete="name" />
	<FilledField type="password" label={m.field_password()} bind:value={password} required autocomplete="new-password" />
	<FilledField label={m.field_org_name()} bind:value={organizationName} required />
	<Button type="submit" variant="solid" size={48}>{m.auth_setup_submit()}</Button>
</form>
