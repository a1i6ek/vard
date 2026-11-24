<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		children: Snippet;
	};

	let { children }: Props = $props();
</script>

<kbd>{@render children()}</kbd>

<style>
	kbd {
		display: inline-grid;
		place-items: center;
		min-width: 20px;
		height: 20px;
		padding: 0 var(--space-1);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-key);
		background: var(--color-surface-alt);
		color: var(--color-text-hint);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		line-height: 1;
	}
</style>
