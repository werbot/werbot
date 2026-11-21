<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button, FilledField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { safeNext } from '#lib/next.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let password = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			await auth.login({ email, password });
			await goto(safeNext(page.url.searchParams.get('next')));
		} catch (err) {
			failToast(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_login_title()}</h1>
	<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
	<FilledField type="password" label={m.field_password()} bind:value={password} required autocomplete="current-password" />
	<Button type="submit" variant="solid" size={48}>{m.auth_login_submit()}</Button>
	<a class="forgot" href="/forgot-password">{m.auth_forgot_link()}</a>
</form>
