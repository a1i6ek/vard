<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { LogEntry } from '#lib/api/index.js';
	import { CodeBlock, CopyButton, Drawer, Tag } from '#lib/components/ui/index.js';
	import { describeUserAgent, formatDateTime, formatRelative } from '#lib/utils/format.js';
	import { describe } from './actions';
	import { actorTypeLabels } from './filters';
	import { targetHref } from './target';

	type Props = {
		/** The entry shown; none closes the drawer. */
		entry: LogEntry | null;
		onclose: () => void;
		/** What can be done from the entry: narrow the log to its actor, say. */
		actions?: Snippet<[LogEntry]>;
	};

	let { entry, onclose, actions }: Props = $props();

	// Kept while the drawer slides out, so it does not empty as it leaves.
	let shown = $state<LogEntry | null>(null);
	let open = $state(false);

	$effect(() => {
		if (entry) {
			shown = entry;
			open = true;
		} else {
			open = false;
		}
	});

	const said = $derived(shown ? describe(shown) : null);
	const link = $derived(targetHref(shown?.target));
</script>

{#if shown && said}
	<Drawer
		bind:open
		title={said.label}
		subtitle="{formatDateTime(shown.created_at)} · {formatRelative(shown.created_at)}"
		{onclose}
		actions={actions ? footer : undefined}
	>
		<p class="sentence">
			<Tag tone={said.tone} dot strong>{said.label}</Tag>
			<span>
				<b>{said.actor}</b>
				{said.verb}
				{#if said.subject}<span class:name={said.named}>{said.subject}</span>{/if}
				{said.after}
			</span>
		</p>
		{#if said.detail}<p class="detail {said.tone}">{said.detail}</p>{/if}

		<dl>
			<dt>Action</dt>
			<dd><code>{shown.action}</code><CopyButton value={shown.action} label="action" /></dd>

			<dt>Who</dt>
			<dd>
				<span>{shown.actor || '—'}</span>
				<Tag small>{actorTypeLabels[shown.actor_type]}</Tag>
			</dd>

			{#if shown.target}
				<dt>About</dt>
				<dd class="stack">
					<span>
						{#if link}
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved in targetHref -->
							<a href={link}>{shown.target.name}</a>
						{:else}
							{shown.target.name || shown.target.type}
						{/if}
						<Tag small>{shown.target.type}</Tag>
					</span>
					{#if shown.target.id}
						<span class="id">
							<code>{shown.target.id}</code><CopyButton value={shown.target.id} label="record id" />
						</span>
					{/if}
				</dd>
			{/if}

			<dt>From</dt>
			<dd class="stack">
				{#if shown.ip}
					<span class="id"
						><code>{shown.ip}</code><CopyButton value={shown.ip} label="address" /></span
					>
				{:else}
					<span>—</span>
				{/if}
				{#if shown.user_agent}
					<small title={shown.user_agent}>{describeUserAgent(shown.user_agent)}</small>
				{/if}
			</dd>

			<dt>When</dt>
			<dd><time datetime={shown.created_at}>{shown.created_at}</time></dd>

			<dt>Entry</dt>
			<dd class="id"><code>{shown.id}</code><CopyButton value={shown.id} label="entry id" /></dd>
		</dl>

		{#if shown.metadata}
			<CodeBlock title="Recorded with it" code={JSON.stringify(shown.metadata, null, 2)} />
		{/if}
	</Drawer>
{/if}

{#snippet footer()}
	{#if shown}{@render actions?.(shown)}{/if}
{/snippet}

<style>
	.sentence {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
		margin: 0;
		color: var(--color-text-hint);
	}

	.sentence b,
	.name {
		color: var(--color-text);
		font-weight: 700;
	}

	.detail {
		margin: var(--space-2) 0 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.detail.danger {
		color: var(--color-danger);
	}

	dl {
		display: grid;
		grid-template-columns: 6.5rem minmax(0, 1fr);
		gap: var(--space-3) var(--space-3);
		margin: var(--space-5) 0;
		font-size: var(--text-sm);
	}

	dt {
		color: var(--color-text-hint);
	}

	dd {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-1);
		min-width: 0;
		margin: 0;
	}

	dd.stack {
		flex-direction: column;
		align-items: flex-start;
	}

	.stack > span {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}

	.id {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		min-width: 0;
	}

	code {
		min-width: 0;
		overflow: hidden;
		font-family: var(--font-mono);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	small {
		color: var(--color-text-hint);
		font-size: var(--text-xs);
	}
</style>
