<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { IconName } from '#lib/ui/index.js';
	import CellLead from '#lib/CellLead.svelte';

	interface Props {
		href: string;
		title: string;
		/** tile icon on the left, used when there is no avatar */
		icon?: IconName;
		/** dot in the tile corner */
		dot?: Snippet;
		/** a person's login: their avatar instead of the tile; empty falls back to the tile */
		avatar?: string;
		/** presence dot on the avatar */
		online?: boolean;
		/** muted second line */
		meta?: Snippet;
		/** right column */
		trail?: Snippet;
	}

	let { href, title, icon, dot, avatar, online, meta, trail }: Props = $props();
</script>

<li>
	<a class="row" {href}>
		<CellLead {icon} {dot} {avatar} {online} size={36} sub={meta}>{title}</CellLead>
		{#if trail}<span class="row__trail">{@render trail()}</span>{/if}
	</a>
</li>

<style>
	/* adapted from the miso-ui demo dashboard .dbrd__project-row */
	li {
		list-style: none;
	}
	.row {
		display: flex;
		align-items: center;
		gap: var(--miso-sp-12);
		min-height: 52px;
		padding: var(--miso-sp-08) var(--miso-sp-12) var(--miso-sp-08) var(--miso-sp-16);
		box-sizing: border-box;
		color: inherit;
		text-decoration: none;
	}
	.row:hover {
		background: var(--miso-hlt-g-hover);
	}
	.row:focus-visible {
		outline: 2px solid var(--wb-focus);
		outline-offset: -2px;
	}
	.row__trail {
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
		align-items: flex-end;
		gap: var(--miso-sp-02);
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
