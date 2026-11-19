<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Breadcrumbs } from '#lib/ui/index.js';
	import type { Crumb } from '#lib/ui/index.js';

	interface Props {
		title: string;
		count?: number;
		breadcrumbs?: Crumb[];
		tabs?: Snippet;
		toolbar?: Snippet;
	}

	let { title, count, breadcrumbs, tabs, toolbar }: Props = $props();
</script>

<!-- work-area header (.headerConfig): crumbs, title + counter, toolbar, tabs -->
<header class="ph">
	{#if breadcrumbs?.length}
		<div class="ph__crumbs"><Breadcrumbs items={breadcrumbs} /></div>
	{/if}
	<div class={['ph__name', breadcrumbs?.length && 'ph__name--crumbs']}>
		<h1 class="ph__title">{title}</h1>
		{#if count !== undefined}
			<span class="ph__count">{count}</span>
		{/if}
		{#if toolbar}
			<div class="ph__toolbar">{@render toolbar()}</div>
		{/if}
	</div>
	{#if tabs}
		<nav class="ph__tabs">{@render tabs()}</nav>
	{:else}
		<div class="ph__tabs ph__tabs--empty"></div>
	{/if}
</header>

<style>
	/* .headerConfig{border-bottom:1px line-secondary}: full width, stays put while the body scrolls */
	.ph {
		position: sticky;
		top: 0;
		z-index: 2;
		background: var(--miso-cont-top);
		border-bottom: 1px solid var(--miso-line-secondary);
	}

	.ph__crumbs {
		padding: 12px 20px 0 26px;
	}

	/* .headerConfig__title-name{padding:2px 20px 8px 22px}, top offset aligns with the column title */
	.ph__name {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 34px 20px 8px 22px;
	}

	.ph__name--crumbs {
		padding-top: 4px;
	}

	/* .headerConfig__title-name>.in{font:ui-22-title-1-bold;padding:0 4px} */
	.ph__title {
		margin: 0;
		padding: 0 4px;
		font: var(--miso-font-ui-22-bold);
		color: var(--miso-txt-primary);
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}

	.ph__count {
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}

	/* toolbar sits on the right: primary action last */
	.ph__toolbar {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-left: auto;
	}

	/* ── tabs (.headerConfig__tabs{padding-left:20px;height:34px}) ── */
	.ph__tabs {
		display: flex;
		height: 34px;
		padding-left: 20px;
		overflow: hidden;
		white-space: nowrap;
		/* live headerConfig flavor of the lib Tabs */
		--miso-tabs-gap: 4px;
		--miso-tabs-height: 34px;
		--miso-tabs-item-pad: 0 6px;
		--miso-tabs-font: var(--miso-font-ui-14-reg);
		--miso-tabs-color: var(--miso-txt-tertiary);
		--miso-tabs-color-hover: var(--miso-txt-secondary);
		--miso-tabs-color-active: var(--miso-txt-accent);
		--miso-tabs-bar: var(--miso-hlt-b-invert);
		--miso-tabs-bar-inset: 0px;
	}

	.ph__tabs--empty {
		height: 8px;
	}
</style>
