<script lang="ts">
	import { Button, Modal } from '#lib/ui/index.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the fresh codes, shown once; an empty list closes the dialog */
		codes: string[];
		onclose: () => void;
	}

	let { codes, onclose }: Props = $props();

	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(codes.join('\n'));
			copied = true;
		} catch {
			failToast(m.mfa_copy_failed());
		}
	}
</script>

<Modal open={codes.length > 0} {onclose} title={m.mfa_recovery_title()} width={440}>
	<p>{m.mfa_recovery_hint()}</p>
	<ul>
		{#each codes as code (code)}<li><code>{code}</code></li>{/each}
	</ul>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={copy}>{copied ? m.gateways_copied() : m.gateways_copy()}</Button>
		<Button size={40} variant="solid" onclick={onclose}>{m.mfa_recovery_saved()}</Button>
	{/snippet}
</Modal>

<style>
	p {
		margin: 0 0 var(--miso-sp-12);
	}

	ul {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: var(--miso-sp-08);
		margin: 0;
		padding: 0;
		list-style: none;
		text-align: center;
	}

	code {
		font-size: 14px;
	}
</style>
