<script lang="ts">
	import type { LogoutDelivery } from '#lib/api/index.js';
	import { Badge, Note } from '#lib/components/ui/index.js';
	import { formatDateTime } from '#lib/utils/format.js';

	type Props = { deliveries: LogoutDelivery[] };

	let { deliveries }: Props = $props();

	const shown: Record<
		LogoutDelivery['status'],
		{ label: string; tone: 'success' | 'warning' | 'danger' }
	> = {
		delivered: { label: 'Delivered', tone: 'success' },
		pending: { label: 'Trying again', tone: 'warning' },
		failed: { label: 'Given up', tone: 'danger' }
	};

	const tries = (n: number) => (n === 1 ? '1 try' : `${n} tries`);
</script>

{#if deliveries.length === 0}
	<Note>No session this application signed someone in through has ended lately.</Note>
{:else}
	<ul class="deliveries">
		{#each deliveries as delivery (delivery.id)}
			{@const status = shown[delivery.status]}
			<li>
				<div class="what">
					<span class="when">{formatDateTime(delivery.created_at)}</span>
					<code title="Session">{delivery.session_id}</code>
					{#if delivery.status === 'delivered' && delivery.delivered_at}
						<span class="detail">
							Taken {formatDateTime(delivery.delivered_at)}, after {tries(delivery.tries)}
						</span>
					{:else if delivery.status === 'pending'}
						<span class="detail">
							{delivery.tries === 0 ? 'Not tried yet' : tries(delivery.tries)}{delivery.next_try_at
								? `; next ${formatDateTime(delivery.next_try_at)}`
								: ''}
						</span>
					{:else}
						<span class="detail">After {tries(delivery.tries)}</span>
					{/if}
					{#if delivery.last_error && delivery.status !== 'delivered'}
						<span class="error">{delivery.last_error}</span>
					{/if}
				</div>
				<Badge tone={status.tone}>{status.label}</Badge>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.deliveries {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	li {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-surface);
	}

	.what {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.when {
		font-weight: 600;
	}

	code {
		overflow-wrap: anywhere;
		color: var(--color-text-hint);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
	}

	.detail,
	.error {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.error {
		color: var(--color-danger);
	}
</style>
