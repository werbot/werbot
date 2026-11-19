<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Banner, Button, EmptyState, InlineEdit, Modal, Skeleton, Table, FilledField, Textarea, Tooltip, toast } from '#lib/ui/index.js';
	import KeyFileDrop from '#lib/KeyFileDrop.svelte';
	import { readPubKeyFile } from '#lib/sshkey.js';
	import RowMenu from '#lib/RowMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
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
	let error = $state('');
	let adding = $state(false);
	let name = $state('');
	let publicKey = $state('');
	let removing = $state<SSHKey | null>(null);
	let menuOpen = $state('');

	async function refresh() {
		keys = (await sshKeys.listSSHKeys({})).keys;
		loaded = true;
	}

	async function run(action: () => Promise<unknown>, message = m.common_saved()) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: message });
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

	async function readKeyFile(file: File) {
		error = '';
		try {
			const parsed = await readPubKeyFile(file);
			if ('error' in parsed) {
				error = parsed.error === 'too_large' ? m.keys_file_too_large() : m.keys_file_private_key();
				return;
			}
			publicKey = autoKey = parsed.key;
			fileName = file.name;
			if (!name.trim() || name === autoName) name = autoName = parsed.name;
		} catch (e) {
			error = errorMessage(e);
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
		if (key) await run(() => sshKeys.deleteSSHKey({ id: key.id }), m.common_deleted());
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
<PageBody flush>
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
					<td class="more">
						<RowMenu label={m.keys_delete()} onselect={() => (removing = key)} bind:open={() => menuOpen === key.id, (v) => (menuOpen = v ? key.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<Modal bind:open={() => adding, setAdding} title={m.keys_add()} width={480}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<form id="add-key" onsubmit={add}>
		<KeyFileDrop hint={m.keys_drop_hint()} {fileName} accept=".pub" onfile={readKeyFile} onclear={clearFile} />
		<FilledField label={m.keys_name()} bind:value={name} />
		<Textarea label={m.keys_public_key()} bind:value={publicKey} required rows={5} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => setAdding(false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="add-key" variant="solid">{m.keys_add()}</Button>
	{/snippet}
</Modal>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.keys_delete()}>
	<p>{m.keys_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.keys_delete()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.more {
		text-align: right;
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
