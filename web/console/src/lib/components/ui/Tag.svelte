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

<!-- A tag, as the Atlassian Design System's: a small label on the neutral
     fill, or a tone's subtle fill; .dot puts the tone in a dot instead. -->
<style>
	.label {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		gap: var(--ds-space-050);
		max-width: 100%;
		min-height: 20px;
		padding: 0 var(--ds-space-050);
		border-radius: var(--ds-radius-small);
		background: var(--ds-background-neutral);
		color: var(--ds-text);
		font-size: var(--text-sm);
		font-weight: 400;
		line-height: var(--leading-md);
		white-space: nowrap;
	}

	.small {
		min-height: 16px;
		font-size: var(--text-xs);
		line-height: var(--leading-xs);
	}

	.mono {
		font-family: var(--font-mono);
	}

	.removable {
		padding-right: 0;
	}

	.remove {
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		padding: 0;
		border: none;
		border-radius: var(--ds-radius-small);
		background: transparent;
		color: var(--ds-icon-subtle);
		font-size: var(--text-sm);
		cursor: pointer;
		transition:
			background-color var(--speed-fast),
			color var(--speed-fast);
	}

	.remove:hover {
		background: var(--ds-background-danger-hovered);
		color: var(--ds-icon-danger);
	}

	.remove:focus-visible {
		outline: var(--ds-border-width-focused) solid var(--ds-border-focused);
		outline-offset: calc(-1 * var(--ds-border-width-focused));
	}

	.strong {
		font-size: 0.8em;
		font-weight: 700;
	}

	.label:not(.dot).success {
		background: var(--ds-background-success);
		color: var(--ds-text-success);
	}

	.label:not(.dot).danger {
		background: var(--ds-background-danger);
		color: var(--ds-text-danger);
	}

	.label:not(.dot).warning {
		background: var(--ds-background-warning);
		color: var(--ds-text-warning);
	}

	.label:not(.dot).info {
		background: var(--ds-background-information);
		color: var(--ds-text-information);
	}

	.dot::before {
		flex-shrink: 0;
		width: 6px;
		height: 6px;
		border-radius: var(--ds-radius-full);
		background: var(--ds-icon-subtlest);
		content: '';
	}

	.dot.success::before {
		background: var(--ds-icon-success);
	}

	.dot.danger::before {
		background: var(--ds-icon-danger);
	}

	.dot.warning::before {
		background: var(--ds-icon-warning);
	}

	.dot.info::before {
		background: var(--ds-icon-information);
	}
</style>
