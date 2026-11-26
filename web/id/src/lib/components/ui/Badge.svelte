<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { IconName } from './icon';

	type Props = {
		tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info';
		size?: 'sm' | 'md';
		icon?: IconName;
		children: Snippet;
	};

	let { tone = 'neutral', size = 'md', icon, children }: Props = $props();
</script>

<span class={['badge', tone, size]}>
	{#if icon}<i class={icon} aria-hidden="true"></i>{/if}
	{@render children()}
</span>

<!-- A lozenge, as the Atlassian Design System's: a short status, upper
     case, in its tone's subtle colours. -->
<style>
	.badge {
		display: inline-flex;
		align-items: center;
		justify-self: start;
		gap: var(--nudge-2);
		border-radius: var(--radius-sm);
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		text-transform: uppercase;
		white-space: nowrap;
	}

	.badge i {
		font-size: 1.1em;
	}

	.sm {
		height: 16px;
		padding: 0 var(--nudge-4);
		font-size: var(--text-2xs);
	}

	.md {
		height: 20px;
		padding: 0 var(--nudge-4);
		font-size: var(--text-xs);
	}

	.neutral {
		background: var(--ds-background-neutral);
		color: var(--ds-text-subtle);
	}

	.success {
		background: var(--ds-background-success);
		color: var(--ds-text-success);
	}

	.warning {
		background: var(--ds-background-warning);
		color: var(--ds-text-warning);
	}

	.danger {
		background: var(--ds-background-danger);
		color: var(--ds-text-danger);
	}

	.info {
		background: var(--ds-background-information);
		color: var(--ds-text-information);
	}
</style>
