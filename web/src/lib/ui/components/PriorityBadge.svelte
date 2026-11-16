<script lang="ts">
	interface Props {
		/** priority level 0..10 */
		value: number;
		/** badge size in px */
		size?: 16 | 20 | 24;
	}

	let { value, size = 20 }: Props = $props();

	let level = $derived(Math.max(0, Math.min(10, Math.round(value))));
</script>

<span
	class="miso-prior miso-prior--{level}"
	style:width="{size}px"
	style:height="{size}px"
	style:font-size="{Math.round(size * 0.55)}px"
	title="Priority {level}"
>
	{level}
</span>

<style>
	.miso-prior {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border-radius: var(--miso-radius-04);
		font-family: var(--miso-font-mono);
		font-weight: 600;
		color: var(--miso-prior-txt);
		user-select: none;
		vertical-align: middle;
	}

	.miso-prior--0 { background: var(--miso-prior-0); }
	.miso-prior--1 { background: var(--miso-prior-1); color: var(--miso-txt-secondary); }
	.miso-prior--2 { background: var(--miso-prior-2); }
	.miso-prior--3 { background: var(--miso-prior-3); }
	.miso-prior--4 { background: var(--miso-prior-4); }
	.miso-prior--5 { background: var(--miso-prior-5); }
	.miso-prior--6 { background: var(--miso-prior-6); }
	.miso-prior--7 { background: var(--miso-prior-7); }
	.miso-prior--8 { background: var(--miso-prior-8); }
	.miso-prior--9 { background: var(--miso-prior-9); }
	.miso-prior--10 { background: var(--miso-prior-10); }
</style>
