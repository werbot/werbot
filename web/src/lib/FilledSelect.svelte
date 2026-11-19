<script lang="ts">
	import { Icon, type SelectOption } from '#lib/ui/index.js';
	import FilledShell from '#lib/FilledShell.svelte';

	interface Props {
		label: string;
		options: SelectOption[];
		value?: string;
		disabled?: boolean;
		onchange?: (value: string) => void;
	}

	let { label, options, value = $bindable(''), disabled = false, onchange }: Props = $props();

	let known = $derived(options.some((o) => o.value === value));
</script>

<FilledShell {label} {disabled}>
	{#snippet children(id)}
		<select {id} bind:value {disabled} onchange={() => onchange?.(value)}>
			{#if !known}<option value="" disabled hidden></option>{/if}
			{#each options as option (option.value)}
				<option value={option.value}>{option.label}</option>
			{/each}
		</select>
		<span class="chevron"><Icon name="chevron-down" size={16} /></span>
	{/snippet}
</FilledShell>

<style>
	select {
		cursor: pointer;
	}

	select:disabled {
		cursor: default;
	}

	.chevron {
		position: absolute;
		right: 14px;
		display: inline-flex;
		color: var(--miso-txt-secondary);
		pointer-events: none;
	}
</style>
