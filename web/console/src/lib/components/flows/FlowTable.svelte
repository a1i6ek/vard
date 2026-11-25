<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { LoginFlow } from '#lib/api/index.js';
	import { Badge, type Column, DataTable, Tag } from '#lib/components/ui/index.js';
	import { methodsOf } from './steps';

	type Props = {
		emptyHint?: string;
		emptyAction?: Snippet;
		flows: LoginFlow[];
		onOpen: (flow: LoginFlow) => void;
		empty: string;
	};

	let { flows, onOpen, empty, emptyHint, emptyAction }: Props = $props();

	const columns: Column[] = [
		{ key: 'flow', label: 'Flow', min: '14rem' },
		{ key: 'methods', label: 'Sign in with', min: '16rem', grow: true },
		{ key: 'signups', label: 'Sign-ups', min: '7rem' },
		{ key: 'applications', label: 'Applications', min: '8rem' },
		{ key: 'status', label: 'Status', min: '7rem' }
	];
</script>

<DataTable
	{columns}
	rows={flows}
	{empty}
	{emptyHint}
	{emptyAction}
	{onOpen}
	label={(flow) => `Open ${flow.name}`}
>
	{#snippet row(flow)}
		<td>
			<span class="names">
				<strong>{flow.name}</strong>
				<span class="slug">{flow.slug}</span>
			</span>
		</td>

		<td>
			<span class="methods">
				{#if !flow.allow_sign_in}
					<Badge tone="warning">Signing in closed</Badge>
				{:else}
					{#each methodsOf(flow.steps) as method (method)}
						<Tag small>{method}</Tag>
					{/each}
				{/if}
			</span>
		</td>

		<td>
			<span class:empty={!flow.allow_registration}>
				{flow.allow_registration ? 'Open' : 'Closed'}
			</span>
		</td>

		<td>
			{#if flow.applications > 0}
				{flow.applications}
			{:else if flow.is_default}
				<span class="empty">everything else</span>
			{:else}
				<span class="empty">none</span>
			{/if}
		</td>

		<td>
			{#if flow.is_default}
				<Tag tone="info" strong>Default</Tag>
			{:else if flow.is_enabled}
				<Badge tone="success">On</Badge>
			{:else}
				<Badge>Off</Badge>
			{/if}
		</td>
	{/snippet}
</DataTable>

<style>
	.names {
		display: flex;
		flex-direction: column;
		min-width: 0;
		line-height: 1.3;
	}

	.slug {
		color: var(--color-text-hint);
		font-family: var(--font-mono);
		font-size: var(--text-sm);
	}

	.methods {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1);
	}

	.empty {
		color: var(--color-text-hint);
	}
</style>
