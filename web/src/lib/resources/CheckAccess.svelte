<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampFromDate } from '@bufbuild/protobuf/wkt';
	import FilledShell from '#lib/FilledShell.svelte';
	import { Badge, Button, Chip, FilledField, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import { policy, system } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import GeoipCredit from './GeoipCredit.svelte';

	let { resourceId, users, withPorts = false }: { resourceId: string; users: SelectOption[]; withPorts?: boolean } = $props();

	let userId = $state('');
	let ip = $state('');
	let time = $state('');
	let port = $state('');
	let result = $state<{ allow: boolean; reasons: string[] }>();

	onMount(async () => {
		try {
			ip = (await system.getClientIP({})).ip;
		} catch (e) {
			failToast(e);
		}
	});

	async function check(e: SubmitEvent) {
		e.preventDefault();
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
			failToast(e);
		}
	}
</script>

<h3>{m.access_check()}</h3>
<form onsubmit={check}>
	<FilledSelect label={m.access_user()} options={users} bind:value={userId} />
	<FilledField label={m.access_check_ip()} bind:value={ip} required />
	<FilledShell label={m.access_check_time()}>
		{#snippet children(id)}<input {id} type="datetime-local" bind:value={time} />{/snippet}
	</FilledShell>
	{#if withPorts}<FilledField type="number" min={1} max={65535} label={m.access_check_port()} bind:value={port} />{/if}
	<div><Button size={40} type="submit" variant="solid" disabled={!userId}>{m.access_check_run()}</Button></div>
	<GeoipCredit />
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
		gap: var(--miso-sp-12);
		max-width: 480px;
	}
</style>
