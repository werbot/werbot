<script lang="ts">
	import { Checkbox, FilledField, Segmented, type SelectOption } from '#lib/ui/index.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import FormField from '#lib/FormField.svelte';
	import LabelsEditor from '#lib/resources/LabelsEditor.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import PickList from './PickList.svelte';
	import type { PolicyDraft } from './policyDraft.js';

	interface Props {
		/** the form element id: the drawer footer submits it */
		id: string;
		draft: PolicyDraft;
		users: SelectOption[];
		teams: SelectOption[];
		resources: SelectOption[];
		durationError: string;
		onsubmit: () => void;
	}

	let { id, draft = $bindable(), users, teams, resources, durationError, onsubmit }: Props = $props();
</script>

<form
	{id}
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	<FilledField label={m.approvals_name()} bind:value={draft.name} required maxlength={63} />

	<fieldset>
		<legend>{m.approvals_resources()}</legend>
		<div>
			<Segmented
				size={32}
				options={[
					{ value: 'resources', label: m.approvals_by_resources() },
					{ value: 'labels', label: m.approvals_by_labels() }
				]}
				bind:value={draft.by}
			/>
		</div>
		{#if draft.by === 'resources'}
			<PickList label={m.approvals_add_resource()} options={resources} bind:ids={draft.resourceIds} />
		{:else}
			<LabelsEditor bind:rows={draft.labels} />
		{/if}
	</fieldset>

	<fieldset>
		<legend>{m.approvals_requesters()}</legend>
		<div class="checks">
			<Checkbox bind:checked={draft.requesters.allUsers}>{m.approvals_all_users()}</Checkbox>
			<Checkbox bind:checked={draft.requesters.allAgents}>{m.approvals_all_agents()}</Checkbox>
		</div>
		{#if !draft.requesters.allUsers}
			<PickList label={m.approvals_add_user()} options={users} bind:ids={draft.requesters.users} />
			<PickList label={m.approvals_add_team()} options={teams} bind:ids={draft.requesters.teams} />
		{/if}
	</fieldset>

	<fieldset>
		<legend>{m.approvals_approvers()}</legend>
		<p class="hint">{m.approvals_approvers_hint()}</p>
		<PickList label={m.approvals_add_user()} options={users} bind:ids={draft.approvers.users} />
		<PickList label={m.approvals_add_team()} options={teams} bind:ids={draft.approvers.teams} />
	</fieldset>

	<fieldset>
		<legend>{m.approvals_mode()}</legend>
		<div>
			<Segmented
				size={32}
				options={[
					{ value: 'manual', label: m.approvals_mode_manual() },
					{ value: 'auto', label: m.approvals_mode_auto() }
				]}
				bind:value={draft.mode}
			/>
		</div>
		<FieldRow even>
			<FormField
				label={m.approvals_max_duration()}
				placeholder="8h"
				hint={m.approvals_max_duration_hint()}
				error={durationError}
				bind:value={draft.maxDuration}
			/>
		</FieldRow>
		<div class="checks">
			<Checkbox bind:checked={draft.requireReason}>{m.approvals_require_reason()}</Checkbox>
			<Checkbox bind:checked={draft.fourEyes}>{m.approvals_four_eyes()}</Checkbox>
			<Checkbox bind:checked={draft.breakGlass}>{m.approvals_break_glass()}</Checkbox>
		</div>
	</fieldset>
</form>

<style>
	form,
	fieldset {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	form {
		gap: var(--miso-sp-24);
	}

	fieldset {
		margin: 0;
		padding: 0;
		border: 0;
		min-width: 0;
	}

	legend {
		padding: 0;
		font: var(--miso-font-ui-13-semi);
		color: var(--miso-txt-primary);
	}

	.checks {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-08);
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
