<script lang="ts">
	import { Checkbox, Segmented, type SelectOption } from '#lib/ui/index.js';
	import LabelsEditor from '#lib/resources/LabelsEditor.svelte';
	import PickList from '#lib/requests/PickList.svelte';
	import type { SelectorDraft, SubjectsDraft } from '#lib/resources/selectorDraft.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		draft: SelectorDraft & { subjects: SubjectsDraft };
		users: SelectOption[];
		teams: SelectOption[];
		resources: SelectOption[];
	}

	let { draft = $bindable(), users, teams, resources }: Props = $props();
</script>

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

<style>
	fieldset {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
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
</style>
