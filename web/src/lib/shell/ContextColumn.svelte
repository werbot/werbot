<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Icon } from '#lib/ui/index.js';
	import type { NavItem, NavSection } from './nav.js';

	interface Props {
		section: NavSection;
		current?: NavItem;
		header?: Snippet;
	}

	let { section, current, header }: Props = $props();
</script>

<aside class="cc">
	{#if header}
		<div class="cc__header">{@render header()}</div>
	{/if}

	<h2 class="cc__title">{section.label()}</h2>

	<nav class="cc__menu">
		{#each section.items as item (item.href)}
			<a
				class={['cc__item', item.href === current?.href && 'cc__item--active']}
				href={item.href}
				aria-current={item.href === current?.href ? 'page' : undefined}
			>
				<span class="cc__item-ico"><Icon name={item.icon} size={20} /></span>
				<span class="cc__item-in">{item.label()}</span>
			</a>
		{/each}
	</nav>
</aside>

<style>
	/* 260px context column on cont-mid with the right hairline (demo ProjectSidebar .psb) */
	.cc {
		display: flex;
		flex-direction: column;
		min-width: 0;
		background: var(--miso-cont-mid);
		/* a border, not a shadow: the sticky page header must not paint over it */
		border-right: 1px solid var(--miso-line-tertiary);
		overflow: hidden;
	}

	.cc__header {
		display: flex;
		flex-direction: column;
		padding: 24px 20px 0;
	}

	/* .sidebar__head .name{font:ui-22-title-1-bold} */
	.cc__title {
		margin: 0;
		padding: 32px 20px 6px 24px;
		font: var(--miso-font-ui-22-bold);
		color: var(--miso-txt-primary);
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}

	.cc__header + .cc__title {
		padding-top: 16px;
	}

	/* .sidebar__menu */
	.cc__menu {
		display: flex;
		flex-direction: column;
		flex: 1;
		padding: 12px 20px 16px;
		overflow-y: auto;
	}

	/* as tall as the 40px buttons (miso .sidebar__menu__item is 32px) */
	.cc__item {
		display: flex;
		align-items: center;
		height: 40px;
		flex-shrink: 0;
		margin-bottom: var(--miso-sp-04);
		padding: 0 var(--miso-sp-04);
		border-radius: var(--miso-radius-08);
		font: var(--miso-font-ui-14-reg);
		color: var(--miso-txt-primary);
		text-decoration: none;
		transition: 0.3s;
	}

	/* 20px glyph centered in a 32px box */
	.cc__item-ico {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		flex-shrink: 0;
		color: var(--miso-txt-secondary);
		transition: color 0.3s;
	}

	.cc__item-in {
		padding: 0 4px 0 6px;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}

	/* hover: violet text on light-violet pill */
	.cc__item:not(.cc__item--active):hover {
		background: var(--miso-hlt-b-pressed);
		color: var(--miso-txt-btn-b);
	}

	.cc__item:not(.cc__item--active):hover .cc__item-ico {
		color: var(--miso-txt-btn-b);
	}

	/* active: solid brand pill, dark text */
	.cc__item--active {
		background: var(--miso-hlt-b-invert);
		color: var(--wb-on-brand);
	}

	.cc__item--active .cc__item-ico {
		color: var(--wb-on-brand);
	}

	/* desktop from 1024px: below it only the rail stays */
	@media (max-width: 1023.98px) {
		.cc {
			display: none;
		}
	}
</style>
