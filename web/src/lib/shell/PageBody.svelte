<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** max content width in px, centered (demo sections: 680, wide ones 960) */
		width?: number;
		/** list page: the table runs to the work-area edges (demo project) */
		flush?: boolean;
		children: Snippet;
	}

	let { width, flush = false, children }: Props = $props();
</script>

<!-- work-area body under PageHeader -->
<div class={['pb', flush && 'pb--flush']} style:max-width={width ? `${width}px` : undefined}>
	{@render children()}
</div>

<style>
	/* demo administration sections (.sec, .api, .tc): column, gap 24, padding 24, centered */
	.pb {
		display: flex;
		flex-direction: column;
		gap: 24px;
		margin: 0 auto;
		padding: 24px;
	}

	/* demo project: banners and skeletons keep the toolbar inset (.prj__toolbar 8px 16px) */
	.pb--flush {
		gap: 16px;
		padding: 16px 20px;
	}

	/* the list table bleeds to both work-area edges */
	.pb--flush > :global(.miso-table) {
		width: calc(100% + 40px);
		margin: 0 -20px;
	}

	.pb--flush > :global(.miso-table:first-child) {
		margin-top: -16px;
	}

	/* .tt__td--name{padding-left:20px}: edge cells keep the demo inset */
	.pb--flush > :global(.miso-table) :global(:is(th, td):first-child) {
		padding-left: 20px;
	}

	.pb--flush > :global(.miso-table) :global(:is(th, td):last-child) {
		padding-right: 20px;
	}
</style>
