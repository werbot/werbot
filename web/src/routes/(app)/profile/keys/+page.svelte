<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, EmptyState, InlineEdit, Label, Modal, Skeleton, Table, FilledField, Textarea, Tooltip, toast } from '#lib/ui/index.js';
	import KeyFileDrop from '#lib/KeyFileDrop.svelte';
	import { readPubKeyFile } from '#lib/sshkey.js';
	import RowMenu from '#lib/RowMenu.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { sshKeys } from '#lib/api.js';
	import { errorMessage, failToast } from '#lib/errors.js';
	import type { SSHKey } from '#lib/gen/werbot/v1/ssh_key_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let keys = $state<SSHKey[]>([]);
	let loaded = $state(false);
	let fileName = $state('');
	let autoName = '';
	let autoKey = '';
	let failed = $state(false);
	let fileError = $state('');
	let adding = $state(false);
	let name = $state('');
	let publicKey = $state('');
	let removing = $state<SSHKey | null>(null);
	let menuOpen = $state('');

	async function refresh() {
		keys = (await sshKeys.listSSHKeys({})).keys;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	async function run(action: () => Promise<unknown>, message = m.common_saved()) {
		try {
			await action();
			toast({ tone: 'success', text: message });
		} catch (e) {
			failToast(e);
			await refresh().catch(() => {});
			return false;
		}
		await load();
		return true;
	}

	async function add(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => sshKeys.addSSHKey({ name, publicKey }))) {
			setAdding(false);
		}
	}

	async function readKeyFile(file: File) {
		fileError = '';
		try {
			const parsed = await readPubKeyFile(file);
			if ('error' in parsed) {
				fileError = parsed.error === 'too_large' ? m.keys_file_too_large() : m.keys_file_private_key();
				return;
			}
			publicKey = autoKey = parsed.key;
			fileName = file.name;
			if (!name.trim() || name === autoName) name = autoName = parsed.name;
		} catch (e) {
			fileError = errorMessage(e);
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
			name = publicKey = fileName = autoName = autoKey = fileError = '';
		}
	}

	async function remove() {
		const key = removing;
		removing = null;
		if (key) await run(() => sshKeys.deleteSSHKey({ id: key.id }), m.common_deleted());
	}

	onMount(load);
</script>

<PageHeader title={m.keys_title()} count={loaded ? keys.length : undefined}>
	{#snippet toolbar()}
		{#if keys.length > 0}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (adding = true)}>{m.keys_add()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
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
				<th>{m.keys_created()}</th>
				<th></th>
			{/snippet}
			{#each keys as key (key.id)}
				<tr>
					<td class="name">
						<CellLead icon="key">
							<InlineEdit value={key.name} placeholder={m.keys_rename()} onsave={(value) => run(() => sshKeys.updateSSHKey({ id: key.id, name: value }))} />
							{#snippet sub()}
								<Label variant="soft" size={16}>{key.publicKey.split(' ')[0].replace(/^ssh-/, '')}</Label>
								<Tooltip text={key.fingerprintSha256}><code class="fp">{key.fingerprintSha256}</code></Tooltip>
							{/snippet}
						</CellLead>
					</td>
					<td class="muted"><Ago ts={key.createdAt} /></td>
					<td class="more">
						<RowMenu label={m.keys_delete()} onselect={() => (removing = key)} bind:open={() => menuOpen === key.id, (v) => (menuOpen = v ? key.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<Modal bind:open={() => adding, setAdding} title={m.keys_add()} width={480}>
	<form id="add-key" onsubmit={add}>
		<KeyFileDrop hint={m.keys_drop_hint()} {fileName} accept=".pub" error={fileError} onfile={readKeyFile} onclear={clearFile} />
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

	/* the editable name reads as the lead title and lines up with the second line */
	.name :global(.miso-inline-edit) {
		margin-left: -4px;
	}

	.name :global(.miso-inline-edit--text .miso-inline-edit__view),
	.name :global(.miso-inline-edit--text .miso-inline-edit__input) {
		height: 22px;
		padding: 0 4px;
		font: var(--miso-font-ui-14-semi);
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
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
