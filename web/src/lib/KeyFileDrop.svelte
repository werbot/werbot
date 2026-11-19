<script lang="ts">
	import { Button, DropZone, FileItem } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** drop caption, also shown next to the button */
		hint: string;
		/** the file that filled the form; empty shows the hint */
		fileName: string;
		accept?: string;
		onfile: (file: File) => void;
		onclear: () => void;
	}

	let { hint, fileName, accept, onfile, onclear }: Props = $props();

	let picker = $state<HTMLInputElement>();

	function pick(file?: File) {
		if (file) onfile(file);
		// the same file can be chosen again
		if (picker) picker.value = '';
	}
</script>

<!-- a key file fills the form: drop it or choose it -->
<DropZone label={hint} ondropfiles={(files) => pick(files[0])}>
	<div class="drop">
		{#if fileName}
			<FileItem name={fileName} ondelete={onclear} />
		{:else}
			<span class="hint">{hint}</span>
			<Button type="button" scheme="g" variant="plain" size={28} iconLeft="upload" onclick={() => picker?.click()}>{m.keys_choose_file()}</Button>
		{/if}
		<input bind:this={picker} type="file" {accept} hidden onchange={(e) => pick(e.currentTarget.files?.[0])} />
	</div>
</DropZone>

<style>
	.drop {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--miso-sp-08);
		padding: var(--miso-sp-12);
		border: 1px dashed var(--miso-line-primary);
		border-radius: var(--miso-radius-08);
		box-sizing: border-box;
	}

	.hint {
		color: var(--miso-txt-secondary);
	}
</style>
