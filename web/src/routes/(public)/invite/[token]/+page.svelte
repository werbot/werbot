<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Banner, Button, TextField } from '#lib/ui/index.js';
	import { members } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let login = $state('');
	let name = $state('');
	let password = $state('');
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await members.acceptInvite({ token: page.params.token, login, name, password });
			await goto('/');
		} catch (err) {
			error = errorMessage(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.invite_title()}</h1>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	{#if !data.hasSession}
		<Banner tone="info">{m.invite_existing_hint()}</Banner>
		<TextField label={m.field_login()} bind:value={login} autocomplete="username" />
		<TextField label={m.field_name()} bind:value={name} autocomplete="name" />
		<TextField type="password" label={m.field_password()} bind:value={password} required autocomplete="current-password" />
	{/if}
	<Button type="submit" variant="solid">{m.invite_accept_submit()}</Button>
</form>
