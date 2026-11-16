<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import type { Snippet } from 'svelte';
	import Button from './Button.svelte';

	interface Props {
		open?: boolean;
		title?: string;
		/** modal width in px */
		width?: number;
		/** gray footer background like the original .buttons_gray */
		footerGray?: boolean;
		/** hairline above the footer like the original .buttons_wline */
		footerLine?: boolean;
		/** remove default body paddings (for custom layouts) */
		plain?: boolean;
		/** footer actions snippet */
		footer?: Snippet;
		children?: Snippet;
		onclose?: () => void;
	}

	let {
		open = $bindable(false),
		title,
		width = 520,
		footerGray = false,
		footerLine = false,
		plain = false,
		footer,
		children,
		onclose
	}: Props = $props();

	function close() {
		open = false;
		onclose?.();
	}

	function onBackdropClick(event: MouseEvent) {
		if (event.target === event.currentTarget) {
			close();
		}
	}

	function onKeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			close();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="miso-modal-overlay" transition:fade={{ duration: 200 }} onclick={onBackdropClick}>
		<div
			class="miso-modal"
			style:width="{width}px"
			transition:scale={{ start: 0.97, duration: 180, easing: cubicOut }}
			role="dialog"
			aria-modal="true"
			aria-label={title}
		>
			{#if title}
				<div class="miso-modal__header">
					<div class="miso-modal__title">{title}</div>
					<Button
						scheme="g"
						variant="ghost"
						size={36}
						icon="close"
						onclick={close}
						aria-label="Close"
					/>
				</div>
			{/if}
			<div class={['miso-modal__content', 'miso-scroll', plain && 'miso-modal__content--plain']}>
				{@render children?.()}
			</div>
			{#if footer}
				<div
					class={[
						'miso-modal__buttons',
						footerGray && 'miso-modal__buttons--gray',
						footerLine && 'miso-modal__buttons--line'
					]}
				>
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	/* 1:1 with the original .modal__overlay: top-aligned, 13vh offset */
	.miso-modal-overlay {
		position: fixed;
		inset: 0;
		z-index: var(--miso-z-modal);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding: 13vh 16px;
		background: var(--miso-overlay);
		transition: background 0.2s ease-out;
	}

	/* 1:1 with the original .modal */
	.miso-modal {
		position: relative;
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
		max-width: 100%;
		max-height: 74vh;
		border-radius: 16px;
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-mod-major);
		overflow: hidden;
		text-align: left;
		font: 12px/20px var(--miso-font-base);
		color: var(--miso-txt-primary);
		pointer-events: auto;
	}

	/* .modal__header{padding:8px 16px 0;gap:12px} */
	.miso-modal__header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 8px 16px 0;
		flex-shrink: 0;
	}

	/* .modal__header>.in{padding:8px 0 8px 12px;font:22/28 bold;height:36px} */
	.miso-modal__title {
		display: flex;
		align-items: center;
		align-self: stretch;
		height: 36px;
		min-width: 0;
		padding: 8px 0 8px 12px;
		font: var(--miso-font-ui-22-bold);
		color: var(--miso-txt-accent);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* .modal__content + .modal__content__block{padding:12px 28px} */
	.miso-modal__content {
		flex: 1 1 auto;
		min-height: 0;
		padding: 12px 28px;
		overflow: auto;
		font: var(--miso-font-txt-13-reg);
		color: var(--miso-txt-primary);
	}

	.miso-modal__content--plain {
		padding: 0;
	}

	/* .modal__content .buttons{padding:12px 28px 28px} */
	.miso-modal__buttons {
		display: flex;
		justify-content: flex-end;
		align-items: center;
		gap: 12px;
		padding: 12px 28px 28px;
		flex-shrink: 0;
	}

	.miso-modal__buttons--line {
		border-top: 0.5px solid var(--miso-line-tertiary);
	}

	.miso-modal__buttons--gray {
		background: var(--miso-cont-low);
	}
</style>
