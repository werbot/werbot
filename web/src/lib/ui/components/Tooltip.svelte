<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** plain text content */
		text?: string;
		/** optional bold first line */
		title?: string;
		/** hotkey hint rendered as keys */
		keys?: string[];
		placement?: 'top' | 'bottom' | 'left' | 'right';
		/** rich content snippet (overrides text) */
		content?: Snippet;
		children?: Snippet;
	}

	let { text, title, keys, placement = 'top', content, children }: Props = $props();

	/*
	 * The bubble is position:fixed and coordinates are computed from the anchor,
	 * so it never clips inside overflow/scroll containers (menus, tables, panels)
	 * and never inflates their scrollWidth.
	 */
	let wrap: HTMLElement | undefined = $state();
	let show = $state(false);
	let top = $state(0);
	let left = $state(0);

	function reposition() {
		if (!wrap) return;
		const a = wrap.getBoundingClientRect();
		const gap = 6;
		if (placement === 'top') {
			top = a.top - gap;
			left = a.left + a.width / 2;
		} else if (placement === 'bottom') {
			top = a.bottom + gap;
			left = a.left + a.width / 2;
		} else if (placement === 'left') {
			top = a.top + a.height / 2;
			left = a.left - gap;
		} else {
			top = a.top + a.height / 2;
			left = a.right + gap;
		}
	}

	function open() {
		reposition();
		show = true;
	}

	function close() {
		show = false;
	}
</script>

<svelte:window onscrollcapture={() => show && close()} onresize={() => show && close()} />

<span
	class="miso-tooltip-wrap"
	bind:this={wrap}
	role="presentation"
	onpointerenter={open}
	onpointerleave={close}
	onfocusin={open}
	onfocusout={close}
>
	{@render children?.()}
	<span
		class={['miso-tooltip', `miso-tooltip--${placement}`, show && 'miso-tooltip--show']}
		style:top="{top}px"
		style:left="{left}px"
		role="tooltip"
	>
		{#if title}
			<span class="miso-tooltip__title">
				<span class="miso-tooltip__title-text">{title}</span>
				{#if keys?.length}
					<span class="miso-tooltip__keys">
						{#each keys as key (key)}
							<kbd>{key}</kbd>
						{/each}
					</span>
				{/if}
			</span>
		{/if}
		{#if content}
			{@render content()}
		{:else if text}
			<span class="miso-tooltip__text">{text}</span>
		{/if}
		{#if !title && keys?.length}
			<span class="miso-tooltip__keys">
				{#each keys as key (key)}
					<kbd>{key}</kbd>
				{/each}
			</span>
		{/if}
	</span>
</span>

<style>
	.miso-tooltip-wrap {
		position: relative;
		display: inline-flex;
	}

	.miso-tooltip {
		position: fixed;
		z-index: var(--miso-z-tooltip);
		display: flex;
		flex-direction: column;
		gap: 2px;
		max-width: 240px;
		width: max-content;
		padding: 6px 10px;
		border-radius: var(--miso-radius-08);
		background: var(--miso-dark-a80);
		backdrop-filter: blur(2px);
		opacity: 0;
		visibility: hidden;
		transition: opacity 120ms ease 150ms;
		pointer-events: none;
	}

	.miso-tooltip--show {
		opacity: 1;
		visibility: visible;
	}

	.miso-tooltip--top {
		translate: -50% -100%;
	}

	.miso-tooltip--bottom {
		translate: -50% 0;
	}

	.miso-tooltip--left {
		translate: -100% -50%;
	}

	.miso-tooltip--right {
		translate: 0 -50%;
	}

	.miso-tooltip__title {
		display: flex;
		align-items: center;
		gap: 8px;
		font: var(--miso-font-ui-13-semi);
		color: var(--miso-light-a90);
	}

	.miso-tooltip__title-text {
		flex: 1;
	}

	.miso-tooltip__text {
		font: var(--miso-font-txt-13-reg);
		color: var(--miso-light-a70);
	}

	.miso-tooltip__keys {
		display: inline-flex;
		gap: 2px;
	}

	.miso-tooltip__keys kbd {
		padding: 0 6px;
		border: 1px solid var(--miso-light-a20);
		border-radius: var(--miso-radius-04);
		background: var(--miso-light-a10);
		font: var(--miso-font-ui-11-med);
		color: var(--miso-light-a90);
		text-transform: capitalize;
	}
</style>
