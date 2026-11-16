<script lang="ts">
	import Button from './Button.svelte';

	interface Props {
		/** selected date (single mode) or range start */
		value?: Date | null;
		/** range end; enables range rendering when set together with value */
		endValue?: Date | null;
		/** select ranges on click */
		range?: boolean;
		/** dates rendered with the holiday tint */
		holidays?: Date[];
		/** first day of week: 1 — Monday (weekend on the right), 0 — Sunday */
		weekStart?: 0 | 1;
		onselect?: (date: Date) => void;
	}

	let {
		value = $bindable(null),
		endValue = $bindable(null),
		range = false,
		holidays = [],
		weekStart = 1,
		onselect
	}: Props = $props();

	const dayNamesMon = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
	const dayNamesSun = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
	const monthNames = [
		'January', 'February', 'March', 'April', 'May', 'June',
		'July', 'August', 'September', 'October', 'November', 'December'
	];

	let viewDate = $state(startOfMonth(new Date()));

	function startOfMonth(date: Date): Date {
		return new Date(date.getFullYear(), date.getMonth(), 1);
	}

	function sameDay(a: Date | null, b: Date | null): boolean {
		return (
			!!a &&
			!!b &&
			a.getFullYear() === b.getFullYear() &&
			a.getMonth() === b.getMonth() &&
			a.getDate() === b.getDate()
		);
	}

	const today = new Date();
	today.setHours(0, 0, 0, 0);
	// live mutes days starting from the current week
	let curWeekStart = $derived.by(() => {
		const d = new Date(today);
		d.setDate(today.getDate() - ((today.getDay() - weekStart + 7) % 7));
		return d;
	});

	// month-only grid, as the live datepicker: leading/trailing cells stay empty
	let grid = $derived.by(() => {
		const y = viewDate.getFullYear();
		const m = viewDate.getMonth();
		const offset = (new Date(y, m, 1).getDay() - weekStart + 7) % 7;
		const days = new Date(y, m + 1, 0).getDate();
		const cells: (Date | null)[] = [];
		for (let i = 0; i < offset; i++) cells.push(null);
		for (let d = 1; d <= days; d++) cells.push(new Date(y, m, d));
		while (cells.length % 7 !== 0) cells.push(null);
		return cells;
	});

	let dayNames = $derived(weekStart === 1 ? dayNamesMon : dayNamesSun);

	function isHoliday(date: Date): boolean {
		return holidays.some((h) => sameDay(h, date));
	}

	function inRange(date: Date): boolean {
		if (!range || !value || !endValue) return false;
		return date >= value && date <= endValue;
	}

	function pick(date: Date) {
		if (range) {
			if (!value || endValue) {
				value = date;
				endValue = null;
			} else if (date < value) {
				value = date;
			} else {
				endValue = date;
			}
		} else {
			value = date;
		}
		onselect?.(date);
	}

	function shiftMonth(delta: number) {
		viewDate = new Date(viewDate.getFullYear(), viewDate.getMonth() + delta, 1);
	}

	function lastWeekRow(date: Date): boolean {
		return new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate() - date.getDate() < 7;
	}
</script>

<div class="miso-calendar">
	<div class="miso-calendar__header">
		<Button scheme="g" variant="ghost" size={32} icon="chevron-left" onclick={() => shiftMonth(-1)} aria-label="Previous month" />
		<span class="miso-calendar__month"><b>{monthNames[viewDate.getMonth()]}</b> {viewDate.getFullYear()}</span>
		<Button scheme="g" variant="ghost" size={32} icon="chevron-right" onclick={() => shiftMonth(1)} aria-label="Next month" />
	</div>
	<div class="miso-calendar__grid">
		{#each dayNames as dn (dn)}
			<span class="miso-calendar__dow">{dn}</span>
		{/each}
		{#each grid as date, i (i)}
			{#if date}
				{@const day = date.getDay()}
				{@const weekend = day === 0 || day === 6}
				{@const first = date.getDate() <= 7}
				{@const last = lastWeekRow(date)}
				{@const selStart = sameDay(date, value)}
				{@const selEnd = range ? sameDay(date, endValue ?? value) : selStart}
				<button
					type="button"
					class={[
						'miso-calendar__day',
						weekend && 'miso-calendar__day--wknd',
						weekend && first && (day === 0 ? 'miso-calendar__day--wknd-tr' : 'miso-calendar__day--wknd-tl'),
						weekend && last && (day === 0 ? 'miso-calendar__day--wknd-br' : 'miso-calendar__day--wknd-bl'),
						date >= curWeekStart && 'miso-calendar__day--dim',
						isHoliday(date) && 'miso-calendar__day--holiday',
						inRange(date) && 'miso-calendar__day--range',
						selStart && 'miso-calendar__day--start',
						selEnd && 'miso-calendar__day--end',
						sameDay(date, today) && 'miso-calendar__day--today'
					]}
					onclick={() => pick(date)}
				>
					<span class="miso-calendar__num">{date.getDate()}</span>
				</button>
			{:else}
				<span class="miso-calendar__day miso-calendar__day--empty"></span>
			{/if}
		{/each}
	</div>
</div>

<style>
	.miso-calendar {
		display: inline-flex;
		flex-direction: column;
		padding: 12px 16px 16px;
		border-radius: var(--miso-radius-12);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
	}

	.miso-calendar__header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 4px;
		padding-bottom: 6px;
	}

	.miso-calendar__month {
		font: 400 1rem/20px var(--miso-font-base);
		color: var(--miso-txt-primary);
	}

	.miso-calendar__month b {
		font-weight: 600;
	}

	.miso-calendar__grid {
		display: grid;
		grid-template-columns: repeat(7, 36px);
	}

	.miso-calendar__dow {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		height: 32px;
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-tertiary);
	}

	.miso-calendar__day {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		padding: 0;
		border: 0;
		background: none;
		font: var(--miso-font-ui-14-reg);
		color: var(--miso-txt-primary);
		cursor: pointer;
	}

	.miso-calendar__day--empty {
		cursor: default;
	}

	.miso-calendar__num {
		position: relative;
		z-index: 1;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 16px;
	}

	.miso-calendar__day:not(.miso-calendar__day--empty):not(.miso-calendar__day--today):hover .miso-calendar__num {
		background: var(--miso-hlt-g-hover);
	}

	/* days from the current week on are muted, as on live */
	.miso-calendar__day--dim {
		color: var(--miso-txt-tertiary);
	}

	/* weekend band: Saturday + Sunday columns on the right */
	.miso-calendar__day--wknd {
		background: var(--miso-cont-low);
	}

	.miso-calendar__day--wknd-tl {
		border-top-left-radius: 8px;
	}

	.miso-calendar__day--wknd-tr {
		border-top-right-radius: 8px;
	}

	.miso-calendar__day--wknd-bl {
		border-bottom-left-radius: 8px;
	}

	.miso-calendar__day--wknd-br {
		border-bottom-right-radius: 8px;
	}

	.miso-calendar__day--holiday .miso-calendar__num {
		background: var(--miso-red-a200);
	}

	/* selection band */
	.miso-calendar__day--range::before,
	.miso-calendar__day--start::before,
	.miso-calendar__day--end::before {
		content: '';
		position: absolute;
		inset: 0;
		background: var(--miso-violet-a400);
	}

	.miso-calendar__day--start::before {
		border-radius: 8px 0 0 8px;
	}

	.miso-calendar__day--end::before {
		border-radius: 0 8px 8px 0;
	}

	/* single picked day: fully rounded pill */
	.miso-calendar__day--start.miso-calendar__day--end::before {
		border-radius: 16px;
	}

	/* today: dark circle */
	.miso-calendar__day--today .miso-calendar__num {
		background: var(--miso-txt-primary);
		color: rgba(255, 255, 255, 0.95);
	}

	.miso-calendar__day:focus-visible {
		outline: none;
	}

	.miso-calendar__day:focus-visible .miso-calendar__num {
		box-shadow: var(--miso-brd-focus);
	}
</style>
