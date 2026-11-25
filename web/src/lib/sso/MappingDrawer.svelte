<script lang="ts">
	import { Button, Drawer, toast, type SelectOption } from '#lib/ui/index.js';
	import { identityProviders } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { IdentityProvider } from '#lib/gen/werbot/v1/identity_provider_pb.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';
	import ClaimsMappingEditor from './ClaimsMappingEditor.svelte';
	import { fromDraft, mappingInvalid, toDraft, type MappingRow } from './idpDraft.js';

	interface Props {
		/** the provider whose claim mapping is edited; null closes the drawer */
		idp: IdentityProvider | null;
		teams: SelectOption[];
		onclose: () => void;
		onsaved: () => void;
	}

	let { idp, teams, onclose, onsaved }: Props = $props();

	const shown = keepLast(() => idp);
	let rows = $state<MappingRow[]>([]);
	let submitted = $state(false);

	// another provider: its rules fill the editor
	$effect(() => {
		rows = toDraft(idp ?? undefined).mapping;
		submitted = false;
	});

	async function save(e: SubmitEvent) {
		e.preventDefault();
		submitted = true;
		if (!idp || rows.some(mappingInvalid)) return;
		try {
			// the other fields as stored; an empty secret keeps the stored one
			await identityProviders.updateIdentityProvider({ ...fromDraft({ ...toDraft(idp), mapping: rows }), id: idp.id });
			toast({ tone: 'success', text: m.common_saved() });
			onsaved();
			onclose();
		} catch (err) {
			failToast(err);
		}
	}
</script>

<!-- Escape reaches the drawer too: ignore it while a select popup is open -->
<Drawer
	bind:open={() => !!idp, (v) => !v && !selectOpen() && onclose()}
	resizable
	snapPoints={[560, 720]}
	width={560}
	title={`${m.sso_mapping()}: ${shown.value?.name ?? ''}`}
>
	<form id="idp-mapping" onsubmit={save}>
		<ClaimsMappingEditor bind:rows {teams} {submitted} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={onclose}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="idp-mapping" variant="solid">{m.settings_save()}</Button>
	{/snippet}
</Drawer>
