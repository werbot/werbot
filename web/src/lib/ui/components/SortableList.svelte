<script lang="ts" generics="T">
	import { flip } from 'svelte/animate';
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	interface Props {
		items: T[];
		/** unique key per item for keyed rendering */
		key: (item: T) => string | number;
		item: Snippet<[T, number]>;
		/** hide the grip handle (whole row becomes the handle) */
		handle?: boolean;
		onreorder?: (items: T[]) => void;
	}

	let { items = $bindable(), key, item, handle = true, onreorder }: Props = $props();

	let dragIndex = $state<number | null>(null);
	let overIndex = $state<number | null>(null);

	function onDragStart(event: DragEvent, index: number) {
		dragIndex = index;
		event.dataTransfer!.effectAllowed = 'move';
		// hide the default ghost offset issues in some browsers
		event.dataTransfer!.setData('text/plain', String(index));
	}

	function onDragOver(event: DragEvent, index: number) {
		event.preventDefault();
		event.dataTransfer!.dropEffect = 'move';
		overIndex = index;
	}

	function onDrop(event: DragEvent, index: number) {
		event.preventDefault();
		if (dragIndex === null || dragIndex === index) {
			reset();
			return;
		}
		const next = [...items];
		const [moved] = next.splice(dragIndex, 1);
		next.splice(index, 0, moved);
		items = next;
		onreorder?.(next);
		reset();
	}

	function reset() {
		dragIndex = null;
		overIndex = null;
	}
</script>

<div class="miso-sortable" role="list">
	{#each items as entry, index (key(entry))}
		<div
			class={[
				'miso-sortable__row',
				dragIndex === index && 'miso-sortable__row--dragging',
				overIndex === index && dragIndex !== null && dragIndex !== index && 'miso-sortable__row--over',
				overIndex === index && dragIndex !== null && index < dragIndex && 'miso-sortable__row--before'
			]}
			role="listitem"
			draggable="true"
			animate:flip={{ duration: 200 }}
			ondragstart={(event) => onDragStart(event, index)}
			ondragover={(event) => onDragOver(event, index)}
			ondrop={(event) => onDrop(event, index)}
			ondragend={reset}
		>
			{#if handle}
				<span class="miso-sortable__grip" aria-hidden="true"><Icon name="grip" size={16} /></span>
			{/if}
			<div class="miso-sortable__content">
				{@render item(entry, index)}
			</div>
		</div>
	{/each}
</div>

<style>
	.miso-sortable {
		display: flex;
		flex-direction: column;
		width: 100%;
	}

	.miso-sortable__row {
		position: relative;
		display: flex;
		align-items: center;
		gap: 4px;
		border-radius: var(--miso-radius-06);
	}

	.miso-sortable__row--dragging {
		opacity: 0.4;
	}

	/* drop indicator: 2px interblue line, like the archive width-dragger */
	.miso-sortable__row--over::after {
		content: '';
		position: absolute;
		left: 0;
		right: 0;
		bottom: -1px;
		height: 2px;
		border-radius: 1px;
		background: var(--miso-hlt-invert);
	}

	.miso-sortable__row--over.miso-sortable__row--before::after {
		top: -1px;
		bottom: auto;
	}

	.miso-sortable__grip {
		display: inline-flex;
		flex-shrink: 0;
		padding: 2px;
		color: var(--miso-txt-muted);
		cursor: grab;
		opacity: 0;
		transition: opacity 0.15s ease;
	}

	.miso-sortable__row:hover .miso-sortable__grip {
		opacity: 1;
	}

	.miso-sortable__row--dragging .miso-sortable__grip {
		cursor: grabbing;
	}

	.miso-sortable__content {
		flex: 1;
		min-width: 0;
	}
</style>
