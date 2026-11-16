<script lang="ts">
	interface Props {
		size?: number;
		/** ring thickness in px */
		thickness?: number;
	}

	let { size = 20, thickness = 2 }: Props = $props();
</script>

<span
	class="miso-spinner"
	style:width="{size}px"
	style:height="{size}px"
	style:--thickness="{thickness}px"
	role="status"
	aria-label="Loading"
></span>

<style>
	.miso-spinner {
		display: inline-block;
		flex-shrink: 0;
		border-radius: var(--miso-radius-full);
		background: conic-gradient(
			from 0deg,
			rgba(136, 207, 255, 0.85) 0deg,
			rgba(255, 255, 255, 0) 41deg,
			#ffd5f5 179deg,
			#72c6ff 352deg,
			rgba(136, 207, 255, 0.85) 360deg
		);
		mask: radial-gradient(
			farthest-side,
			transparent calc(100% - var(--thickness) - 0.5px),
			#000 calc(100% - var(--thickness))
		);
		animation: miso-spin 0.9s linear infinite;
	}

	@keyframes miso-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
