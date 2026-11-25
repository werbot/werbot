<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { Button } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { allKeys, dayKeys, slotKey, weekOrder, workKeys } from './schedule.js';

	let { slots = $bindable() }: { slots: SvelteSet<number> } = $props();

	const days = [m.rules_day_0, m.rules_day_1, m.rules_day_2, m.rules_day_3, m.rules_day_4, m.rules_day_5, m.rules_day_6];

	// the state the drag paints: the opposite of the hour it started on; null when not dragging
	let painting = $state<boolean | null>(null);

	function set(key: number, on: boolean) {
		if (on) slots.add(key);
		else slots.delete(key);
	}

	function start(event: PointerEvent, key: number) {
		if (event.button !== 0) return;
		painting = !slots.has(key);
		set(key, painting);
		// a touch captures the pointer to the first hour: release it so the next hours get pointerenter
		(event.currentTarget as Element).releasePointerCapture(event.pointerId);
	}

	function toggleDay(weekday: number) {
		const keys = dayKeys(weekday);
		const on = !keys.every((k) => slots.has(k));
		for (const k of keys) set(k, on);
	}

	function only(keys: number[]) {
		slots.clear();
		for (const k of keys) slots.add(k);
	}
</script>

<svelte:window onpointerup={() => (painting = null)} onpointercancel={() => (painting = null)} />

<div class="schedule">
<p class="hint">{m.rules_schedule_hint()}</p>
<div class="scroll">
	<div class="grid">
		{#each weekOrder as d (d)}
			<button type="button" class="day" onclick={() => toggleDay(d)}>{days[d]()}</button>
			{#each { length: 24 } as _, h (h)}
				{@const key = slotKey(d, h)}
				<button
					type="button"
					class={['hour', slots.has(key) && 'hour--on']}
					aria-pressed={slots.has(key)}
					aria-label={`${days[d]()} ${m.rules_hour({ hour: h })}`}
					onpointerdown={(e) => start(e, key)}
					onpointerenter={() => painting !== null && set(key, painting)}
					onclick={(e) => e.detail === 0 && set(key, !slots.has(key))}
				>
					{h}
				</button>
			{/each}
		{/each}
	</div>
</div>
<div class="wb-actions">
	<Button type="button" variant="plain" onclick={() => only(allKeys())}>{m.rules_schedule_all()}</Button>
	<Button type="button" variant="plain" onclick={() => only(workKeys())}>{m.rules_schedule_work()}</Button>
	<Button type="button" variant="plain" onclick={() => slots.clear()}>{m.rules_schedule_clear()}</Button>
</div>
</div>

<style>
	.schedule {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.scroll {
		overflow-x: auto;
	}

	.grid {
		display: grid;
		grid-template-columns: 40px repeat(24, 24px);
		gap: 3px;
		align-items: center;
		width: max-content;
		/* a drag paints hours instead of selecting text or scrolling */
		user-select: none;
		touch-action: none;
	}

	.day {
		padding: 0;
		border: 0;
		background: none;
		text-align: left;
		font: var(--miso-font-ui-13-semi);
		color: var(--miso-txt-secondary);
		cursor: pointer;
	}

	/* the PriorityBadge square: gray when off, green when on */
	.hour {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 0;
		border-radius: var(--miso-radius-04);
		background: var(--miso-prior-0);
		font: 600 12px var(--miso-font-mono);
		color: var(--miso-prior-txt);
		cursor: pointer;
		transition: background 80ms ease;
	}

	.hour--on {
		background: var(--miso-prior-5);
	}

	.hour:hover {
		filter: brightness(1.1);
	}

	.hour:focus-visible,
	.day:focus-visible {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 1px;
	}
</style>
