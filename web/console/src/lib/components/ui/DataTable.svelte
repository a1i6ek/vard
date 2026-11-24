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

	let scrolled = $state(false);

	function track(event: Event) {
		scrolled = (event.currentTarget as HTMLElement).scrollLeft > 0;
	}
</script>

<div class="table-card">
	<div class="scroller" class:scrolled onscroll={track}>
		<table>
			{#if heading}
				<thead>
					<tr>
						{#if selectable}
							<th class="tick">
								<Checkbox checked={headerState} onChange={tickAll} title="Select every row" />
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
							<th class="pin"></th>
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
			<span class="empty-state">
				<span class="empty-icon"
					><i class="ri-inbox-2-line" style:font-size="20px" aria-hidden="true"></i></span
				>
				<strong>{empty}</strong>
				{#if emptyHint}<span class="empty-hint">{emptyHint}</span>{/if}
				{#if emptyAction}<span class="empty-action">{@render emptyAction()}</span>{/if}
			</span>
		</div>
	{/if}
</div>

<style>
	.table-card {
		overflow: hidden;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-surface);
		background: var(--color-surface);
		box-shadow: var(--shadow-sm);
	}

	.scroller {
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: separate;
		border-spacing: 0;
	}

	/* The cells of a row come from the caller's snippet, so they carry that
	   component's scope rather than this one's: every rule that has to reach
	   a `td` says so with :global, anchored to the table itself. One padding
	   everywhere, first column to last, so every column lines up on the same
	   rhythm. */
	th,
	table :global(td) {
		padding: 0 var(--space-4);
		text-align: left;
		vertical-align: middle;
		white-space: nowrap;
	}

	table :global(td) {
		height: 56px;
		padding-top: var(--space-2);
		padding-bottom: var(--space-2);
		border-bottom: 1px solid var(--color-border);
		background: var(--color-surface);
		color: var(--color-text);
		font-size: var(--text-base);
		transition: background-color var(--speed-fast);
	}

	table :global(tbody tr:last-child td) {
		border-bottom: none;
	}

	th {
		height: 44px;
		border-bottom: 1px solid var(--color-border-strong);
		background: var(--color-surface-alt);
		color: var(--color-text-hint);
		font-size: var(--text-xs);
		font-weight: 700;
		letter-spacing: 0.08em;
		line-height: 1;
		text-transform: uppercase;
	}

	/* A figure column's heading sits on the same digit widths as its cells,
	   so the label lines up over the numbers. */
	th.end {
		font-variant-numeric: tabular-nums;
	}

	th.grow {
		width: 100%;
	}

	th.end,
	table :global(td.end) {
		text-align: right;
	}

	/* Figures line up down a column only if every digit is as wide as the next. */
	table :global(td.end) {
		font-variant-numeric: tabular-nums;
	}

	tr.clickable {
		cursor: pointer;
	}

	tr.clickable:hover :global(td) {
		background: var(--row-hover);
	}

	tr.selected :global(td),
	tr.selected:hover :global(td) {
		background: var(--color-brand-subtle);
	}

	.tick {
		position: sticky;
		left: 0;
		z-index: 1;
		width: 1%;
		padding-right: 0;
		line-height: 0;
	}

	.tick::after {
		position: absolute;
		top: 0;
		right: 0;
		bottom: 0;
		width: 1px;
		background: var(--color-border);
		opacity: 0;
		transition: opacity var(--speed);
		content: '';
	}

	.scrolled .tick::after {
		opacity: 1;
	}

	.scrolled .tick {
		box-shadow: var(--nudge-8) 0 var(--nudge-8) calc(-1 * var(--nudge-8)) var(--shadow-edge);
	}

	.pin {
		position: sticky;
		right: 0;
		z-index: 1;
		width: 1%;
		padding-left: var(--space-1);
	}

	.pin::before {
		position: absolute;
		top: 0;
		bottom: 0;
		left: 0;
		width: 1px;
		background: var(--color-border);
		opacity: 0;
		transition: opacity var(--speed);
		content: '';
	}

	.scrolled .pin {
		box-shadow: calc(-1 * var(--nudge-8)) 0 var(--nudge-8) calc(-1 * var(--nudge-8))
			var(--shadow-edge);
	}

	.scrolled .pin::before {
		opacity: 1;
	}

	.pin :global(.control) {
		margin-left: auto;
		color: var(--color-text-disabled);
		transition:
			color var(--speed-fast),
			transform var(--speed);
	}

	tr.clickable:hover .pin :global(.control) {
		color: var(--color-text);
		transform: translateX(2px);
	}

	/* Outside the scroller: on a narrow screen the table scrolls sideways,
	   and what to do next stays in view. */
	.empty {
		padding: var(--space-6) var(--space-4);
		border-top: 1px solid var(--color-border);
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		text-align: center;
	}

	.empty-state strong {
		color: var(--color-text);
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
		border-radius: var(--radius-round);
		background: var(--color-secondary);
		color: var(--color-text-disabled);
	}
</style>
