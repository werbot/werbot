<script lang="ts">
	import { FilledField, type SelectOption } from '#lib/ui/index.js';
	import ToneSegmented from '#lib/ToneSegmented.svelte';
	import FormField from '#lib/FormField.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import ClassPicker from './ClassPicker.svelte';
	import SubjectsTarget from './SubjectsTarget.svelte';
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

	<SubjectsTarget bind:draft {users} {teams} {resources} />

	<fieldset>
		<legend>{m.policies_commands_classes()}</legend>
		<ClassPicker bind:selected={draft.classes} />
		{#if classesError}<p class="hint hint--error">{m.policies_commands_classes_required()}</p>{/if}
	</fieldset>

	<fieldset>
		<legend>{m.policies_commands_effect()}</legend>
		<div>
			<ToneSegmented
				size={32}
				options={[
					{ value: 'allow', label: m.policies_commands_effect_allow(), tone: 'allow' },
					{ value: 'deny', label: m.policies_commands_effect_deny(), tone: 'deny' },
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

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.hint--error {
		color: var(--miso-txt-alert-major);
	}
</style>
