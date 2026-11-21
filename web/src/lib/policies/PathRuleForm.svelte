<script lang="ts">
	import { Button, FilledField, Segmented, type SelectOption } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import SubjectsTarget from './SubjectsTarget.svelte';
	import { ACCESS_LABELS, applyTemplate, EFFECT_LABELS, PATH_TEMPLATES, parsePaths, type PathDraft } from './paths.js';

	interface Props {
		/** the form element id: the drawer footer submits it */
		id: string;
		draft: PathDraft;
		users: SelectOption[];
		teams: SelectOption[];
		resources: SelectOption[];
		onsubmit: () => void;
	}

	let { id, draft = $bindable(), users, teams, resources, onsubmit }: Props = $props();

	const templates = {
		work_folder: m.policies_paths_template_work_folder,
		deny_folder: m.policies_paths_template_deny_folder,
		read_logs: m.policies_paths_template_read_logs
	};
	const options = (labels: Record<string, () => string>) => Object.entries(labels).map(([value, label]) => ({ value, label: label() }));

	const hasHome = $derived(parsePaths(draft.paths).some((p) => p.startsWith('~')));
</script>

<form
	{id}
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	<div class="templates">
		{#each PATH_TEMPLATES as t (t.key)}
			<Button size={40} scheme="g" variant="plain" type="button" onclick={() => (draft = applyTemplate(draft, t))}>{templates[t.key]()}</Button>
		{/each}
	</div>

	<FilledField label={m.policies_paths_name()} bind:value={draft.name} required maxlength={63} />

	<SubjectsTarget bind:draft {users} {teams} {resources} />

	<fieldset>
		<legend>{m.policies_paths_paths()}</legend>
		<FormField label={m.policies_paths_paths()} bind:value={draft.paths} multiline rows={4} required placeholder="~/app/**" hint={m.policies_paths_glob_hint()} />
		{#if hasHome}<p class="hint">{m.policies_paths_home_hint()}</p>{/if}
	</fieldset>

	<fieldset>
		<legend>{m.policies_paths_access()}</legend>
		<div><Segmented size={32} options={options(ACCESS_LABELS)} bind:value={draft.access} /></div>
	</fieldset>

	<fieldset>
		<legend>{m.policies_paths_effect()}</legend>
		<div><Segmented size={32} options={options(EFFECT_LABELS)} bind:value={draft.effect} /></div>
		{#if draft.effect === 'only'}<p class="hint">{m.policies_paths_only_hint()}</p>{/if}
		<FormField label={m.policies_paths_message()} bind:value={draft.message} maxlength={256} />
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

	.templates {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-08);
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
