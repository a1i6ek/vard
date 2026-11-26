<script lang="ts">
	import { pageTitle } from '#lib/brand.js';
	import FlowSettings from '#lib/components/flows/FlowSettings.svelte';
	import { can } from '#lib/permissions.js';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head><title>{pageTitle(data.flow.name, 'Login flows')}</title></svelte:head>

<!-- Keyed, so moving from one flow to another starts the form afresh. -->
{#key data.flow.id}
	<FlowSettings
		flow={data.flow}
		initial={data.flow}
		taken={data.taken}
		editable={can(data.admin, 'login_flows.write')}
	/>
{/key}
