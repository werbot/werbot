<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampFromDate } from '@bufbuild/protobuf/wkt';
	import { Badge, Banner, Button, Chip, Select, TextField, type SelectOption } from '#lib/ui/index.js';
	import { policy, system } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let { resourceId, users, withPorts = false }: { resourceId: string; users: SelectOption[]; withPorts?: boolean } = $props();

	let userId = $state('');
	let ip = $state('');
	let time = $state('');
	let port = $state('');
	let result = $state<{ allow: boolean; reasons: string[] }>();
	let error = $state('');

	onMount(async () => {
		try {
			ip = (await system.getClientIP({})).ip;
		} catch (e) {
			error = errorMessage(e);
		}
	});

	async function check(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			result = await policy.checkAccess({
				userId,
				resourceId,
				ip,
				port: Number(port) || 0,
				time: time ? timestampFromDate(new Date(`${time}Z`)) : undefined // schedules are UTC
			});
		} catch (e) {
			result = undefined;
			error = errorMessage(e);
		}
	}
</script>

<h3>{m.access_check()}</h3>
{#if error}<Banner tone="alert">{error}</Banner>{/if}
<form onsubmit={check}>
	<Select label={m.access_user()} options={users} bind:value={userId} />
	<TextField label={m.access_check_ip()} bind:value={ip} required />
	<TextField type="datetime-local" label={m.access_check_time()} bind:value={time} />
	{#if withPorts}<TextField type="number" min={1} max={65535} label={m.access_check_port()} bind:value={port} />{/if}
	<div><Button type="submit" variant="solid" disabled={!userId}>{m.access_check_run()}</Button></div>
</form>
{#if result}
	<p>
		<Badge tone={result.allow ? 'accent' : 'alert'}>{result.allow ? m.access_allowed() : m.access_denied()}</Badge>
		{#each result.reasons as reason (reason)}<Chip>{reason}</Chip>{/each}
	</p>
{/if}

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
		max-width: 480px;
	}
</style>
