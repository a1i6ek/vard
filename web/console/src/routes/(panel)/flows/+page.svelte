<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import type { LoginFlow } from '#lib/api/index.js';
	import { pageTitle } from '#lib/brand.js';
	import {
		Button,
		IconButton,
		LinkButton,
		PageHeader,
		SearchInput,
		ShowMore,
		Toolbar
	} from '#lib/components/ui/index.js';
	import FlowTable from '#lib/components/flows/FlowTable.svelte';
	import { can } from '#lib/permissions.js';
	import { firstPage, keys, LIST_PAGE_SIZE, loginFlowsOptions } from '#lib/query/index.js';
	import type { PageData } from './$types';
	import { debounce, setSearchParams } from '#lib/utils/url.js';

	let { data }: { data: PageData } = $props();

	/** A search or a filter is narrowing the list: an empty one then says
	    nothing matches, not that nothing exists. */
	const narrowed = $derived(Boolean(data.search));

	const queryClient = useQueryClient();

	const flows = createQuery(() =>
		loginFlowsOptions({ flows: data.flows, step_kinds: data.stepKinds })
	);

	const canWrite = $derived(can(data.admin, 'login_flows.write'));

	let search = $derived(data.search);

	const visible = $derived.by(() => {
		const term = data.search.trim().toLowerCase();
		if (term === '') return flows.data.flows;

		return flows.data.flows.filter((flow) =>
			[flow.name, flow.slug, flow.description].some((value) => value.toLowerCase().includes(term))
		);
	});

	let shown = $derived(firstPage(data.search));
	const rows = $derived(visible.slice(0, shown));

	const debounced = debounce(() => setSearchParams({ search }));

	function open(flow: LoginFlow) {
		void goto(resolve('/(panel)/flows/[id]', { id: flow.id }));
	}

	let refreshing = $state(false);

	async function refresh() {
		refreshing = true;

		try {
			await Promise.all([
				queryClient.invalidateQueries({ queryKey: keys.flows.all }),
				new Promise((done) => setTimeout(done, 400))
			]);
		} finally {
			refreshing = false;
		}
	}
</script>

<svelte:head><title>{pageTitle('Login flows')}</title></svelte:head>

<PageHeader
	crumbs={['Dashboard', 'Login flows']}
	count={flows.data.flows.length}
	description="How people sign in. Each application follows its own flow or the default one: open a flow to turn sign-ups, passwords, password resets, other accounts and two-step verification on or off."
>
	{#snippet secondary()}
		<IconButton
			icon="ri-refresh-line"
			label="Refresh the data"
			onclick={refresh}
			loading={refreshing}
			disabled={refreshing}
		/>
	{/snippet}

	{#snippet actions()}
		{#if canWrite}
			<LinkButton href={resolve('/(panel)/flows/new')} icon="ri-add-line">New flow</LinkButton>
		{/if}
	{/snippet}
</PageHeader>

<Toolbar>
	<SearchInput
		label="Search login flows"
		placeholder="Search name, identifier or description…"
		bind:value={search}
		onsubmit={() => setSearchParams({ search })}
		oninput={debounced}
	/>
</Toolbar>

<FlowTable
	flows={rows}
	onOpen={open}
	empty={narrowed ? 'No flows match this.' : 'No flows yet.'}
	emptyHint={narrowed ? 'Try other words, or clear the search and filters.' : ''}
>
	{#snippet emptyAction()}
		{#if narrowed}
			<Button variant="subtle" size="sm" onclick={() => setSearchParams({ search: '' })}>
				Clear search and filters
			</Button>
		{/if}
	{/snippet}
</FlowTable>

{#if visible.length > shown}
	<ShowMore {shown} total={visible.length} onclick={() => (shown += LIST_PAGE_SIZE)} />
{/if}
