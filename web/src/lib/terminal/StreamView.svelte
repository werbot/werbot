<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Banner, Button } from '#lib/ui/index.js';
	import { requestStepUp } from '#lib/stepup.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import XTerm from './XTerm.svelte';
	import { closeMessage, connect, wsURL, type Socket } from './socket.js';

	interface Props {
		/** WebSocket route on this origin */
		path: string;
		params?: Record<string, string>;
		/** web terminal: keyboard input and window size go to the server; otherwise a read-only live view */
		interactive?: boolean;
	}

	let { path, params = {}, interactive = false }: Props = $props();

	let xterm: XTerm | undefined = $state();
	let sock: Socket | undefined;
	let size = { cols: 0, rows: 0 };
	let closed = $state<{ code: number; reason: string } | null>(null);
	let destroyed = false;

	function open() {
		closed = null;
		const query = interactive ? { ...params, cols: size.cols, rows: size.rows } : params;
		sock = connect(wsURL(path, query), {
			ondata: (d) => xterm?.write(d),
			onresize: (cols, rows) => xterm?.resize(cols, rows),
			onclose: (code, reason) => {
				sock = undefined;
				closed = { code, reason };
				// 4001: the terminal needs a fresh second factor; once confirmed, connect again unless the view is gone
				if (interactive && code === 4001) void requestStepUp().then((ok) => ok && !destroyed && reconnect());
			}
		});
	}

	function reconnect() {
		xterm?.reset();
		open();
	}

	// the web terminal opens with the measured window; a live view opens at once and follows the server's size
	function onresize(cols: number, rows: number) {
		size = { cols, rows };
		if (!interactive) return;
		if (sock) sock.resize(cols, rows);
		else if (!closed) open();
	}

	onMount(() => {
		if (!interactive) open();
	});

	onDestroy(() => {
		destroyed = true;
		sock?.close();
	});
</script>

<div class="stream">
	{#if closed}
		<Banner tone="info">
			{closeMessage(closed.code, closed.reason)}
			<Button scheme="g" variant="plain" size={40} onclick={reconnect}>{m.terminal_reconnect()}</Button>
		</Banner>
	{/if}
	<div class="stream__screen">
		<XTerm bind:this={xterm} readonly={!interactive} fit={interactive} ondata={(d) => sock?.send(d)} {onresize} />
	</div>
</div>

<style>
	.stream {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		height: calc(100dvh - 190px);
		min-height: 320px;
	}

	.stream__screen {
		flex: 1;
		min-height: 0;
		overflow: auto;
	}
</style>
