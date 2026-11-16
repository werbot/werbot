<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** uppercase caption on the left (ASSIGNEE, STATUS, …) */
		label: string;
		children?: Snippet;
	}

	let { label, children }: Props = $props();
</script>

<div class="miso-property">
	<span class="miso-property__label">{label}</span>
	<span class="miso-property__value">{@render children?.()}</span>
</div>

<style>
	.miso-property {
		display: flex;
		align-items: center;
		gap: 16px;
		min-height: 28px;
	}

	.miso-property__label {
		width: 96px;
		flex-shrink: 0;
		font: var(--miso-font-caption-11-upper);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--miso-txt-tertiary);
	}

	.miso-property__value {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
	}
</style>
