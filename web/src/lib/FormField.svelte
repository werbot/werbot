<script lang="ts">
	import { FilledField } from '#lib/ui/index.js';
	import type { ComponentProps } from 'svelte';

	type Props = Omit<ComponentProps<typeof FilledField>, 'error'> & { hint?: string; error?: string };

	let { value = $bindable(''), hint, error = '', ...rest }: Props = $props();

	const id = $props.id();
	const note = $derived(error || hint);
</script>

<div class="field">
	<FilledField bind:value error={!!error} aria-invalid={!!error || undefined} aria-describedby={note ? id : undefined} {...rest} />
	{#if note}<p {id} class={['note', error && 'note--error']}>{note}</p>{/if}
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-04);
		min-width: 0;
	}

	/* the floating label sits where the placeholder is: show the example only once focus lifts the label */
	.field :global(.miso-filled :is(input, textarea):not(:focus)::placeholder) {
		opacity: 0;
	}

	.note {
		margin: 0;
		padding: 0 var(--miso-sp-04);
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.note--error {
		color: var(--miso-txt-alert-major);
	}
</style>
