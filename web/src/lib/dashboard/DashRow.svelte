<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Icon, type IconName } from '#lib/ui/index.js';

	interface Props {
		href: string;
		title: string;
		/** tile icon on the left, used when there is no lead */
		icon?: IconName;
		/** dot in the tile corner */
		dot?: Snippet;
		/** custom left part (an avatar) */
		lead?: Snippet;
		/** muted second line */
		meta?: Snippet;
		/** right column */
		trail?: Snippet;
	}

	let { href, title, icon, dot, lead, meta, trail }: Props = $props();
</script>

<li>
	<a class="row" {href}>
		{#if lead}
			{@render lead()}
		{:else if icon}
			<span class="row__tile">
				<Icon name={icon} size={20} />
				{#if dot}<span class="row__dot">{@render dot()}</span>{/if}
			</span>
		{/if}
		<span class="row__info">
			<span class="row__title">{title}</span>
			{#if meta}<span class="row__meta">{@render meta()}</span>{/if}
		</span>
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
	.row__tile {
		position: relative;
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-mid);
		color: var(--miso-txt-secondary);
	}
	.row__dot {
		position: absolute;
		top: -3px;
		right: -3px;
		display: inline-flex;
		border: 2px solid var(--miso-cont-top);
		border-radius: 50%;
	}
	.row__info {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-width: 0;
	}
	.row__title,
	.row__meta {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.row__title {
		font: var(--miso-font-ui-14-semi);
		color: var(--miso-txt-primary);
	}
	/* block, not flex: plain text in the meta line still gets the ellipsis */
	.row__meta {
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
	.row__meta > :global(* + *) {
		margin-left: var(--miso-sp-04);
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
