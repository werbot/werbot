<script lang="ts">
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import '@xterm/xterm/css/xterm.css';

	interface Props {
		/** no keyboard input (live view) */
		readonly?: boolean;
		/** follow the container size and report it through onresize */
		fit?: boolean;
		ondata?: (d: Uint8Array<ArrayBuffer>) => void;
		onresize?: (cols: number, rows: number) => void;
	}

	let { readonly = false, fit = false, ondata, onresize }: Props = $props();

	let term: Terminal | undefined;
	const encoder = new TextEncoder();

	export function write(d: Uint8Array) {
		term?.write(d);
	}

	export function resize(cols: number, rows: number) {
		term?.resize(cols, rows);
	}

	export function reset() {
		term?.reset();
	}

	function mount(el: HTMLElement) {
		const t = new Terminal({ disableStdin: readonly, cursorBlink: !readonly, fontFamily: 'ui-monospace, Menlo, Consolas, monospace', fontSize: 13 });
		term = t;
		t.open(el);
		t.onData((s) => ondata?.(encoder.encode(s)));
		t.onResize(({ cols, rows }) => onresize?.(cols, rows));
		let observer: ResizeObserver | undefined;
		if (fit) {
			const addon = new FitAddon();
			t.loadAddon(addon);
			addon.fit();
			observer = new ResizeObserver(() => addon.fit());
			observer.observe(el);
		}
		onresize?.(t.cols, t.rows);
		return () => {
			observer?.disconnect();
			t.dispose();
			term = undefined;
		};
	}
</script>

<div class="xterm-host" data-testid="xterm" {@attach mount}></div>

<style>
	.xterm-host {
		width: 100%;
		height: 100%;
		min-height: 0;
		padding: var(--miso-sp-08);
		box-sizing: border-box;
		background: #000;
		border-radius: var(--miso-radius-08);
		overflow: hidden;
	}
</style>
