<script lang="ts">
	import type { IconName } from './icon';
	import type { ColorPalette } from './control';
	import type { Delta } from './stat';

	type Props = {
		label: string;
		/** A count, or a figure already written out ("92%"). */
		value: number | string;
		icon: IconName;
		note?: string;
		tone?: ColorPalette;
		href?: string;
		delta?: Delta | null;
	};

	let { label, value, icon, note, tone = 'neutral', href, delta }: Props = $props();

	const arrows = { up: 'ri-arrow-up-line', down: 'ri-arrow-down-line', flat: 'ri-subtract-line' };
</script>

<svelte:element this={href ? 'a' : 'div'} class="stat" class:link={href} {href} data-tone={tone}>
	<span class="head">
		<i class={icon} style:font-size="15px" aria-hidden="true"></i>
		<span class="label">{label}</span>
		{#if href}
			<span class="go" aria-hidden="true"
				><i class="ri-arrow-right-s-line" style:font-size="13px" aria-hidden="true"></i></span
			>
		{/if}
	</span>

	<span class="figure">
		<strong>{typeof value === 'number' ? value.toLocaleString() : value}</strong>
		{#if delta}
			<span
				class="delta"
				data-good={delta.good === null ? undefined : String(delta.good)}
				title={delta.title}
			>
				<i class={arrows[delta.direction]} aria-hidden="true"></i>{delta.text}
				<span class="sr-only">{delta.title}</span>
			</span>
		{/if}
	</span>

	{#if note}<span class="note" title={note}>{note}</span>{/if}
</svelte:element>

<style>
	.stat {
		display: flex;
		flex-direction: column;
		gap: var(--nudge-2);
		min-width: 0;
		padding: var(--space-2) var(--space-3);
		border: var(--ds-border-width) solid var(--ds-border);
		border-radius: var(--radius-surface);
		background: var(--color-surface);
		color: var(--color-text);
		text-decoration: none;
		transition:
			background-color var(--speed),
			border-color var(--speed);
	}

	.link:hover,
	.link:focus-visible {
		outline: 0;
		border-color: var(--ds-border-bold);
		background: var(--ds-surface-hovered);
	}

	.head {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		min-width: 0;
		color: var(--color-text-hint);
	}

	.label {
		overflow: hidden;
		font-size: var(--text-sm);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.go {
		margin-left: auto;
		color: var(--color-text-disabled);
		transition: color var(--speed);
	}

	.link:hover .go {
		color: var(--color-text);
	}

	.figure {
		display: flex;
		align-items: baseline;
		gap: var(--space-2);
		min-width: 0;
	}

	strong {
		flex: none;
		font-size: var(--text-xl);
		font-weight: 700;
		line-height: 1.25;
		font-variant-numeric: tabular-nums;
	}

	.delta {
		display: inline-flex;
		flex: none;
		align-items: center;
		gap: var(--nudge-1);
		color: var(--color-text-hint);
		font-size: var(--text-xs);
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}

	.delta[data-good='true'] {
		color: var(--color-success);
	}

	.delta[data-good='false'] {
		color: var(--color-danger);
	}

	.note {
		min-width: 0;
		overflow: hidden;
		color: var(--color-text-hint);
		font-size: var(--text-xs);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.stat[data-tone='success'] .note {
		color: var(--color-success);
	}

	.stat[data-tone='warning'] .note {
		color: var(--color-warning);
	}

	.stat[data-tone='danger'] .note {
		color: var(--color-danger);
	}

	.stat[data-tone='info'] .note {
		color: var(--color-info);
	}

	@media (max-width: 34rem) {
		.stat {
			padding: var(--space-2);
		}
	}
</style>
