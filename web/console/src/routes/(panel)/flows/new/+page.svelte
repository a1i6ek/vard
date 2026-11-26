<script lang="ts">
	import { pageTitle } from '#lib/brand.js';
	import FlowSettings from '#lib/components/flows/FlowSettings.svelte';
	import { NEW_FLOW } from '#lib/components/flows/steps.js';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head><title>{pageTitle('New flow', 'Login flows')}</title></svelte:head>

<FlowSettings flow={null} initial={NEW_FLOW} taken={data.taken} editable />
