<script lang="ts">
	import { Badge, Button, FilledField, Textarea, toast } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import { resources } from '#lib/api.js';
	import { errorMessage, failToast } from '#lib/errors.js';
	import KeyFileDrop from '#lib/KeyFileDrop.svelte';
	import { readPrivateKeyFile } from '#lib/sshkey.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let { resource = $bindable(), type }: { resource: Resource; type?: ResourceType } = $props();

	const labels: Record<string, () => string> = { password: m.cred_kind_password, private_key: m.cred_kind_private_key, client_cert: m.cred_kind_client_cert };
	const setLabels: Record<string, () => string> = { password: m.cred_set_password, private_key: m.cred_set_private_key, client_cert: m.cred_set_client_cert };
	const label = (kind: string) => (labels[kind] ?? (() => kind))();

	let picked = $state('');
	const kind = $derived(picked || type?.credentialKinds[0] || '');
	let value = $state('');
	let fileName = $state('');
	let fileValue = '';
	let fileError = $state('');

	async function run(action: () => Promise<unknown>) {
		try {
			await action();
			value = fileName = fileValue = fileError = '';
			const res = await resources.getResource({ id: resource.id });
			if (res.resource) resource = res.resource;
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			failToast(e);
		}
	}

	async function readKeyFile(file: File) {
		fileError = '';
		try {
			const read = await readPrivateKeyFile(file);
			if ('error' in read) {
				fileError = m.cred_file_too_large();
				return;
			}
			value = fileValue = read.key;
			fileName = file.name;
		} catch (e) {
			fileError = errorMessage(e);
		}
	}

	// the chip removes only what the file filled in, never text the user edited
	function clearFile() {
		if (value === fileValue) value = '';
		fileName = fileValue = '';
	}

	const save = (e: SubmitEvent) => {
		e.preventDefault();
		return run(() => resources.setCredential({ resourceId: resource.id, kind, value: new TextEncoder().encode(value) }));
	};
</script>

<p>
	<Badge tone={resource.credentialKind ? 'accent' : 'ghost'}>
		{resource.credentialKind ? (setLabels[resource.credentialKind] ?? (() => resource.credentialKind))() : m.cred_not_set()}
	</Badge>
	{m.cred_write_only()}
</p>
<form onsubmit={save}>
	<FilledSelect
		label={m.cred_kind()}
		options={(type?.credentialKinds ?? []).map((k) => ({ value: k, label: label(k) }))}
		value={kind}
		onchange={(v) => ((picked = v), (fileError = ''))}
	/>
	{#if kind === 'private_key' || kind === 'client_cert'}
		<KeyFileDrop hint={kind === 'client_cert' ? m.cred_client_cert_hint() : m.cred_drop_hint()} {fileName} error={fileError} onfile={readKeyFile} onclear={clearFile} />
		<Textarea label={label(kind)} bind:value rows={8} autocomplete="off" spellcheck="false" required />
	{:else}
		<FilledField type="password" label={label(kind)} bind:value autocomplete="new-password" required />
	{/if}
	<div class="wb-actions">
		<Button size={40} type="submit" variant="solid">{m.cred_save()}</Button>
		<Button size={40}
			type="button"
			scheme="w"
			variant="plain"
			disabled={!resource.credentialKind}
			onclick={() => run(() => resources.clearCredential({ resourceId: resource.id }))}
		>
			{m.cred_clear()}
		</Button>
	</div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		max-width: 480px;
	}
</style>
