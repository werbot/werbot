<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Banner, Button, EmptyState, InlineEdit, Modal, Table, TextField, Textarea, toast } from '#lib/ui/index.js';
	import { sshKeys } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { SSHKey } from '#lib/gen/werbot/v1/ssh_key_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let keys = $state<SSHKey[]>([]);
	let error = $state('');
	let adding = $state(false);
	let name = $state('');
	let publicKey = $state('');
	let removing = $state<SSHKey | null>(null);

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			keys = (await sshKeys.listSSHKeys({})).keys;
			return true;
		} catch (e) {
			error = errorMessage(e);
			return false;
		}
	}

	async function add(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => sshKeys.addSSHKey({ name, publicKey }))) {
			adding = false;
			name = publicKey = '';
			toast({ tone: 'success', text: m.common_saved() });
		}
	}

	async function remove() {
		const key = removing;
		removing = null;
		if (key) await run(() => sshKeys.deleteSSHKey({ id: key.id }));
	}

	onMount(() => run(async () => {}));
</script>

<h1>{m.keys_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}
<Button variant="solid" onclick={() => (adding = true)}>{m.keys_add()}</Button>

{#if keys.length === 0}
	<EmptyState title={m.keys_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.keys_name()}</th>
			<th>{m.keys_fingerprint()}</th>
			<th>{m.keys_created()}</th>
			<th></th>
		{/snippet}
		{#each keys as key (key.id)}
			<tr>
				<td>
					<InlineEdit value={key.name} placeholder={m.keys_rename()} onsave={(value) => run(() => sshKeys.updateSSHKey({ id: key.id, name: value }))} />
				</td>
				<td><code>{key.fingerprintSha256}</code></td>
				<td>{key.createdAt ? timestampDate(key.createdAt).toLocaleDateString() : ''}</td>
				<td><Button scheme="w" variant="plain" onclick={() => (removing = key)}>{m.keys_delete()}</Button></td>
			</tr>
		{/each}
	</Table>
{/if}

<Modal bind:open={adding} title={m.keys_add()}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<form id="add-key" onsubmit={add}>
		<TextField label={m.keys_name()} bind:value={name} />
		<Textarea label={m.keys_public_key()} bind:value={publicKey} required rows={5} />
	</form>
	{#snippet footer()}
		<Button variant="plain" onclick={() => (adding = false)}>{m.common_cancel()}</Button>
		<Button type="submit" form="add-key" variant="solid">{m.keys_add()}</Button>
	{/snippet}
</Modal>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.keys_delete()}>
	<p>{m.keys_delete_confirm()}</p>
	{#snippet footer()}
		<Button variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={remove}>{m.keys_delete()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
</style>
