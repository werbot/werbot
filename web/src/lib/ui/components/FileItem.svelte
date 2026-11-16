<script lang="ts">
	import type { IconName } from '../icons.js';
	import Icon from './Icon.svelte';
	import Button from './Button.svelte';

	interface Props {
		name: string;
		/** caption below the name, e.g. size or date */
		meta?: string;
		/** overrides the extension-based icon */
		icon?: IconName;
		selected?: boolean;
		/** card width in px; archive default is 204 */
		width?: number;
		onclick?: () => void;
		ondownload?: () => void;
		ondelete?: () => void;
	}

	let { name, meta, icon, selected = false, width = 204, onclick, ondownload, ondelete }: Props =
		$props();

	const typeIcons: Record<string, { icon: IconName; color: string }> = {
		png: { icon: 'image', color: 'var(--miso-violet-700)' },
		jpg: { icon: 'image', color: 'var(--miso-violet-700)' },
		jpeg: { icon: 'image', color: 'var(--miso-violet-700)' },
		gif: { icon: 'image', color: 'var(--miso-violet-700)' },
		svg: { icon: 'image', color: 'var(--miso-violet-700)' },
		pdf: { icon: 'document', color: 'var(--miso-red-700)' },
		doc: { icon: 'document', color: 'var(--miso-interblue-700)' },
		docx: { icon: 'document', color: 'var(--miso-interblue-700)' },
		xls: { icon: 'document', color: 'var(--miso-pine-700)' },
		xlsx: { icon: 'document', color: 'var(--miso-pine-700)' },
		zip: { icon: 'archive', color: 'var(--miso-amber-700)' }
	};

	let ext = $derived(name.split('.').pop()?.toLowerCase() ?? '');
	let type = $derived(typeIcons[ext] ?? { icon: 'attach' as IconName, color: 'var(--miso-txt-tertiary)' });
</script>

<!-- archive #files_new .fileItem: 204px card, radius 8, ghost ring, hover shadow -->
<!-- role/tabindex are set only when onclick is provided -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
	class={['miso-file', selected && 'miso-file--selected', onclick && 'miso-file--clickable']}
	style:width="{width}px"
	role={onclick ? 'button' : undefined}
	tabindex={onclick ? 0 : undefined}
	onclick={onclick}
	onkeydown={(event) => {
		if (onclick && (event.key === 'Enter' || event.key === ' ')) {
			event.preventDefault();
			onclick();
		}
	}}
>
	<span class="miso-file__icon" style:color={icon ? undefined : type.color}>
		<Icon name={icon ?? type.icon} size={20} />
	</span>
	<span class="miso-file__info">
		<span class="miso-file__name">{name}</span>
		{#if meta}
			<span class="miso-file__meta">{meta}</span>
		{/if}
	</span>
	<span class="miso-file__actions">
		{#if ondownload}
			<Button scheme="g" variant="ghost" size={24} icon="download" onclick={ondownload} aria-label="Download" />
		{/if}
		{#if ondelete}
			<Button scheme="g" variant="ghost" size={24} icon="trash" onclick={ondelete} aria-label="Delete" />
		{/if}
	</span>
</div>

<style>
	.miso-file {
		box-sizing: border-box;
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px;
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-top);
		box-shadow:
			0 0 0 1px var(--miso-line-ghost),
			0 2px 8px -2px var(--miso-graphite-a300);
		transition: box-shadow 0.15s ease;
	}

	.miso-file--clickable {
		cursor: pointer;
	}

	.miso-file:hover {
		box-shadow: var(--miso-shadow-cont-major-outline);
	}

	.miso-file--selected,
	.miso-file--selected:hover {
		box-shadow: var(--miso-brd-focus);
	}

	.miso-file:focus-visible {
		outline: none;
		box-shadow: var(--miso-brd-focus);
	}

	.miso-file__icon {
		display: inline-flex;
		flex-shrink: 0;
	}

	.miso-file__info {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
	}

	.miso-file__name {
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.miso-file__meta {
		font: var(--miso-font-ui-10-reg);
		color: var(--miso-txt-tertiary);
	}

	.miso-file__actions {
		display: none;
		flex-shrink: 0;
		gap: 2px;
	}

	.miso-file:hover .miso-file__actions,
	.miso-file--selected .miso-file__actions {
		display: inline-flex;
	}
</style>
