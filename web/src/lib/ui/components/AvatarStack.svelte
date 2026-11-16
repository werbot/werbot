<script lang="ts">
	import Avatar from './Avatar.svelte';
	import type { AvatarColor } from './Avatar.svelte';

	interface Person {
		name: string;
		src?: string;
		/** explicit palette color 1..10 */
		color?: AvatarColor;
	}

	interface Props {
		people: Person[];
		/** avatar size in px */
		size?: 16 | 20 | 24 | 32;
		/** max visible avatars before the +N counter */
		max?: number;
	}

	let { people, size = 24, max = 4 }: Props = $props();

	let visible = $derived(people.length > max ? people.slice(0, max) : people);
	let more = $derived(people.length - visible.length);
</script>

<span class="miso-avatar-stack" style:--overlap="{-Math.round(size / 4)}px">
	{#each visible as person, i (person.name)}
		<span class="miso-avatar-stack__item" style:z-index={i + 1}>
			<Avatar name={person.name} src={person.src} color={person.color} {size} />
		</span>
	{/each}
	{#if more > 0}
		<span
			class="miso-avatar-stack__item miso-avatar-stack__more"
			style:z-index={visible.length + 1}
			style:width="{size}px"
			style:height="{size}px"
			style:border-radius="{Math.round(size / 3.2)}px"
			style:font-size="{Math.round(size * 0.38)}px"
		>
			+{more}
		</span>
	{/if}
</span>

<style>
	.miso-avatar-stack {
		display: inline-flex;
		align-items: center;
	}

	.miso-avatar-stack__item {
		position: relative;
		display: inline-flex;
		border-radius: var(--miso-radius-full);
		box-shadow: 0 0 0 2px var(--miso-cont-top);
	}

	.miso-avatar-stack__item + .miso-avatar-stack__item {
		margin-left: var(--overlap);
	}

	.miso-avatar-stack__more {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		box-sizing: border-box;
		/* solid fill — translucent --miso-hlt-g-selected vanishes on white and
		   looks like the square is only half-painted */
		background: var(--miso-cont-low);
		color: var(--miso-txt-secondary);
		font-family: var(--miso-font-base);
		font-weight: 600;
	}
</style>
