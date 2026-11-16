<script lang="ts">
	import Icon from './Icon.svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		/** text — 13px row; title — 22px semibold, as the task name in the archive */
		variant?: 'text' | 'title';
		readonly?: boolean;
		/** called after committing a change */
		onsave?: (value: string) => void;
	}

	let {
		value = $bindable(''),
		placeholder = 'Empty',
		variant = 'text',
		readonly = false,
		onsave
	}: Props = $props();

	let editing = $state(false);
	let draft = $state('');

	function autofocus(node: HTMLInputElement) {
		node.focus();
		node.select();
	}

	function startEdit() {
		if (readonly) return;
		draft = value;
		editing = true;
	}

	function commit() {
		if (!editing) return;
		editing = false;
		if (draft !== value) {
			value = draft;
			onsave?.(draft);
		}
	}

	function cancel() {
		editing = false;
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			commit();
		} else if (event.key === 'Escape') {
			cancel();
		}
	}
</script>

<span class={['miso-inline-edit', `miso-inline-edit--${variant}`, readonly && 'miso-inline-edit--readonly']}>
	{#if editing}
		<input
			class="miso-inline-edit__input"
			type="text"
			bind:value={draft}
			onkeydown={onKeydown}
			onblur={commit}
			{@attach autofocus}
		/>
	{:else}
		<button type="button" class="miso-inline-edit__view" onclick={startEdit} disabled={readonly}>
			<span class={['miso-inline-edit__value', !value && 'miso-inline-edit__value--empty']}>
				{value || placeholder}
			</span>
			{#if !readonly}
				<span class="miso-inline-edit__pencil"><Icon name="edit" size={variant === 'title' ? 16 : 14} /></span>
			{/if}
		</button>
	{/if}
</span>

<style>
	.miso-inline-edit {
		display: inline-flex;
		max-width: 100%;
		min-width: 0;
	}

	.miso-inline-edit__view {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		max-width: 100%;
		min-width: 0;
		margin: 0;
		border: 0;
		background: none;
		text-align: left;
		cursor: text;
		color: inherit;
	}

	/* archive .editable-value: hover ring 1px inset, blue */
	.miso-inline-edit__view:hover:not(:disabled) {
		box-shadow: 0 0 0 1px var(--miso-hlt-invert-minor) inset;
	}

	.miso-inline-edit__view:disabled {
		cursor: default;
	}

	.miso-inline-edit__value {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.miso-inline-edit__value--empty {
		color: var(--miso-txt-muted);
	}

	.miso-inline-edit__pencil {
		display: inline-flex;
		flex-shrink: 0;
		color: var(--miso-txt-tertiary);
		opacity: 0;
		transition: opacity 0.15s ease;
	}

	.miso-inline-edit__view:hover .miso-inline-edit__pencil {
		opacity: 1;
	}

	.miso-inline-edit__input {
		box-sizing: border-box;
		width: 100%;
		margin: 0;
		border: 0;
		background: var(--miso-cont-top);
		color: var(--miso-txt-primary);
		outline: none;
		box-shadow: var(--miso-brd-focus);
	}

	/* text: 13px row, 28px tall, radius 6 */
	.miso-inline-edit--text .miso-inline-edit__view,
	.miso-inline-edit--text .miso-inline-edit__input {
		font: var(--miso-font-ui-13-reg);
		padding: 0 8px;
		height: 28px;
		border-radius: var(--miso-radius-06);
	}

	/* title: task name — 22px semibold, radius 8 */
	.miso-inline-edit--title .miso-inline-edit__view,
	.miso-inline-edit--title .miso-inline-edit__input {
		font: var(--miso-font-ui-22-semi);
		color: var(--miso-txt-accent);
		padding: 4px 8px;
		border-radius: var(--miso-radius-08);
	}

	.miso-inline-edit--title .miso-inline-edit__input {
		min-width: 280px;
	}
</style>
