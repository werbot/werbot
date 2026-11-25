<script lang="ts" module>
	import type { SegmentedOption } from '#lib/ui/index.js';

	/** an option that allows (green) or denies (red) when it is the active one */
	export type ToneOption = SegmentedOption & { tone?: 'allow' | 'deny' };
</script>

<script lang="ts">
	import type { ComponentProps } from 'svelte';
	import { Segmented } from '#lib/ui/index.js';

	type Props = Omit<ComponentProps<typeof Segmented>, 'options'> & { options: ToneOption[] };

	let { options, value = $bindable(options[0]?.value), ...rest }: Props = $props();

	let tone = $derived(options.find((o) => o.value === value)?.tone);
</script>

<!-- Segmented whose active pill turns green for an allowing option and red for a denying one -->
<span class={['tone', tone && `tone--${tone}`]}>
	<Segmented {options} bind:value {...rest} />
</span>

<style>
	.tone {
		display: inline-flex;
	}

	/* the PriorityBadge colours, as the schedule hours */
	.tone--allow {
		--tone-bg: var(--miso-prior-5);
	}

	.tone--deny {
		--tone-bg: var(--miso-prior-9);
	}

	.tone[class*='tone--'] :global(.miso-segmented__thumb) {
		background: var(--tone-bg);
		box-shadow: none;
	}

	.tone[class*='tone--'] :global(.miso-segmented__item--active) {
		color: var(--miso-prior-txt);
	}
</style>
