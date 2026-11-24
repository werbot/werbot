<script lang="ts">
	import { Button, Segmented, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import FormField from '#lib/FormField.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { mappingInvalid, type MappingRow } from './idpDraft.js';

	interface Props {
		rows: MappingRow[];
		teams: SelectOption[];
		/** show a row's error: set after a submit with a bad row */
		submitted: boolean;
	}

	let { rows = $bindable(), teams, submitted }: Props = $props();

	const roles = [
		{ value: 'member', label: m.sso_role_member() },
		{ value: 'admin', label: m.sso_role_admin() }
	];
	const kinds = [
		{ value: 'team', label: m.sso_mapping_team() },
		{ value: 'role', label: m.sso_mapping_role() }
	];

	// the target is a team id or a role name: a changed kind drops the other kind's value
	const setKind = (row: MappingRow, kind: string) => {
		row.kind = kind;
		row.target = '';
	};
</script>

<div class="mapping">
	<div class="head">
		<span>{m.sso_mapping()}</span>
		<Button type="button" variant="plain" size={28} onclick={() => rows.push({ claim: 'groups', regex: '', kind: 'team', target: '' })}>{m.sso_mapping_add()}</Button>
	</div>
	<p class="hint">{m.sso_mapping_hint()}</p>
	{#each rows as row, i (i)}
		<div class="row">
			<div class="pair">
				<FormField label={m.sso_mapping_claim()} bind:value={row.claim} />
				<FormField label={m.sso_mapping_regex()} bind:value={row.regex} placeholder="^/ops$" error={submitted && mappingInvalid(row) ? m.sso_mapping_invalid() : ''} />
			</div>
			<div class="pair">
				<Segmented size={32} options={kinds} value={row.kind} onchange={(v) => setKind(row, v)} />
				<FilledSelect label={row.kind === 'team' ? m.sso_mapping_team() : m.sso_mapping_role()} options={row.kind === 'team' ? teams : roles} bind:value={row.target} />
			</div>
			<Button type="button" scheme="w" variant="plain" size={28} iconLeft="trash" onclick={() => rows.splice(i, 1)}>{m.sso_mapping_remove()}</Button>
		</div>
	{/each}
</div>

<style>
	.mapping {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-08);
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font: var(--miso-font-ui-13-semi);
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.row {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: var(--miso-sp-08);
		padding: var(--miso-sp-12);
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-mid);
	}

	.pair {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: var(--miso-sp-12);
		align-items: start;
		align-self: stretch;
	}
</style>
