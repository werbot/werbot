<script lang="ts">
	import { Badge, Banner, Button, Select, TextField, Textarea, toast } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import KeyFileDrop from '#lib/KeyFileDrop.svelte';
	import { readPrivateKeyFile } from '#lib/sshkey.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let { resource = $bindable(), type }: { resource: Resource; type?: ResourceType } = $props();

	const labels: Record<string, () => string> = { password: m.cred_kind_password, private_key: m.cred_kind_private_key };
	const setLabels: Record<string, () => string> = { password: m.cred_set_password, private_key: m.cred_set_private_key };
	const label = (kind: string) => (labels[kind] ?? (() => kind))();

	let picked = $state('');
	const kind = $derived(picked || type?.credentialKinds[0] || '');
	let value = $state('');
	let fileName = $state('');
	let error = $state('');

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			value = fileName = '';
			const res = await resources.getResource({ id: resource.id });
			if (res.resource) resource = res.resource;
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			error = errorMessage(e);
		}
	}

	async function readKeyFile(file: File) {
		error = '';
		try {
			const read = await readPrivateKeyFile(file);
			if ('error' in read) {
				error = m.cred_file_too_large();
				return;
			}
			value = read.key;
			fileName = file.name;
		} catch (e) {
			error = errorMessage(e);
		}
	}

	const save = (e: SubmitEvent) => {
		e.preventDefault();
		return run(() => resources.setCredential({ resourceId: resource.id, kind, value: new TextEncoder().encode(value) }));
	};
</script>

{#if error}<Banner tone="alert">{error}</Banner>{/if}
<p>
	<Badge tone={resource.credentialKind ? 'accent' : 'ghost'}>
		{resource.credentialKind ? (setLabels[resource.credentialKind] ?? (() => resource.credentialKind))() : m.cred_not_set()}
	</Badge>
	{m.cred_write_only()}
</p>
<form onsubmit={save}>
	<Select
		label={m.cred_kind()}
		options={(type?.credentialKinds ?? []).map((k) => ({ value: k, label: label(k) }))}
		value={kind}
		onchange={(v) => (picked = v)}
	/>
	{#if kind === 'private_key'}
		<KeyFileDrop hint={m.cred_drop_hint()} {fileName} onfile={readKeyFile} onclear={() => (value = fileName = '')} />
		<Textarea label={label(kind)} bind:value rows={8} autocomplete="off" required />
	{:else}
		<TextField type="password" label={label(kind)} bind:value autocomplete="new-password" required />
	{/if}
	<div class="actions">
		<Button type="submit" variant="solid">{m.cred_save()}</Button>
		<Button
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
		gap: 12px;
		max-width: 480px;
	}

	.actions {
		display: flex;
		gap: 8px;
	}
</style>
