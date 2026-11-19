<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Avatar, Icon, type IconName } from '#lib/ui/index.js';
	import { isMe } from '#lib/me.js';

	interface Props {
		/** icon tile on the left, used when there is no avatar */
		icon?: IconName;
		/** a person's login: their avatar instead of the tile; empty falls back to the tile */
		avatar?: string;
		/** presence dot on the avatar */
		online?: boolean;
		/** dot in the tile corner */
		dot?: Snippet;
		/** tile and avatar size in px */
		size?: number;
		/** the title: text, a link or a button */
		children: Snippet;
		/** muted second line */
		sub?: Snippet;
	}

	let { icon, avatar, online = false, dot, size = 32, children, sub }: Props = $props();
</script>

<span class={['lead', isMe(avatar) && 'wb-me']}>
	{#if avatar}
		<Avatar name={avatar} {size} {online} />
	{:else if icon}
		<span class="lead__tile" style:--size="{size}px">
			<Icon name={icon} size={20} />
			{#if dot}<span class="lead__dot">{@render dot()}</span>{/if}
		</span>
	{/if}
	<span class="lead__info">
		<span class="lead__title">{@render children()}</span>
		{#if sub}<span class="lead__sub">{@render sub()}</span>{/if}
	</span>
</span>

<style>
	/* adapted from the miso-ui demo dashboard .dbrd__project-row and administration .mem__who */
	.lead {
		display: flex;
		flex: 1;
		align-items: center;
		gap: var(--miso-sp-12);
		min-width: 0;
	}
	.lead__tile {
		position: relative;
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		width: var(--size);
		height: var(--size);
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-mid);
		color: var(--miso-txt-secondary);
	}
	.lead__dot {
		position: absolute;
		top: -3px;
		right: -3px;
		display: inline-flex;
		border: 2px solid var(--miso-cont-top);
		border-radius: 50%;
	}
	.lead__info {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-width: 0;
	}
	.lead__title,
	.lead__sub {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	/* the padding keeps a focused link's outline inside the clip box */
	.lead__title {
		padding: 4px;
		margin: -4px;
		font: var(--miso-font-ui-14-semi);
		color: var(--miso-txt-primary);
	}
	/* block, not flex: plain text in the second line still gets the ellipsis */
	.lead__sub {
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
	.lead__sub > :global(* + *) {
		margin-left: var(--miso-sp-04);
	}
</style>
