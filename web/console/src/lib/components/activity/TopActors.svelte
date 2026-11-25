<script lang="ts">
	import type { Overview } from '#lib/api/index.js';
	import { EmptyState, List, ListItem, Tag, Thumb } from '#lib/components/ui/index.js';
	import { initials } from '#lib/utils/format.js';

	type Props = {
		actors: Overview['top_actors'];
		href?: (actor: string) => string;
	};

	let { actors, href }: Props = $props();

	const most = $derived(Math.max(1, ...actors.map((actor) => actor.events)));

	function actorInitials(actor: string): string {
		return initials((actor.split('@')[0] ?? actor).replace(/[._-]+/g, ' '));
	}
</script>

{#if actors.length === 0}
	<div class="empty">
		<EmptyState icon="ri-user-star-line" title="No changes in this range">
			Administrators who change something are ranked here.
		</EmptyState>
	</div>
{:else}
	<List label="Most active administrators">
		{#each actors as actor (actor.actor)}
			<ListItem href={href?.(actor.actor)}>
				{#snippet lead()}<Thumb text={actorInitials(actor.actor)} />{/snippet}

				<span class="name" title={actor.actor}>{actor.actor}</span>
				<span class="track">
					<span class="fill" style:width="{(actor.events / most) * 100}%"></span>
				</span>

				{#snippet end()}<Tag strong>{actor.events.toLocaleString()}</Tag>{/snippet}
			</ListItem>
		{/each}
	</List>
{/if}

<style>
	.empty {
		padding: var(--space-3);
	}

	.name {
		overflow: hidden;
		font-size: var(--text-sm);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.track {
		display: block;
		height: 4px;
		margin-top: var(--space-1);
		overflow: hidden;
		border-radius: var(--radius-lg);
		background: var(--color-secondary);
	}

	.fill {
		display: block;
		height: 100%;
		background: var(--color-border);
	}
</style>
