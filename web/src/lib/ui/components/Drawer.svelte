<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import type { Snippet } from 'svelte';
	import Button from './Button.svelte';

	interface Props {
		open?: boolean;
		/** edge the panel slides from */
		side?: 'right' | 'left';
		/** panel width in px; bindable — updates while resizing */
		width?: number;
		title?: string;
		/** dimmed backdrop behind the panel */
		backdrop?: boolean;
		/** drag the inner edge to resize */
		resizable?: boolean;
		minWidth?: number;
		/** default: 95% of the viewport */
		maxWidth?: number;
		/** magnet: widths the resizer snaps to (±16px), as in the archive */
		snapPoints?: number[];
		footer?: Snippet;
		children?: Snippet;
		onclose?: () => void;
	}

	let {
		open = $bindable(false),
		side = 'right',
		width = $bindable(520),
		title,
		backdrop = true,
		resizable = false,
		minWidth = 320,
		maxWidth,
		snapPoints = [],
		footer,
		children,
		onclose
	}: Props = $props();

	const SNAP = 16;

	let dragging = $state(false);
	let startX = 0;
	let startW = 0;

	function close() {
		open = false;
		onclose?.();
	}

	function onKeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			close();
		}
	}

	function limit(): number {
		return maxWidth ?? Math.round(window.innerWidth * 0.95);
	}

	function clamp(w: number): number {
		const max = limit();
		if (w < minWidth) return minWidth;
		if (w > max) return max;
		// magnet to explicit snap points
		for (const point of snapPoints) {
			if (Math.abs(w - point) <= SNAP) return point;
		}
		// archive behavior: dragging past ~85% of the limit jumps to full width
		if (w > max * 0.85) return max;
		return w;
	}

	function onResizeStart(event: PointerEvent) {
		dragging = true;
		startX = event.clientX;
		startW = width;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		document.body.style.cursor = 'col-resize';
		document.body.style.userSelect = 'none';
	}

	function onResizeMove(event: PointerEvent) {
		if (!dragging) return;
		const delta = side === 'right' ? startX - event.clientX : event.clientX - startX;
		width = clamp(startW + delta);
	}

	function onResizeEnd() {
		if (!dragging) return;
		dragging = false;
		document.body.style.cursor = '';
		document.body.style.userSelect = '';
	}

	function onResizeKeydown(event: KeyboardEvent) {
		const grow = side === 'right' ? 'ArrowLeft' : 'ArrowRight';
		const shrink = side === 'right' ? 'ArrowRight' : 'ArrowLeft';
		if (event.key === grow) {
			width = clamp(width + SNAP);
			event.preventDefault();
		} else if (event.key === shrink) {
			width = clamp(width - SNAP);
			event.preventDefault();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	{#if backdrop}
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div
			class="miso-drawer-backdrop"
			transition:fade={{ duration: 300, easing: cubicOut }}
			onclick={close}
		></div>
	{/if}
	<div
		class={['miso-drawer', `miso-drawer--${side}`, dragging && 'miso-drawer--dragging']}
		style:width="{width}px"
		transition:fly={{ x: side === 'right' ? width : -width, duration: 300, opacity: 1, easing: cubicOut }}
		role="dialog"
		aria-modal="true"
		aria-label={title}
	>
		{#if resizable}
			<!-- focusable separator is a valid ARIA pattern for resizers -->
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions, a11y_no_noninteractive_tabindex -->
			<div
				class="miso-drawer__resize"
				role="separator"
				aria-orientation="vertical"
				aria-label="Resize panel"
				tabindex="0"
				onpointerdown={onResizeStart}
				onpointermove={onResizeMove}
				onpointerup={onResizeEnd}
				onpointercancel={onResizeEnd}
				onkeydown={onResizeKeydown}
			></div>
		{/if}
		<header class="miso-drawer__header">
			<h2 class="miso-drawer__title">{title}</h2>
			<Button scheme="g" variant="ghost" size={28} icon="x" onclick={close} aria-label="Close" />
		</header>
		<div class="miso-drawer__body miso-scroll">
			{@render children?.()}
		</div>
		{#if footer}
			<footer class="miso-drawer__footer">
				{@render footer()}
			</footer>
		{/if}
	</div>
{/if}

<style>
	.miso-drawer-backdrop {
		position: fixed;
		inset: 0;
		z-index: calc(var(--miso-z-drawer) - 1);
		background: var(--miso-dark-a20);
	}

	/* archive: #sidePanel__wrapper — 520px, cont-mid, major outline shadow */
	.miso-drawer {
		position: fixed;
		top: 0;
		bottom: 0;
		z-index: var(--miso-z-drawer);
		display: flex;
		flex-direction: column;
		max-width: 100vw;
		background: var(--miso-cont-mid);
		box-shadow: var(--miso-shadow-cont-major-outline);
	}

	.miso-drawer--right {
		right: 0;
	}

	.miso-drawer--left {
		left: 0;
	}

	/* archive: .taskView__resizeHandler — 9px strip over the inner edge */
	.miso-drawer__resize {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 9px;
		z-index: 5;
		cursor: ew-resize;
		touch-action: none;
	}

	.miso-drawer--right .miso-drawer__resize {
		left: -5px;
	}

	.miso-drawer--left .miso-drawer__resize {
		right: -5px;
	}

	.miso-drawer__resize::after {
		content: '';
		position: absolute;
		top: 0;
		bottom: 0;
		left: 4px;
		width: 2px;
		background: var(--miso-hlt-invert);
		opacity: 0;
		transition: opacity 0.15s ease;
	}

	.miso-drawer__resize:hover::after,
	.miso-drawer__resize:focus-visible::after,
	.miso-drawer--dragging .miso-drawer__resize::after {
		opacity: 1;
	}

	.miso-drawer__resize:focus-visible {
		outline: none;
	}

	.miso-drawer--dragging {
		user-select: none;
	}

	.miso-drawer__header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 14px 12px 10px 20px;
	}

	.miso-drawer__title {
		margin: 0;
		font: var(--miso-font-ui-16-semi);
		color: var(--miso-txt-accent);
	}

	.miso-drawer__body {
		flex: 1;
		padding: 4px 20px 20px;
		overflow: auto;
		font: var(--miso-font-txt-13-reg);
		color: var(--miso-txt-primary);
	}

	.miso-drawer__footer {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding: 12px 20px;
		border-top: 1px solid var(--miso-line-tertiary);
		background: var(--miso-cont-top);
	}
</style>
