<script lang="ts">
	interface Props {
		/** shape: text line, rectangle or circle */
		variant?: 'text' | 'rect' | 'circle';
		/** css width; numbers are px */
		width?: number | string;
		/** css height; numbers are px */
		height?: number | string;
		/** number of text lines (variant="text"); last line is shortened */
		lines?: number;
	}

	let { variant = 'text', width, height, lines = 1 }: Props = $props();

	function css(value: number | string | undefined, fallback: string): string {
		if (value === undefined) return fallback;
		return typeof value === 'number' ? `${value}px` : value;
	}

	let defWidth = $derived(variant === 'circle' ? '32px' : variant === 'text' ? '140px' : '100%');
	let defHeight = $derived(variant === 'circle' ? '32px' : variant === 'text' ? '12px' : '48px');
</script>

{#if variant === 'text' && lines > 1}
	<span class="miso-skeleton-stack" style:width={css(width, '100%')}>
		{#each Array(lines), i (i)}
			<span
				class="miso-skeleton miso-skeleton--text"
				style:width={i === lines - 1 ? '60%' : '100%'}
				style:height={css(height, defHeight)}
			></span>
		{/each}
	</span>
{:else}
	<span
		class={['miso-skeleton', `miso-skeleton--${variant}`]}
		style:width={css(width, defWidth)}
		style:height={css(height, defHeight)}
	></span>
{/if}

<style>
	/* shimmer recreated from the archive: placeholder-loading, 1.5s infinite */
	.miso-skeleton {
		display: inline-block;
		flex-shrink: 0;
		color: var(--miso-graphite-700);
		border-radius: var(--miso-radius-04);
		background: linear-gradient(
			100deg,
			currentColor 20%,
			rgb(255 255 255 / 0.2) 50%,
			currentColor 80%
		);
		background-size: 200% 100%;
		animation: miso-skeleton-loading 1.5s infinite;
		opacity: 0.2;
	}

	.miso-skeleton--rect {
		border-radius: var(--miso-radius-08);
	}

	.miso-skeleton--circle {
		border-radius: var(--miso-radius-full);
	}

	.miso-skeleton-stack {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	@keyframes miso-skeleton-loading {
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}
</style>
