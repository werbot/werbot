<script lang="ts">
	import { fly } from 'svelte/transition';
	import type { Snippet } from 'svelte';

	interface Props {
		open?: boolean;
		placement?: 'bottom-start' | 'bottom-end' | 'top-start' | 'top-end';
		/** trigger element; receives toggle callback and open state */
		trigger: Snippet<[{ toggle: () => void; open: boolean }]>;
		/** dropdown content; receives close callback */
		children: Snippet<[{ close: () => void }]>;
	}

	let { open = $bindable(false), placement = 'bottom-start', trigger, children }: Props = $props();

	let root: HTMLElement | undefined = $state();
	let panelEl: HTMLElement | undefined;
	let measured = $state(false);
	let top = $state(0);
	let left = $state(0);

	function toggle() {
		open = !open;
	}

	function close() {
		open = false;
	}

	function reposition() {
		if (!root || !panelEl) return;
		const anchor = root.getBoundingClientRect();
		const menu = panelEl.getBoundingClientRect();
		const gap = 4;
		let nextTop = placement.startsWith('top') ? anchor.top - menu.height - gap : anchor.bottom + gap;
		let nextLeft = placement.endsWith('end') ? anchor.right - menu.width : anchor.left;
		nextLeft = Math.min(Math.max(4, nextLeft), window.innerWidth - menu.width - 4);
		if (nextTop + menu.height > window.innerHeight - 4) {
			nextTop = Math.max(4, anchor.top - menu.height - gap);
		}
		if (nextTop < 4) nextTop = anchor.bottom + gap;
		if (top !== nextTop) top = nextTop;
		if (left !== nextLeft) left = nextLeft;
	}

	function place(node: HTMLElement) {
		panelEl = node;
		reposition();
		measured = true;
		return () => {
			panelEl = undefined;
			measured = false;
		};
	}

	function onWindowPointerDown(event: PointerEvent) {
		const target = event.target as Node;
		if (open && !root?.contains(target) && !panelEl?.contains(target)) {
			close();
		}
	}

	function onWindowKeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			event.stopImmediatePropagation();
			close();
		}
	}
</script>

<svelte:window
	onpointerdown={onWindowPointerDown}
	onkeydown={onWindowKeydown}
	onresize={() => open && reposition()}
	onscrollcapture={() => open && reposition()}
/>

<span class="miso-dropdown" bind:this={root}>
	{@render trigger({ toggle, open })}
	{#if open}
		<div
			class="miso-dropdown__panel"
			style:top="{top}px"
			style:left="{left}px"
			style:visibility={measured ? 'visible' : 'hidden'}
			transition:fly={{ y: placement.startsWith('top') ? 4 : -4, duration: 120 }}
			{@attach place}
		>
			{@render children({ close })}
		</div>
	{/if}
</span>

<style>
	.miso-dropdown {
		position: relative;
		display: inline-flex;
	}

	.miso-dropdown__panel {
		position: fixed;
		z-index: var(--miso-z-dropdown);
	}
</style>
