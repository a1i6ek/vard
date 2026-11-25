<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ColorPalette } from './control';

	type Props = {
		tone?: ColorPalette;
		dot?: boolean;
		small?: boolean;
		strong?: boolean;
		mono?: boolean;
		title?: string;
		/** A tag that can be taken away carries its own small remove button. */
		onremove?: () => void;
		removeLabel?: string;
		children: Snippet;
	};

	let {
		tone = 'neutral',
		dot = false,
		small = false,
		strong = false,
		mono = false,
		title,
		onremove,
		removeLabel,
		children
	}: Props = $props();
</script>

<span
	class="label {tone}"
	class:dot
	class:small
	class:strong
	class:mono
	class:removable={onremove}
	{title}
>
	{@render children()}
	{#if onremove}
		<button type="button" class="remove" aria-label={removeLabel} onclick={onremove}>
			<i class="ri-close-line" aria-hidden="true"></i>
		</button>
	{/if}
</span>

<style>
	.label {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		gap: var(--space-1);
		max-width: 100%;
		min-height: 25px;
		padding: var(--space-1) var(--nudge-7);
		border-radius: var(--radius-round);
		background: var(--color-secondary-alt);
		color: var(--color-text);
		font-size: var(--text-sm);
		font-weight: normal;
		line-height: var(--leading-2xs);
		white-space: nowrap;
	}

	.small {
		min-height: 20px;
		padding: var(--nudge-3) var(--space-1);
		font-size: var(--text-2xs);
	}

	.mono {
		font-family: var(--font-mono);
	}

	.removable {
		padding-right: var(--nudge-3);
	}

	.remove {
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		padding: 0;
		border: none;
		border-radius: var(--radius-round);
		background: transparent;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		cursor: pointer;
		transition:
			background-color var(--speed-fast),
			color var(--speed-fast);
	}

	.remove:hover {
		background: var(--surface-danger);
		color: var(--color-danger);
	}

	.strong {
		font-size: 0.8em;
		font-weight: bold;
	}

	.label:not(.dot).success {
		background: var(--surface-success);
	}

	.label:not(.dot).danger {
		background: var(--surface-danger);
	}

	.label:not(.dot).warning {
		background: var(--surface-warning);
	}

	.label:not(.dot).info {
		background: var(--surface-info);
	}

	.dot::before {
		flex-shrink: 0;
		width: 6px;
		height: 6px;
		border-radius: var(--radius-lg);
		background: var(--color-text-disabled);
		content: '';
	}

	.dot.success::before {
		background: var(--color-success);
	}

	.dot.danger::before {
		background: var(--color-danger);
	}

	.dot.warning::before {
		background: var(--color-warning);
	}

	.dot.info::before {
		background: var(--color-info);
	}
</style>
