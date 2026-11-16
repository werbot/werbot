<script lang="ts">
	import Icon from './Icon.svelte';

	interface Props {
		running?: boolean;
		/** starting offset in seconds */
		seconds?: number;
		/** idle caption; running state always shows the elapsed time */
		label?: string;
		size?: number;
		onstart?: () => void;
		onstop?: (seconds: number) => void;
	}

	let {
		running = $bindable(false),
		seconds = $bindable(0),
		label = 'Start timer',
		size = 28,
		onstart,
		onstop
	}: Props = $props();

	$effect(() => {
		if (!running) return;
		const id = setInterval(() => {
			seconds += 1;
		}, 1000);
		return () => clearInterval(id);
	});

	let time = $derived.by(() => {
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = seconds % 60;
		const mm = String(m).padStart(2, '0');
		const ss = String(s).padStart(2, '0');
		return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
	});

	function toggle() {
		if (running) {
			running = false;
			onstop?.(seconds);
		} else {
			running = true;
			onstart?.();
		}
	}
</script>

<button
	type="button"
	class={['miso-timer', running && 'miso-timer--running']}
	style:height="{size}px"
	onclick={toggle}
	aria-label={running ? 'Stop timer' : 'Start timer'}
>
	{#if running}
		<!-- archive task-timers: pulsing dot, act-timer-dot 1s alternate -->
		<span class="miso-timer__dot"></span>
		<span class="miso-timer__time">{time}</span>
		<span class="miso-timer__stop"><Icon name="pause" size={14} /></span>
	{:else}
		<Icon name="play" size={14} />
		<span class="miso-timer__label">{label}</span>
	{/if}
</button>

<style>
	.miso-timer {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 0 10px;
		border: 0;
		border-radius: var(--miso-radius-07);
		background: var(--miso-hlt-g-easy);
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-primary);
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.miso-timer:hover {
		background: var(--miso-hlt-g-hover);
	}

	.miso-timer--running {
		background: var(--miso-red-50);
		color: var(--miso-txt-alert-major);
	}

	.miso-timer--running:hover {
		background: var(--miso-red-100);
	}

	.miso-timer__dot {
		width: 8px;
		height: 8px;
		border-radius: var(--miso-radius-full);
		background: var(--miso-txt-alert);
		animation: miso-timer-dot 1s ease-in-out infinite alternate;
	}

	@keyframes miso-timer-dot {
		0% {
			transform: scale(0.9);
		}
		100% {
			transform: scale(1.12);
		}
	}

	.miso-timer__time {
		font: 500 12px/16px var(--miso-font-mono);
		font-variant-numeric: tabular-nums;
	}

	.miso-timer__stop {
		display: inline-flex;
		margin-right: -2px;
	}

	.miso-timer:focus-visible {
		outline: none;
		box-shadow: var(--miso-brd-focus);
	}
</style>
