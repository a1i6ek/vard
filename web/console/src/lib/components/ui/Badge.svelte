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

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		justify-self: start;
		gap: var(--nudge-2);
		border-radius: var(--radius-round);
		font-weight: 500;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.badge i {
		font-size: 1.1em;
	}

	.sm {
		height: 20px;
		padding: 0 var(--nudge-8);
		font-size: var(--text-xs);
	}

	.md {
		height: 24px;
		padding: 0 var(--nudge-7);
		font-size: var(--text-sm);
	}

	.neutral {
		background: var(--color-field);
		color: var(--color-text-hint);
	}

	.success {
		background: var(--surface-success);
		color: var(--color-success);
	}

	.warning {
		background: var(--surface-warning);
		color: var(--color-warning);
	}

	.danger {
		background: var(--surface-danger);
		color: var(--color-danger);
	}

	.info {
		background: var(--surface-info);
		color: var(--color-info);
	}
</style>
