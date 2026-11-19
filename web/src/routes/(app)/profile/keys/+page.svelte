<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Banner, Button, DropZone, EmptyState, FileItem, InlineEdit, Modal, Skeleton, Table, TextField, Textarea, Tooltip, toast } from '#lib/ui/index.js';
	import { PUB_KEY_MAX_BYTES, parsePubKeyFile } from '#lib/sshkey.js';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { sshKeys } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { SSHKey } from '#lib/gen/werbot/v1/ssh_key_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let keys = $state<SSHKey[]>([]);
	let loaded = $state(false);
	let fileName = $state('');
	let autoName = '';
	let autoKey = '';
	let picker = $state<HTMLInputElement>();
	let error = $state('');
	let adding = $state(false);
	let name = $state('');
	let publicKey = $state('');
	let removing = $state<SSHKey | null>(null);

	async function refresh() {
		keys = (await sshKeys.listSSHKeys({})).keys;
		loaded = true;
	}

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			error = errorMessage(e);
			await refresh().catch(() => {});
			return false;
		}
		await refresh().catch((e) => (error = errorMessage(e)));
		return true;
	}

	async function add(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => sshKeys.addSSHKey({ name, publicKey }))) {
			setAdding(false);
		}
	}

	async function readKeyFile(file?: File) {
		if (!file) return;
		error = '';
		try {
			// one byte over the limit is enough to reject, without reading a huge file
			const parsed = parsePubKeyFile(await file.slice(0, PUB_KEY_MAX_BYTES + 1).text(), file.name);
			if ('error' in parsed) {
				error = parsed.error === 'too_large' ? m.keys_file_too_large() : m.keys_file_private_key();
				return;
			}
			publicKey = autoKey = parsed.key;
			fileName = file.name;
			if (!name.trim() || name === autoName) name = autoName = parsed.name;
		} catch (e) {
			error = errorMessage(e);
		} finally {
			if (picker) picker.value = '';
		}
	}

	// the chip removes only what the file filled in, never text the user edited
	function clearFile() {
		if (publicKey === autoKey) publicKey = '';
		if (name === autoName) name = '';
		fileName = autoName = autoKey = '';
	}

	function setAdding(open: boolean) {
		adding = open;
		if (!open) {
			name = publicKey = fileName = autoName = autoKey = error = '';
		}
	}

	async function remove() {
		const key = removing;
		removing = null;
		if (key) await run(() => sshKeys.deleteSSHKey({ id: key.id }));
	}

	onMount(() => refresh().catch((e) => (error = errorMessage(e))));
</script>

<PageHeader title={m.keys_title()} count={loaded ? keys.length : undefined}>
	{#snippet toolbar()}
		{#if keys.length > 0}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (adding = true)}>{m.keys_add()}</Button>
		{/if}
	{/snippet}
</PageHeader>
{#if error && !adding}<Banner tone="alert">{error}</Banner>{/if}

{#if !loaded}
	{#if !error}<Skeleton variant="text" lines={4} />{/if}
{:else if keys.length === 0}
	<EmptyState icon="key" title={m.keys_empty()}>
		{#snippet actions()}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (adding = true)}>{m.keys_add()}</Button>
		{/snippet}
	</EmptyState>
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
				<td>
					<Tooltip text={key.fingerprintSha256}><code class="fp">{key.fingerprintSha256}</code></Tooltip>
				</td>
				<td>{key.createdAt ? timestampDate(key.createdAt).toLocaleDateString() : ''}</td>
				<td><Button scheme="w" variant="plain" onclick={() => (removing = key)}>{m.keys_delete()}</Button></td>
			</tr>
		{/each}
	</Table>
{/if}

<Modal bind:open={() => adding, setAdding} title={m.keys_add()}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<form id="add-key" onsubmit={add}>
		<DropZone label={m.keys_drop_hint()} ondropfiles={(files) => readKeyFile(files[0])}>
			<div class="drop">
				{#if fileName}
					<FileItem name={fileName} ondelete={clearFile} />
				{:else}
					<span class="hint">{m.keys_drop_hint()}</span>
					<Button scheme="g" variant="plain" size={28} iconLeft="upload" onclick={() => picker?.click()}>{m.keys_choose_file()}</Button>
				{/if}
				<input bind:this={picker} type="file" accept=".pub" hidden onchange={(e) => readKeyFile(e.currentTarget.files?.[0])} />
			</div>
		</DropZone>
		<TextField label={m.keys_name()} bind:value={name} />
		<Textarea label={m.keys_public_key()} bind:value={publicKey} required rows={5} />
	</form>
	{#snippet footer()}
		<Button variant="plain" onclick={() => setAdding(false)}>{m.common_cancel()}</Button>
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

	.drop {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--miso-sp-08);
		padding: var(--miso-sp-12);
		border: 1px dashed var(--miso-line-primary);
		border-radius: var(--miso-radius-08);
		box-sizing: border-box;
	}

	.hint {
		color: var(--miso-txt-secondary);
	}

	.fp {
		display: inline-block;
		max-width: 320px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		vertical-align: bottom;
	}
</style>
