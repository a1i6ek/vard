<script lang="ts">
	import { createInfiniteQuery, useQueryClient } from '@tanstack/svelte-query';
	import { activityApi, type LogEntry } from '#lib/api/index.js';
	import { pageTitle } from '#lib/brand.js';
	import ActivityTable from '#lib/components/activity/ActivityTable.svelte';
	import LogDetail from '#lib/components/activity/LogDetail.svelte';
	import { actionsIn } from '#lib/components/activity/actions.js';
	import {
		actorTypeLabels,
		isFiltered,
		kindLabels,
		rangeLabels,
		type ActorType,
		type Kind,
		type Range
	} from '#lib/components/activity/filters.js';
	import {
		Button,
		DatePicker,
		FilterChip,
		FilterMenu,
		IconButton,
		LinkButton,
		PageHeader,
		SearchInput,
		Select,
		ShowMore,
		Toolbar,
		todayIso
	} from '#lib/components/ui/index.js';
	import { keys, logsOptions } from '#lib/query/index.js';
	import { debounce, setSearchParams } from '#lib/utils/url.js';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const queryClient = useQueryClient();

	const logs = createInfiniteQuery(() => logsOptions(data.view.filter, data.first));

	const entries = $derived(logs.data?.pages.flatMap((one) => one.logs) ?? []);
	const first = $derived(logs.data?.pages[0]);

	// What is typed in the search box, applied once the typing pauses; the
	// address is what the list follows.
	// svelte-ignore state_referenced_locally
	let search = $state(data.view.q);

	let opened = $state<LogEntry | null>(null);

	const kinds = (Object.keys(kindLabels) as Kind[])
		.filter((kind) => kind === 'refused' || actionsIn(kind).length > 0)
		.map((kind) => ({ value: kind, label: kindLabels[kind] }));

	const actorTypes = (Object.keys(actorTypeLabels) as ActorType[]).map((value) => ({
		value,
		label: actorTypeLabels[value]
	}));

	const ranges = (Object.keys(rangeLabels) as Range[]).map((value) => ({
		value,
		label: rangeLabels[value]
	}));

	const filtered = $derived(isFiltered(data.view));

	const cleared = {
		q: '',
		kind: [],
		actor: '',
		actor_type: '',
		ip: '',
		range: '',
		from: '',
		to: ''
	};

	function clear() {
		search = '';
		void setSearchParams(cleared);
	}

	const debounced = debounce(() => setSearchParams({ q: search.trim() }), 300);

	function pickRange(range: string) {
		// A preset reads back from now; custom keeps any days already chosen.
		void setSearchParams(range === 'custom' ? { range } : { range, from: '', to: '' });
	}

	function narrow(changes: Record<string, string>) {
		opened = null;
		void setSearchParams(changes);
	}

	let refreshing = $state(false);

	async function refresh() {
		refreshing = true;
		try {
			await Promise.all([
				queryClient.resetQueries({ queryKey: keys.logs.all }),
				new Promise((done) => setTimeout(done, 400))
			]);
		} finally {
			refreshing = false;
		}
	}

	const today = todayIso();

	// The first page counts what matches, up to a limit the server stops at.
	const total = $derived.by(() => {
		if (first?.total === undefined) return '';
		const words = `${first.total.toLocaleString('en')} ${first.total === 1 ? 'entry' : 'entries'}`;
		return first.total_capped ? `More than ${words}` : words;
	});
</script>

<svelte:head>
	<title>{pageTitle('Logs')}</title>
</svelte:head>

<PageHeader
	crumbs={['Dashboard', 'Logs']}
	description="Every sign-in and every change administrators made, newest first. Filters apply to the whole log, not just the page shown."
>
	{#snippet secondary()}
		<IconButton
			icon="ri-refresh-line"
			label="Refresh"
			onclick={refresh}
			loading={refreshing}
			disabled={refreshing}
		/>
	{/snippet}

	{#snippet actions()}
		<LinkButton href={activityApi.exportUrl(data.view.filter)} variant="subtle" download>
			<i class="ri-download-2-line" aria-hidden="true"></i>
			Export CSV
		</LinkButton>
	{/snippet}
</PageHeader>

<Toolbar>
	<SearchInput
		label="Search the log"
		placeholder="Search who, what, an address or an id…"
		bind:value={search}
		onsubmit={() => setSearchParams({ q: search.trim() })}
		oninput={debounced}
	/>

	<FilterMenu
		label="Kind"
		icon="ri-filter-3-line"
		options={kinds}
		value={data.view.kinds}
		onChange={(kind) => setSearchParams({ kind })}
	/>

	<div class="select">
		<Select
			compact
			label="Who"
			value={data.view.actorType}
			options={actorTypes}
			placeholder="Anyone"
			clearable
			onChange={(actor_type) => setSearchParams({ actor_type })}
		/>
	</div>

	<div class="select">
		<Select
			compact
			label="When"
			value={data.view.range}
			options={ranges}
			placeholder="Any time"
			clearable
			onChange={pickRange}
		/>
	</div>

	{#if data.view.range === 'custom'}
		<div class="dates">
			<DatePicker
				compact
				clearable
				label="From"
				value={data.view.from}
				max={data.view.to || today}
				onChange={(from) => setSearchParams({ from })}
			/>
			<DatePicker
				compact
				clearable
				label="To"
				value={data.view.to}
				min={data.view.from || undefined}
				max={today}
				onChange={(to) => setSearchParams({ to })}
			/>
		</div>
	{/if}
</Toolbar>

<!-- What the log is narrowed by, each taken away on its own. -->
{#if filtered}
	<div class="chips" role="list" aria-label="Filters in use">
		{#if data.view.q}
			<span role="listitem">
				<FilterChip
					title="Clear the search"
					onclear={() => {
						search = '';
						void setSearchParams({ q: '' });
					}}
				>
					“{data.view.q}”
				</FilterChip>
			</span>
		{/if}
		{#each data.view.kinds as kind (kind)}
			<span role="listitem">
				<FilterChip
					title="Show every kind but this"
					onclear={() => setSearchParams({ kind: data.view.kinds.filter((it) => it !== kind) })}
				>
					{kindLabels[kind]}
				</FilterChip>
			</span>
		{/each}
		{#if data.view.actorType}
			<span role="listitem">
				<FilterChip title="Show anyone" onclear={() => setSearchParams({ actor_type: '' })}>
					by {actorTypeLabels[data.view.actorType].toLowerCase()}
				</FilterChip>
			</span>
		{/if}
		{#if data.view.actor}
			<span role="listitem">
				<FilterChip title="Show everyone" onclear={() => setSearchParams({ actor: '' })}>
					by <strong>{data.view.actor}</strong>
				</FilterChip>
			</span>
		{/if}
		{#if data.view.ip}
			<span role="listitem">
				<FilterChip title="Show every address" onclear={() => setSearchParams({ ip: '' })}>
					from <code>{data.view.ip}</code>
				</FilterChip>
			</span>
		{/if}
		{#if data.view.range && (data.view.range !== 'custom' || data.view.from || data.view.to)}
			<span role="listitem">
				<FilterChip
					title="Show any time"
					onclear={() => setSearchParams({ range: '', from: '', to: '' })}
				>
					{data.view.range === 'custom'
						? `${data.view.from || 'the start'} to ${data.view.to || 'today'}`
						: rangeLabels[data.view.range].toLowerCase()}
				</FilterChip>
			</span>
		{/if}
		<Button variant="link" icon="ri-filter-off-line" onclick={clear}>Clear all</Button>
	</div>
{/if}

{#if total}
	<p class="total" aria-live="polite">{total}</p>
{/if}

<ActivityTable
	events={entries}
	onOpen={(entry) => (opened = entry)}
	empty={filtered ? 'No entries match these filters.' : 'Nothing has happened yet.'}
	emptyHint={filtered
		? 'Try other words, or clear the search and filters.'
		: 'What administrators and people do is written here as it happens.'}
>
	{#snippet emptyAction()}
		{#if filtered}
			<Button variant="subtle" size="sm" onclick={clear}>Clear search and filters</Button>
		{/if}
	{/snippet}
</ActivityTable>

{#if logs.hasNextPage}
	<ShowMore
		shown={entries.length}
		loading={logs.isFetchingNextPage}
		onclick={() => logs.fetchNextPage()}
	/>
{/if}

<LogDetail entry={opened} onclose={() => (opened = null)}>
	{#snippet actions(entry)}
		{#if entry.actor && entry.actor !== 'A user'}
			<Button
				variant="subtle"
				size="sm"
				icon="ri-user-search-line"
				onclick={() => narrow({ actor: entry.actor })}
			>
				Everything by them
			</Button>
		{/if}
		{#if entry.ip}
			<Button
				variant="subtle"
				size="sm"
				icon="ri-map-pin-line"
				onclick={() => narrow({ ip: entry.ip })}
			>
				Everything from this address
			</Button>
		{/if}
	{/snippet}
</LogDetail>

<style>
	.select {
		width: 12.5rem;
	}

	.dates {
		display: flex;
		gap: var(--space-2);
	}

	.dates > :global(*) {
		width: 12rem;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
		margin-bottom: var(--space-3);
	}

	.chips code {
		font-family: var(--font-mono);
	}

	.total {
		margin: 0 0 var(--space-2);
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		font-variant-numeric: tabular-nums;
	}

	@media (max-width: 40rem) {
		.select,
		.dates,
		.dates > :global(*) {
			width: 100%;
		}
	}
</style>
