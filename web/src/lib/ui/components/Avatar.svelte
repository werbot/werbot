<script lang="ts" module>
	/** palette slot — maps to `--miso-av-1` … `--miso-av-10` */
	export type AvatarColor = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10;
</script>

<script lang="ts">
	interface Props {
		/** person name — used for initials and the deterministic color */
		name?: string;
		src?: string;
		/** avatar square size in px */
		size?: number;
		/** explicit palette color 1..10; otherwise derived from name */
		color?: AvatarColor;
		/** presence dot */
		online?: boolean;
		/** explicit letters override (original av_let shows first 2 chars, e.g. "my") */
		label?: string;
	}

	let { name = '', src, size = 24, color, online = false, label }: Props = $props();

	function hashName(value: string): number {
		// FNV-1a — fewer collisions than the previous 31-hash on short Latin names
		let hash = 0x811c9dc5;
		for (let i = 0; i < value.length; i++) {
			hash ^= value.charCodeAt(i);
			hash = Math.imul(hash, 0x01000193);
		}
		return Math.abs(hash);
	}

	let slot = $derived(color ?? (name ? (((hashName(name) % 10) + 1) as AvatarColor) : 10));

	let initials = $derived(
		name
			.split(/\s+/)
			.filter(Boolean)
			.slice(0, 2)
			.map((part) => part[0]!.toUpperCase())
			.join('')
	);

	let fontSize = $derived(Math.round(size * 0.42));
</script>

<span
	class="miso-avatar"
	style:width="{size}px"
	style:height="{size}px"
	style:border-radius="{Math.round(size / 3.2)}px"
	title={name || undefined}
>
	{#if src}
		<img {src} alt={name} width={size} height={size} />
	{:else}
		<span class="miso-avatar__letters miso-avatar--c{slot}" style:font-size="{fontSize}px">
			{label ?? initials}
		</span>
	{/if}
	{#if online}
		<span class="miso-avatar__dot" style:width="{Math.max(6, size / 4)}px" style:height="{Math.max(6, size / 4)}px"></span>
	{/if}
</span>

<style>
	.miso-avatar {
		position: relative;
		display: inline-flex;
		flex-shrink: 0;
		overflow: visible;
		vertical-align: middle;
	}

	.miso-avatar img,
	.miso-avatar__letters {
		width: 100%;
		height: 100%;
		border-radius: inherit;
		object-fit: cover;
	}

	.miso-avatar__letters {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-family: var(--miso-font-base);
		font-weight: 600;
		color: #fff;
		letter-spacing: 0.02em;
		user-select: none;
	}

	.miso-avatar__dot {
		position: absolute;
		right: -1px;
		bottom: -1px;
		border-radius: var(--miso-radius-full);
		background: var(--miso-lime-500);
		box-shadow: 0 0 0 2px var(--miso-cont-top);
	}

	.miso-avatar--c1 { background: var(--miso-av-1); }
	.miso-avatar--c2 { background: var(--miso-av-2); }
	.miso-avatar--c3 { background: var(--miso-av-3); }
	.miso-avatar--c4 { background: var(--miso-av-4); }
	.miso-avatar--c5 { background: var(--miso-av-5); }
	.miso-avatar--c6 { background: var(--miso-av-6); }
	.miso-avatar--c7 { background: var(--miso-av-7); }
	.miso-avatar--c8 { background: var(--miso-av-8); }
	.miso-avatar--c9 { background: var(--miso-av-9); }
	.miso-avatar--c10 { background: var(--miso-av-10); }
</style>
