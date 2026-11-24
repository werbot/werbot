<script lang="ts">
	import { Segmented, type SelectOption } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import SubjectsTarget from '#lib/policies/SubjectsTarget.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { maxAgeInvalid, type MfaRuleDraft } from './ruleDraft.js';

	interface Props {
		/** the form element id: the drawer footer submits it */
		id: string;
		draft: MfaRuleDraft;
		users: SelectOption[];
		teams: SelectOption[];
		resources: SelectOption[];
		/** show the max age error: set after a submit with a bad value */
		submitted: boolean;
		onsubmit: () => void;
	}

	let { id, draft = $bindable(), users, teams, resources, submitted, onsubmit }: Props = $props();
</script>

<form
	{id}
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	<FormField label={m.mfa_rules_name()} bind:value={draft.name} required maxlength={63} />

	<SubjectsTarget bind:draft {users} {teams} {resources} />

	<fieldset>
		<legend>{m.mfa_rules_max_age()}</legend>
		<FormField label={m.mfa_rules_max_age()} hint={m.mfa_rules_max_age_hint()} error={submitted && maxAgeInvalid(draft.maxAge) ? m.mfa_rules_max_age_invalid() : ''} bind:value={draft.maxAge} required />
	</fieldset>

	<fieldset>
		<legend>{m.mfa_rules_strength()}</legend>
		<div>
			<Segmented
				size={32}
				options={[
					{ value: 'any', label: m.mfa_rules_strength_any() },
					{ value: 'webauthn', label: m.mfa_rules_strength_webauthn() }
				]}
				bind:value={draft.strength}
			/>
		</div>
		<p class="hint">{draft.strength === 'webauthn' ? m.mfa_rules_strength_webauthn_hint() : m.mfa_rules_strength_any_hint()}</p>
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
</style>
