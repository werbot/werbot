<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	interface Props {
		/** caption while dragging over the page */
		label?: string;
		/** caption while hovering the zone, right before drop */
		readyLabel?: string;
		/** cover the whole viewport like the original body drop zone */
		fullscreen?: boolean;
		children?: Snippet;
		ondropfiles?: (files: File[]) => void;
	}

	let {
		label = 'drag & drop files here',
		readyLabel = 'drop',
		fullscreen = false,
		children,
		ondropfiles
	}: Props = $props();

	let depth = $state(0);
	let ready = $derived(depth > 0);

	function hasFiles(event: DragEvent): boolean {
		return Array.from(event.dataTransfer?.types ?? []).includes('Files');
	}

	function onDragEnter(event: DragEvent) {
		if (!hasFiles(event)) return;
		event.preventDefault();
		depth += 1;
	}

	function onDragOver(event: DragEvent) {
		if (!hasFiles(event)) return;
		event.preventDefault();
	}

	function onDragLeave(event: DragEvent) {
		if (!hasFiles(event)) return;
		depth = Math.max(0, depth - 1);
	}

	function onDrop(event: DragEvent) {
		if (!hasFiles(event)) return;
		event.preventDefault();
		depth = 0;
		const files = Array.from(event.dataTransfer?.files ?? []);
		if (files.length) {
			ondropfiles?.(files);
		}
	}
</script>

<div
	class="miso-dropzone"
	role="region"
	aria-label={label}
	ondragenter={onDragEnter}
	ondragover={onDragOver}
	ondragleave={onDragLeave}
	ondrop={onDrop}
>
	{@render children?.()}
	{#if ready}
		<div class={['miso-dropzone__in', fullscreen && 'miso-dropzone__in--fixed']}>
			<div class="miso-dropzone__circle">
				<span class="miso-dropzone__icon"><Icon name="helper/drop" size={96} /></span>
				<span class="miso-dropzone__label">{readyLabel}</span>
			</div>
		</div>
	{:else if children == null}
		<div class="miso-dropzone__idle">
			<div class="miso-dropzone__circle">
				<span class="miso-dropzone__icon"><Icon name="helper/drop" size={96} /></span>
				<span class="miso-dropzone__label">{label}</span>
			</div>
		</div>
	{/if}
</div>

<style>
	.miso-dropzone {
		position: relative;
	}

	/* 1:1 with the original .drop_zone_in: light-80 tint that turns blue
	   (interblue-trans-200) when the file hovers the zone */
	.miso-dropzone__in {
		position: absolute;
		inset: 0;
		z-index: var(--miso-z-dropzone);
		display: flex;
		justify-content: center;
		align-items: center;
		background: var(--miso-interblue-a200);
		pointer-events: none;
	}

	.miso-dropzone__in--fixed {
		position: fixed;
	}

	.miso-dropzone__idle {
		display: flex;
		justify-content: center;
		align-items: center;
		padding: 24px;
	}

	/* 1:1 with .drop_zone_move / .drop_zone_ready: 240px circle with blur */
	.miso-dropzone__circle {
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;
		gap: 16px;
		width: 240px;
		height: 240px;
		border-radius: 50%;
		background: linear-gradient(0deg, var(--miso-light-a80), var(--miso-cont-top));
		backdrop-filter: blur(4px);
		text-align: center;
	}

	.miso-dropzone__icon {
		display: inline-flex;
		color: var(--miso-txt-primary);
		transform-origin: bottom center;
	}

	.miso-dropzone__in .miso-dropzone__icon {
		animation: miso-dropready 3s ease 0.5s 1 normal forwards;
	}

	.miso-dropzone__label {
		max-width: 200px;
		font: 600 1.375rem/28px var(--miso-font-mono);
		color: var(--miso-txt-primary);
	}

	/* original @keyframes miso-dropready: wiggle then bounce-scale */
	@keyframes miso-dropready {
		10% {
			transform: rotate(35deg);
		}
		20% {
			transform: rotate(-35deg);
		}
		30% {
			transform: rotate(0);
		}
		60% {
			transform: scale(1.15);
		}
		70% {
			transform: scale(0.9);
		}
		80% {
			transform: scale(1.3);
		}
		90% {
			transform: scale(0.8);
		}
		100% {
			transform: scale(1);
		}
	}
</style>
