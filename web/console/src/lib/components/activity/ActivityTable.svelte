<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { LogEntry } from '#lib/api/index.js';
	import { DataTable, Tag, type Column } from '#lib/components/ui/index.js';
	import { describeUserAgent, formatDateTime, formatRelative } from '#lib/utils/format.js';
	import { describe } from './actions';
	import { actorTypeLabels } from './filters';

	type Props = {
		emptyHint?: string;
		emptyAction?: Snippet;
		events: LogEntry[];
		empty?: string;
		/** Opens an entry in full. */
		onOpen?: (entry: LogEntry) => void;
	};

	let {
		events,
		empty = 'Nothing has happened yet.',
		emptyHint,
		emptyAction,
		onOpen
	}: Props = $props();

	const columns: Column[] = [
		{ key: 'event', label: 'Event', min: '13rem' },
		{ key: 'what', label: 'What happened', min: '22rem', grow: true },
		{ key: 'from', label: 'From', min: '10rem' },
		{ key: 'when', label: 'When', min: '7rem', align: 'end' }
	];

	const actorIcons = {
		admin: 'ri-shield-user-line',
		user: 'ri-user-line',
		system: 'ri-server-line'
	} as const;
</script>

<DataTable
	{columns}
	rows={events}
	{empty}
	{emptyHint}
	{emptyAction}
	{onOpen}
	label={(entry) => `Open ${describe(entry).label}, ${formatDateTime(entry.created_at)}`}
>
	{#snippet row(event)}
		{@const said = describe(event)}
		<td>
			<span class="event">
				<span class="icon {said.tone}"><i class={said.icon} aria-hidden="true"></i></span>
				<Tag tone={said.tone} small strong>{said.label}</Tag>
			</span>
		</td>
		<td>
			<span class="sentence">
				<span class="actor" title={actorTypeLabels[event.actor_type]}>
					<i class={actorIcons[event.actor_type]} aria-hidden="true"></i>
					<b>{said.actor}</b>
				</span>
				{said.verb}
				{#if said.subject}<span class:name={said.named}>{said.subject}</span>{/if}
				{said.after}
			</span>
			{#if said.detail}<span class="detail {said.tone}">{said.detail}</span>{/if}
		</td>
		<td>
			<span class="from">
				<code>{event.ip || '—'}</code>
				{#if event.user_agent}
					<small title={event.user_agent}>{describeUserAgent(event.user_agent)}</small>
				{/if}
			</span>
		</td>
		<td class="end">
			<time datetime={event.created_at} title={formatDateTime(event.created_at)}>
				{formatRelative(event.created_at)}
			</time>
		</td>
	{/snippet}
</DataTable>

<style>
	.event {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.icon {
		display: grid;
		flex: none;
		place-items: center;
		width: 26px;
		height: 26px;
		border-radius: var(--radius-control-sm);
		background: var(--color-field);
		color: var(--color-text-hint);
	}

	.icon.success {
		background: var(--surface-success);
		color: var(--color-success);
	}

	.icon.danger {
		background: var(--surface-danger);
		color: var(--color-danger);
	}

	.icon.warning {
		background: var(--surface-warning);
		color: var(--color-warning);
	}

	.sentence {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.actor {
		display: inline-flex;
		align-items: center;
		gap: var(--nudge-4);
	}

	.actor i {
		color: var(--color-text-hint);
	}

	.sentence b,
	.name {
		color: var(--color-text);
		font-weight: 700;
	}

	.detail {
		display: block;
		margin-top: var(--nudge-2);
		color: var(--color-text-hint);
		font-size: var(--text-xs);
	}

	.detail.danger {
		color: var(--color-danger);
	}

	.from {
		display: flex;
		flex-direction: column;
	}

	code {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
	}

	small {
		color: var(--color-text-hint);
		font-size: var(--text-xs);
	}

	time {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
</style>
