<script lang="ts" module>
	export type ToasterPosition =
		| 'bottom-left'
		| 'bottom-center'
		| 'bottom-right'
		| 'top-left'
		| 'top-center'
		| 'top-right';
</script>

<script lang="ts">
	import { fly } from 'svelte/transition';
	import { flip } from 'svelte/animate';
	import { toasts, dismissToast } from '../toaster.svelte.js';
	import Toast from './Toast.svelte';

	interface Props {
		position?: ToasterPosition;
	}

	let { position = 'bottom-right' }: Props = $props();

	let fromTop = $derived(position.startsWith('top'));
</script>

<div class={['miso-toaster', `miso-toaster--${position}`]}>
	{#each toasts as item (item.id)}
		<div
			class="miso-toaster__item"
			transition:fly={{ y: fromTop ? -28 : 28, duration: 400 }}
			animate:flip={{ duration: 300 }}
		>
			<Toast
				tone={item.tone}
				title={item.title}
				text={item.text}
				icon={item.icon}
				duration={item.duration}
				count={item.count}
				action={item.action}
				onclose={() => dismissToast(item.id)}
			/>
		</div>
	{/each}
</div>

<style>
	.miso-toaster {
		position: fixed;
		z-index: var(--miso-z-toast);
		display: flex;
		flex-direction: column;
		gap: 14px;
		pointer-events: none;
	}

	.miso-toaster--bottom-left,
	.miso-toaster--bottom-center,
	.miso-toaster--bottom-right {
		bottom: 28px;
	}

	.miso-toaster--top-left,
	.miso-toaster--top-center,
	.miso-toaster--top-right {
		top: 20px;
		flex-direction: column-reverse;
	}

	.miso-toaster--bottom-left,
	.miso-toaster--top-left {
		left: 24px;
	}

	.miso-toaster--bottom-right,
	.miso-toaster--top-right {
		right: 24px;
	}

	.miso-toaster--bottom-center,
	.miso-toaster--top-center {
		left: 50%;
		translate: -50% 0;
		align-items: center;
	}

	.miso-toaster__item {
		pointer-events: auto;
	}
</style>
