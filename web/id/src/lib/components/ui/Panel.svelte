<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { IconName } from './icon';

	type Props = {
		title: string;
		description?: string;
		icon?: IconName;
		/** At the header's right end: a count, a status, an action. */
		meta?: Snippet;
		/** A bar under the content for its actions, Save last. */
		footer?: Snippet;
		/** The content runs edge to edge: a list or a table. */
		flush?: boolean;
		children: Snippet;
	};

	let { title, description, icon, meta, footer, flush = false, children }: Props = $props();

	const uid = $props.id();
</script>

<!-- A flat surface with a hairline edge: a header saying what it is, the
     content, and the actions in a quieter bar under it. -->
<section class="panel" aria-labelledby="panel-{uid}">
	<header>
		{#if icon}<i class={icon} aria-hidden="true"></i>{/if}
		<div class="heading">
			<h2 id="panel-{uid}">{title}</h2>
			{#if description}<p>{description}</p>{/if}
		</div>
		{#if meta}<div class="meta">{@render meta()}</div>{/if}
	</header>

	<div class={['body', { flush }]}>
		{@render children()}
	</div>

	{#if footer}
		<footer>{@render footer()}</footer>
	{/if}
</section>

<style>
	.panel {
		display: flex;
		flex-direction: column;
		min-width: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-surface);
		background: var(--color-surface);
		box-shadow: var(--shadow-card);
	}

	header {
		display: flex;
		align-items: flex-start;
		gap: var(--space-2);
		padding: var(--space-4) var(--space-5) 0;
	}

	header > i {
		margin-top: var(--nudge-2);
		color: var(--color-text-hint);
		font-size: var(--text-lg);
	}

	.heading {
		flex: 1;
		min-width: 0;
	}

	h2 {
		margin: 0;
		overflow: hidden;
		font-size: var(--text-lg);
		font-weight: 700;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.heading p {
		margin: var(--space-1) 0 0;
		color: var(--color-text-hint);
	}

	.meta {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		gap: var(--space-1);
	}

	.body {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: var(--space-4);
		min-width: 0;
		padding: var(--space-4) var(--space-5) var(--space-5);
	}

	/* A list or a table meets the edges, set off from the header by a
	   hairline. */
	.body.flush {
		gap: 0;
		margin-top: var(--space-4);
		padding: 0;
		border-top: 1px solid var(--color-border);
	}

	.body.flush:last-child {
		overflow: hidden;
		border-radius: 0 0 var(--radius-surface) var(--radius-surface);
	}

	footer {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-5);
		border-top: 1px solid var(--color-border);
		border-radius: 0 0 var(--radius-surface) var(--radius-surface);
		background: var(--color-surface-alt);
	}

	@media (max-width: 30rem) {
		header,
		.body:not(.flush) {
			padding-right: var(--space-4);
			padding-left: var(--space-4);
		}

		footer {
			padding: var(--space-3) var(--space-4);
		}

		footer :global(.control) {
			flex: 1;
		}
	}
</style>
