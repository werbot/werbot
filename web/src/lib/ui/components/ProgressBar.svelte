<script lang="ts" module>
	export type ProgressTone = 'task' | 'money' | 'time' | 'neutral' | 'notice' | 'alert';
</script>

<script lang="ts">
	interface Props {
		/** progress 0..1 */
		value: number;
		/** overrun part 0..1 rendered in red after the value */
		overrun?: number;
		tone?: ProgressTone;
		/** bar thickness in px */
		thickness?: 4 | 6 | 8 | 10;
		/** quota bar look: 10px striped track, 4px radius, outlined fill (Payment Plan) */
		quota?: boolean;
	}

	let { value, overrun = 0, tone = 'task', thickness = 6, quota = false }: Props = $props();

	let safeValue = $derived(Math.max(0, Math.min(1, value)));
	let safeOverrun = $derived(Math.max(0, Math.min(1 - safeValue, overrun)));
</script>

<div
	class={['miso-progress', `miso-progress--${tone}`, quota && 'miso-progress--quota']}
	style:height="{quota ? 10 : thickness}px"
	role="progressbar"
	aria-valuenow={Math.round(safeValue * 100)}
	aria-valuemin={0}
	aria-valuemax={100}
>
	<div class="miso-progress__fill" style:width="{safeValue * 100}%"></div>
	{#if safeOverrun > 0}
		<div class="miso-progress__overrun" style:width="{safeOverrun * 100}%"></div>
	{/if}
</div>

<style>
	.miso-progress {
		display: flex;
		width: 100%;
		border-radius: var(--miso-radius-full);
		background: var(--miso-progbar-track);
		overflow: hidden;
	}

	.miso-progress__fill {
		border-radius: inherit;
		background: var(--fill, var(--miso-costs-task-major));
		transition: width 200ms ease;
	}

	.miso-progress__overrun {
		background: var(--miso-costs-overrun);
	}

	.miso-progress--task { --fill: var(--miso-costs-task-major); }
	.miso-progress--money { --fill: var(--miso-costs-money-major); }
	.miso-progress--time { --fill: var(--miso-costs-time-major); }
	.miso-progress--neutral { --fill: var(--miso-graphite-a400); }
	.miso-progress--notice { --fill: var(--miso-amber-a600); }
	.miso-progress--alert { --fill: var(--miso-hlt-w-focus); }

	/* Payment Plan quota bar: hatched track + outlined fill */
	.miso-progress--quota {
		border-radius: 4px;
		background: repeating-linear-gradient(
			45deg,
			var(--miso-cont-top),
			var(--miso-cont-top) 3px,
			var(--miso-cont-trans-low) 3px,
			var(--miso-cont-trans-low) 5px
		);
		overflow: visible;
	}

	.miso-progress--quota .miso-progress__fill {
		border-radius: 4px;
		box-shadow:
			0 0 0 2px var(--miso-cont-top),
			0 0 0 1px var(--miso-dark-a10) inset;
	}

	.miso-progress--quota.miso-progress--neutral {
		--fill: var(--miso-lbl-lightgrey);
	}
</style>
