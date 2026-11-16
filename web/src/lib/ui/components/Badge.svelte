<script lang="ts" module>
	export type BadgeTone = 'alert' | 'base' | 'upcoming' | 'ghost' | 'accent';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		tone?: BadgeTone;
		children?: Snippet;
	}

	let { tone = 'base', children }: Props = $props();
</script>

<span class={['miso-badge', `miso-badge--${tone}`]}>
	{@render children?.()}
</span>

<style>
	.miso-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 18px;
		height: 16px;
		padding: 0 5px;
		box-sizing: border-box;
		border-radius: var(--miso-radius-full);
		font: var(--miso-font-ui-10-bold);
		color: var(--miso-sidebar-badge-txt);
		vertical-align: middle;
	}

	.miso-badge--alert { background: var(--miso-sidebar-badge-alert); }
	.miso-badge--base { background: var(--miso-sidebar-badge-base); }
	.miso-badge--upcoming { background: var(--miso-sidebar-badge-upcoming); }
	.miso-badge--ghost { background: var(--miso-sidebar-badge-ghost); }
	.miso-badge--accent { background: var(--miso-hlt-invert); }
</style>
