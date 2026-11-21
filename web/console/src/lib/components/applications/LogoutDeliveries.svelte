<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import type { Application } from '#lib/api/index.js';
	import { Alert, IconButton, Note } from '#lib/components/ui/index.js';
	import { logoutDeliveriesOptions } from '#lib/query/index.js';
	import LogoutDeliveryList from './LogoutDeliveryList.svelte';

	type Props = { application: Application };

	let { application }: Props = $props();

	const deliveries = createQuery(() => logoutDeliveriesOptions(application.id));
</script>

<section class="logouts" aria-labelledby="logouts-title">
	<div class="heading">
		<h2 id="logouts-title">Back-channel logout</h2>
		<IconButton
			icon="ri-refresh-line"
			label="Refresh the notices"
			onclick={() => deliveries.refetch()}
		/>
	</div>
	<p class="lead">
		When a session the application signed someone in through ends, it is told, server to server. A
		notice it does not take is tried again, waiting longer each time, for a day; one it never takes
		is also in the activity log. Notices are kept for a week.
	</p>

	{#if !application.backchannel_logout_uri}
		<Note>
			This application has no back-channel logout URI, so it is not told. Set one under Settings →
			Redirects.
		</Note>
	{/if}

	{#if deliveries.isPending}
		<Note>Loading the notices…</Note>
	{:else if deliveries.isError}
		<Alert>Could not load the notices.</Alert>
	{:else}
		<LogoutDeliveryList deliveries={deliveries.data} />
	{/if}
</section>

<style>
	.logouts {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	h2 {
		margin: 0;
		font-size: var(--text-lg);
	}

	.lead {
		margin: 0 0 var(--space-2);
		max-width: 75ch;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		line-height: 1.5;
	}
</style>
