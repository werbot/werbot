<script lang="ts">
	import { Checkbox, FilledField, Segmented, type SelectOption } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import LabelsEditor from '#lib/resources/LabelsEditor.svelte';
	import PickList from '#lib/requests/PickList.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import ClassPicker from './ClassPicker.svelte';
	import type { RuleDraft } from './ruleDraft.js';

	interface Props {
		/** the form element id: the drawer footer submits it */
		id: string;
		draft: RuleDraft;
		users: SelectOption[];
		teams: SelectOption[];
		resources: SelectOption[];
		/** a class is required: the form shows it after a submit without one */
		classesError: boolean;
		onsubmit: () => void;
	}

	let { id, draft = $bindable(), users, teams, resources, classesError, onsubmit }: Props = $props();
</script>

<form
	{id}
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	<FilledField label={m.policies_commands_name()} bind:value={draft.name} required maxlength={63} />

	<fieldset>
		<legend>{m.policies_commands_subjects()}</legend>
		<div class="checks">
			<Checkbox bind:checked={draft.subjects.allUsers}>{m.policies_commands_all_users()}</Checkbox>
		</div>
		{#if !draft.subjects.allUsers}
			<PickList label={m.approvals_add_user()} options={users} bind:ids={draft.subjects.users} />
			<PickList label={m.approvals_add_team()} options={teams} bind:ids={draft.subjects.teams} />
		{/if}
	</fieldset>

	<fieldset>
		<legend>{m.approvals_resources()}</legend>
		<div>
			<Segmented
				size={32}
				options={[
					{ value: 'resources', label: m.policies_commands_target_resources() },
					{ value: 'labels', label: m.policies_commands_target_labels() }
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
		<legend>{m.policies_commands_classes()}</legend>
		<ClassPicker bind:selected={draft.classes} />
		{#if classesError}<p class="hint hint--error">{m.policies_commands_classes_required()}</p>{/if}
	</fieldset>

	<fieldset>
		<legend>{m.policies_commands_effect()}</legend>
		<div>
			<Segmented
				size={32}
				options={[
					{ value: 'allow', label: m.policies_commands_effect_allow() },
					{ value: 'deny', label: m.policies_commands_effect_deny() },
					{ value: 'approval', label: m.policies_commands_effect_approval() }
				]}
				bind:value={draft.effect}
			/>
		</div>
		{#if draft.effect === 'allow'}<p class="hint">{m.policies_commands_allow_hint()}</p>{/if}
		<FormField label={m.policies_commands_message()} bind:value={draft.message} maxlength={256} />
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

	.hint--error {
		color: var(--miso-txt-alert-major);
	}
</style>
