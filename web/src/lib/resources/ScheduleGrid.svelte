<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { Checkbox } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	// Keys are weekday * 24 + hour; weekday 0 is Sunday, UTC.
	let { slots = $bindable() }: { slots: SvelteSet<number> } = $props();

	const days = [m.rules_day_0, m.rules_day_1, m.rules_day_2, m.rules_day_3, m.rules_day_4, m.rules_day_5, m.rules_day_6];

	function toggle(key: number, on: boolean) {
		if (on) slots.add(key);
		else slots.delete(key);
	}
</script>

<fieldset>
	<legend>{m.rules_schedule()}</legend>
	<p class="hint">{m.rules_schedule_hint()}</p>
	<div class="grid">
		{#each days as day, d (d)}
			<span class="day">{day()}</span>
			{#each { length: 24 } as _, h (h)}
				<Checkbox
					size={14}
					aria-label={`${day()} ${m.rules_hour({ hour: h })}`}
					checked={slots.has(d * 24 + h)}
					onchange={(e) => toggle(d * 24 + h, e.currentTarget.checked)}
				/>
			{/each}
		{/each}
	</div>
</fieldset>

<style>
	fieldset {
		border: 0;
		padding: 0;
		overflow-x: auto;
	}

	.grid {
		display: grid;
		grid-template-columns: 40px repeat(24, 16px);
		gap: 2px;
		align-items: center;
	}

	.hint {
		margin: 0 0 8px;
	}
</style>
