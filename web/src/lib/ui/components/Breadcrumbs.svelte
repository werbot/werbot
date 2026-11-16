<script lang="ts" module>
	export interface Crumb {
		label: string;
		href?: string;
	}
</script>

<script lang="ts">
	interface Props {
		items: Crumb[];
	}

	let { items }: Props = $props();
</script>

<nav class="miso-breadcrumbs" aria-label="Breadcrumb">
	{#each items as item, index (item.label + index)}
		{#if index > 0}
			<span class="miso-breadcrumbs__sep" aria-hidden="true">/</span>
		{/if}
		{#if item.href && index < items.length - 1}
			<a class="miso-breadcrumbs__link" href={item.href}>{item.label}</a>
		{:else}
			<span class="miso-breadcrumbs__current" aria-current="page">{item.label}</span>
		{/if}
	{/each}
</nav>

<style>
	.miso-breadcrumbs {
		display: flex;
		align-items: center;
		gap: 6px;
		font: var(--miso-font-ui-12-reg);
		min-width: 0;
	}

	.miso-breadcrumbs__link {
		color: var(--miso-txt-tertiary);
		text-decoration: none;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.miso-breadcrumbs__link:hover {
		color: var(--miso-txt-link);
	}

	.miso-breadcrumbs__sep {
		color: var(--miso-txt-ghost);
	}

	.miso-breadcrumbs__current {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
