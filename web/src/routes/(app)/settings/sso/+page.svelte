<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Drawer, EmptyState, Label, MenuItem, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import { identityProviders, teams as teamsApi } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { IdentityProvider } from '#lib/gen/werbot/v1/identity_provider_pb.js';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { selectOpen } from '#lib/selectOpen.js';
	import IdpForm from '#lib/sso/IdpForm.svelte';
	import { fromDraft, toDraft, type IdpDraft } from '#lib/sso/idpDraft.js';
	import MappingDrawer from '#lib/sso/MappingDrawer.svelte';
	import RequireMfaCard from '#lib/sso/RequireMfaCard.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<IdentityProvider[]>([]);
	let teamOptions = $state<{ value: string; label: string }[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<IdentityProvider | null>(null);
	let mapping = $state<IdentityProvider | null>(null);
	// the drawer edits `editing` ('new' for a new provider) through `draft`
	let editing = $state('');
	let draft = $state<IdpDraft>(toDraft());

	const current = $derived(list.find((p) => p.id === editing));

	async function refresh() {
		const [r, t] = await Promise.all([identityProviders.listIdentityProviders({}), teamsApi.listTeams({})]);
		list = r.identityProviders;
		teamOptions = t.teams.map((x) => ({ value: x.id, label: x.name }));
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function edit(p?: IdentityProvider) {
		draft = toDraft(p);
		editing = p?.id ?? 'new';
	}

	async function save() {
		const input = fromDraft(draft);
		try {
			if (editing === 'new') {
				// stay open on the saved provider: its redirect URI is what the admin registers next
				editing = (await identityProviders.createIdentityProvider(input)).identityProvider?.id ?? '';
			} else {
				await identityProviders.updateIdentityProvider({ ...input, id: editing });
				editing = '';
			}
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			failToast(e);
		}
		await load();
		if (editing && editing !== 'new') draft = toDraft(current);
	}

	async function remove() {
		const p = removing;
		removing = null;
		if (!p) return;
		try {
			await identityProviders.deleteIdentityProvider({ id: p.id });
			toast({ tone: 'success', text: m.common_deleted() });
			if (editing === p.id) editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(load);
</script>

<PageHeader title={m.sso_title()}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.sso_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody width={960}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else if list.length === 0}
		<EmptyState icon="key" title={m.sso_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.sso_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.sso_name()}</th>
				<th>{m.sso_issuer()}</th>
				<th>{m.sso_status()}</th>
				<th></th>
			{/snippet}
			{#each list as p (p.id)}
				<tr>
					<td>
						<CellLead icon="key">
							<button class="row-name" onclick={() => edit(p)}>{p.name}</button>
							{#snippet sub()}{p.clientId}{/snippet}
						</CellLead>
					</td>
					<td class="muted">{p.issuer}</td>
					<td><Label variant="soft">{p.enabled ? m.sso_enabled() : m.sso_disabled()}</Label></td>
					<td class="more">
						<RowMenu bind:open={() => menuOpen === p.id, (v) => (menuOpen = v ? p.id : '')}>
							{#snippet items(close)}
								<MenuItem icon="people" onclick={() => (close(), (mapping = p))}>{m.sso_mapping()}</MenuItem>
								<MenuItem icon="trash" danger onclick={() => (close(), (removing = p))}>{m.sso_delete()}</MenuItem>
							{/snippet}
						</RowMenu>
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
	<RequireMfaCard />
</PageBody>

<!-- Escape reaches the drawer too: ignore it while a select popup, the delete confirm or a menu is open -->
<Drawer
	bind:open={() => editing !== '', (v) => !v && !selectOpen() && !removing && !menuOpen && (editing = '')}
	resizable
	snapPoints={[560, 720]}
	width={560}
	title={current?.name ?? m.sso_new()}
>
	{#if editing}
		<IdpForm id="idp" bind:draft redirectUri={current?.redirectUri ?? ''} existing={!!current} onsubmit={save} />
	{/if}
	{#snippet footer()}
		{#if current}
			<Button size={40} scheme="w" variant="solid" iconLeft="trash" onclick={() => (removing = current ?? null)}>{m.sso_delete()}</Button>
		{/if}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (editing = '')}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="idp" variant="solid">{current ? m.settings_save() : m.sso_create()}</Button>
	{/snippet}
</Drawer>

<MappingDrawer idp={mapping} teams={teamOptions} onclose={() => (mapping = null)} onsaved={load} />

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.sso_delete()}>
	<p>{m.sso_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.sso_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.more {
		text-align: right;
	}

	.muted {
		color: var(--miso-txt-secondary);
	}

	.grow {
		flex: 1;
	}
</style>
