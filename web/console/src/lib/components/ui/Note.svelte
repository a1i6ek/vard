<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		/** Set apart on a quiet fill: a remark beside fields rather than under them. */
		boxed?: boolean;
		align?: 'start' | 'center';
		size?: 'sm' | 'md';
		children: Snippet;
	};

	let { boxed = false, align = 'start', size = 'sm', children }: Props = $props();
</script>

<p class={['note', align, size, { boxed }]}>{@render children()}</p>

<style>
	.note {
		margin: 0;
		color: var(--color-text-hint);
		line-height: 1.5;
	}

	.sm {
		font-size: var(--text-sm);
	}

	.md {
		font-size: var(--text-base);
	}

	.center {
		text-align: center;
	}

	.boxed {
		padding: var(--space-3);
		border-radius: var(--radius-box);
		background: var(--color-secondary-alt);
	}

	.note :global(a),
	.note :global(strong) {
		color: var(--color-text);
	}

	/* An action that belongs to the sentence, such as undoing what it says. */
	.note :global(button) {
		margin-left: var(--space-1);
		padding: 0;
		border: none;
		background: none;
		color: var(--color-text);
		font: inherit;
		text-decoration: underline;
		text-underline-offset: 3px;
		cursor: pointer;
	}
</style>
