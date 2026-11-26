<script lang="ts" generics="Row extends { id: string }">
	import type { Snippet } from 'svelte';
	import Checkbox from './Checkbox.svelte';
	import IconButton from './IconButton.svelte';
	import type { Column } from './table';

	type Props = {
		columns: Column[];
		rows: Row[];
		/** What the table says with no rows: that nothing exists yet, or that
		    nothing matches what is searched for. */
		empty: string;
		/** A sentence under it saying what would fill the table. */
		emptyHint?: string;
		/** The next step: make the first one, or clear the search. */
		emptyAction?: Snippet;
		row: Snippet<[Row]>;
		onOpen?: (row: Row) => void;
		label?: (row: Row) => string;
		selected?: string[];
		onSelect?: (ids: string[]) => void;
	};

	let {
		columns,
		rows,
		empty,
		emptyHint,
		emptyAction,
		row,
		onOpen,
		label,
		selected,
		onSelect
	}: Props = $props();

	const selectable = $derived(selected !== undefined && onSelect !== undefined);
	const ticked = $derived(new Set(selected ?? []));
	const allTicked = $derived(rows.length > 0 && rows.every((item) => ticked.has(item.id)));

	const headerState = $derived<boolean | 'indeterminate'>(
		allTicked ? true : rows.some((item) => ticked.has(item.id)) ? 'indeterminate' : false
	);

	function tickAll(checked: boolean) {
		onSelect?.(checked ? rows.map((item) => item.id) : []);
	}

	function tick(id: string, checked: boolean) {
		const ids = selected ?? [];

		onSelect?.(checked ? [...ids, id] : ids.filter((it) => it !== id));
	}

	const heading = $derived(columns.some((column) => column.label));

	let scroller = $state<HTMLElement>();
	let table = $state<HTMLTableElement>();

	/** Whether more of the table lies past either edge, so the edges say so. */
	let moreBefore = $state(false);
	let moreAfter = $state(false);

	function measure() {
		if (!scroller) return;
		const { scrollLeft, scrollWidth, clientWidth } = scroller;
		moreBefore = scrollLeft > 1;
		moreAfter = scrollLeft + clientWidth < scrollWidth - 1;
	}

	$effect(() => {
		if (!scroller || !table) return;
		const observer = new ResizeObserver(measure);
		observer.observe(scroller);
		observer.observe(table);
		return () => observer.disconnect();
	});

	/* The cells come from the caller's snippet. Each is told its column's name,
	   which the narrow layout shows beside the value, and every part keeps its
	   table role, which a layout that is not a grid would otherwise drop. */
	$effect(() => {
		const labels = columns.map((column) => column.label ?? '');
		void rows.length;
		if (!table) return;

		table.setAttribute('role', 'table');
		for (const group of table.querySelectorAll(':scope > thead, :scope > tbody')) {
			group.setAttribute('role', 'rowgroup');
		}
		for (const line of table.querySelectorAll(':scope > * > tr')) {
			line.setAttribute('role', 'row');
		}
		for (const head of table.querySelectorAll(':scope > thead th')) {
			head.setAttribute('role', 'columnheader');
		}
		for (const line of table.querySelectorAll(':scope > tbody > tr')) {
			let index = 0;
			for (const cell of line.children) {
				cell.setAttribute('role', 'cell');
				if (cell.classList.contains('tick') || cell.classList.contains('pin')) continue;
				(cell as HTMLElement).dataset.label = labels[index] ?? '';
				index += 1;
			}
		}
		measure();
	});
</script>

<div
	class="table-card"
	class:selectable
	class:openable={onOpen !== undefined}
	class:more-before={moreBefore}
	class:more-after={moreAfter}
>
	<div class="scroller" bind:this={scroller} onscroll={measure}>
		<table bind:this={table}>
			{#if heading}
				<thead>
					<tr>
						{#if selectable}
							<th class="tick">
								<Checkbox checked={headerState} onChange={tickAll} label="Select every row" />
							</th>
						{/if}

						{#each columns as column (column.key)}
							<th
								scope="col"
								style:min-width={column.min}
								class:end={column.align === 'end'}
								class:grow={column.grow}
							>
								{column.label}
							</th>
						{/each}
						{#if onOpen}
							<th class="pin"><span class="sr-only">Open</span></th>
						{/if}
					</tr>
				</thead>
			{/if}

			<tbody>
				{#each rows as item (item.id)}
					<tr
						class:clickable={onOpen !== undefined}
						class:selected={ticked.has(item.id)}
						onclick={() => onOpen?.(item)}
					>
						{#if selectable}
							<td class="tick" onclick={(event) => event.stopPropagation()}>
								<Checkbox
									checked={ticked.has(item.id)}
									onChange={(checked) => tick(item.id, checked)}
									title="Select this row"
								/>
							</td>
						{/if}

						{@render row(item)}

						{#if onOpen}
							<td class="pin">
								<IconButton
									icon="ri-arrow-right-s-line"
									label={label?.(item) ?? 'Open'}
									size="sm"
									placement="left"
								/>
							</td>
						{/if}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if rows.length === 0}
		<div class="empty">
			<span class="empty-icon"><i class="ri-inbox-2-line" aria-hidden="true"></i></span>
			<strong>{empty}</strong>
			{#if emptyHint}<span class="empty-hint">{emptyHint}</span>{/if}
			{#if emptyAction}<span class="empty-action">{@render emptyAction()}</span>{/if}
		</div>
	{/if}
</div>

<!-- The Atlassian Design System's table, refined for the panel: a sunken
     header, 48px rows, see-through tints for hover and selection laid over a
     solid surface, a sticky tick column and open arrow, and edge shadows
     where more of the table lies out of sight. Narrower than 40rem — a
     phone, a dialog — each row becomes a card of label and value pairs. -->
<style>
	.table-card {
		position: relative;
		overflow: hidden;
		border: var(--ds-border-width) solid var(--ds-border);
		border-radius: var(--radius-surface);
		background: var(--ds-surface);
		container: data-table / inline-size;
	}

	/* Fades at an edge with more beyond it, where no sticky column already
	   marks the edge with its shadow. */
	.table-card::before,
	.table-card::after {
		position: absolute;
		top: 0;
		bottom: 0;
		z-index: 2;
		width: var(--space-5);
		opacity: 0;
		pointer-events: none;
		transition: opacity var(--speed);
		content: '';
	}

	.table-card::before {
		left: 0;
		background: linear-gradient(to right, var(--ds-surface), transparent);
	}

	.table-card::after {
		right: 0;
		background: linear-gradient(to left, var(--ds-surface), transparent);
	}

	.table-card.more-before:not(.selectable)::before,
	.table-card.more-after:not(.openable)::after {
		opacity: 1;
	}

	.scroller {
		overflow-x: auto;
		overscroll-behavior-x: contain;
	}

	table {
		width: 100%;
		border-collapse: separate;
		border-spacing: 0;
		font-size: var(--text-base);
	}

	/* The cells of a row come from the caller's snippet and carry its scope,
	   so every rule that reaches a td says so with :global, anchored here. */
	th,
	table :global(td) {
		padding: 0 var(--space-3);
		text-align: left;
		vertical-align: middle;
		white-space: nowrap;
	}

	th:first-child,
	table :global(td:first-child) {
		padding-left: var(--space-4);
	}

	th:last-child,
	table :global(td:last-child) {
		padding-right: var(--space-4);
	}

	th {
		position: relative;
		height: 40px;
		border-bottom: var(--ds-border-width) solid var(--ds-border);
		background: var(--ds-surface-sunken);
		color: var(--ds-text-subtle);
		font-size: var(--text-xs);
		font-weight: 700;
		line-height: var(--leading-xs);
	}

	th.grow {
		width: 100%;
	}

	th.end,
	table :global(td.end) {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}

	/* Hover and selection are see-through tints, laid over the surface rather
	   than in place of it, so sticky cells stay solid and nothing scrolling
	   under them shows through. */
	tbody tr {
		--row-tint: transparent;
	}

	table :global(td) {
		height: 48px;
		padding-top: var(--space-1);
		padding-bottom: var(--space-1);
		border-bottom: var(--ds-border-width) solid var(--ds-border);
		background: linear-gradient(var(--row-tint), var(--row-tint)), var(--ds-surface);
		color: var(--ds-text);
		transition: background-color var(--speed-fast);
	}

	table :global(tbody tr:last-child td) {
		border-bottom: none;
	}

	tr.clickable {
		cursor: pointer;
	}

	tr.clickable:hover {
		--row-tint: var(--ds-background-neutral-subtle-hovered);
	}

	tr.selected {
		--row-tint: var(--ds-background-selected);
	}

	tr.selected:hover {
		--row-tint: var(--ds-background-selected-hovered);
	}

	/* A selected row is marked at its edge too, not by its tint alone. */
	tr.selected > :global(td:first-child) {
		box-shadow: inset var(--ds-border-width-selected) 0 0 var(--ds-border-selected);
	}

	/* ------------------------------------------------ sticky ends of a row */

	.tick,
	.pin {
		position: sticky;
		z-index: 1;
		width: 1%;
		transition: box-shadow var(--speed);
	}

	.tick {
		left: 0;
		padding-right: var(--space-2);
		line-height: 0;
	}

	.pin {
		right: 0;
		padding-left: var(--space-1);
	}

	/* Atlassian's overflow shadow, and a hairline, where content runs under
	   a sticky end. */
	.more-before .tick {
		box-shadow:
			var(--nudge-8) 0 var(--nudge-8) calc(-1 * var(--nudge-8)) var(--ds-shadow-overflow-spread),
			var(--ds-border-width) 0 0 var(--ds-border);
	}

	.more-after .pin {
		box-shadow:
			calc(-1 * var(--nudge-8)) 0 var(--nudge-8) calc(-1 * var(--nudge-8))
				var(--ds-shadow-overflow-spread),
			calc(-1 * var(--ds-border-width)) 0 0 var(--ds-border);
	}

	/* The select-all box names itself only where the column names are gone. */
	th.tick :global([data-part='label']) {
		display: none;
	}

	tr.selected > .tick {
		box-shadow: inset var(--ds-border-width-selected) 0 0 var(--ds-border-selected);
	}

	.pin :global(.control) {
		margin-left: auto;
		color: var(--ds-icon-subtlest);
		transition:
			color var(--speed-fast),
			transform var(--speed);
	}

	tr.clickable:hover .pin :global(.control) {
		color: var(--ds-icon);
		transform: translateX(2px);
	}

	/* ----------------------------------------------------------- nothing */

	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-6) var(--space-4);
		border-top: var(--ds-border-width) solid var(--ds-border);
		color: var(--ds-text-subtle);
		font-size: var(--text-sm);
		text-align: center;
	}

	.empty strong {
		color: var(--ds-text);
		font-size: var(--text-base);
		font-weight: 700;
	}

	.empty-hint {
		max-width: 32rem;
	}

	.empty-action {
		margin-top: var(--space-1);
	}

	.empty-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 40px;
		height: 40px;
		border-radius: var(--ds-radius-medium);
		background: var(--ds-background-neutral);
		color: var(--ds-icon-subtle);
		font-size: var(--icon-lg);
	}

	/* -------------------------------------------------- narrow: cards */

	@container data-table (max-width: 40rem) {
		table,
		tbody {
			display: block;
		}

		/* The column names move into each card, so the heading shows only the
		   select-all box, when there is one. */
		thead {
			display: block;
		}

		thead tr {
			display: flex;
			align-items: center;
			min-height: 40px;
			border-bottom: var(--ds-border-width) solid var(--ds-border);
			background: var(--ds-surface-sunken);
		}

		thead th {
			display: none;
		}

		thead th.tick {
			display: block;
			position: static;
			height: auto;
			padding: 0 var(--space-4);
			border: none;
			box-shadow: none;
			line-height: var(--leading-md);
		}

		th.tick :global([data-part='label']) {
			display: inline;
			color: var(--ds-text-subtle);
			font-size: var(--text-xs);
			font-weight: 700;
		}

		.table-card:not(.selectable) thead {
			display: none;
		}

		tbody tr {
			position: relative;
			display: block;
			padding: var(--space-3) var(--space-4);
			border-bottom: var(--ds-border-width) solid var(--ds-border);
			background: linear-gradient(var(--row-tint), var(--row-tint)), var(--ds-surface);
		}

		tbody tr:last-child {
			border-bottom: none;
		}

		.selectable tbody tr {
			padding-left: calc(var(--space-4) + var(--icon-md) + var(--space-3));
		}

		.openable tbody tr {
			padding-right: calc(var(--space-4) + var(--control-height-sm));
		}

		tr.selected {
			box-shadow: inset var(--ds-border-width-selected) 0 0 var(--ds-border-selected);
		}

		table :global(td),
		table :global(td:first-child),
		table :global(td:last-child) {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: var(--space-3);
			height: auto;
			min-height: 28px;
			padding: var(--nudge-2) 0;
			border: none;
			background: none;
			box-shadow: none;
			text-align: right;
			white-space: normal;
			overflow-wrap: anywhere;
		}

		table :global(td)::before {
			flex: none;
			max-width: 45%;
			color: var(--ds-text-subtle);
			font-size: var(--text-xs);
			font-weight: 700;
			text-align: left;
			content: attr(data-label);
		}

		table :global(td[data-label=''])::before {
			display: none;
		}

		table :global(td[data-label='']) {
			justify-content: flex-start;
		}

		td.tick,
		td.pin {
			position: absolute;
			width: auto;
			min-height: 0;
			padding: 0;
			box-shadow: none;
		}

		td.tick {
			top: calc(var(--space-3) + var(--nudge-6));
			left: var(--space-4);
		}

		td.pin {
			top: 50%;
			right: var(--space-2);
			transform: translateY(-50%);
		}

		td.tick::before,
		td.pin::before {
			display: none;
		}

		.table-card::before,
		.table-card::after {
			display: none;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		table :global(td),
		.pin :global(.control) {
			transition: none;
		}
	}
</style>
