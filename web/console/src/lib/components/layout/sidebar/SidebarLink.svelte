<script lang="ts">
	import type { IconName } from '#lib/components/ui/index.js';
	import { resolve } from '$app/paths';
	import { Tooltip } from '#lib/components/ui/index.js';
	import { currentIcon, type Section } from './sections';

	type Props = {
		route: Section;
		label: string;
		icon: IconName;
		current: boolean;
		collapsed: boolean;
	};

	let { route, label, icon, current, collapsed }: Props = $props();

	const glyph = $derived(current ? currentIcon(icon) : icon);
</script>

<Tooltip {label} placement="right" disabled={!collapsed}>
	{#snippet children(trigger)}
		<a
			{...trigger()}
			href={resolve(route)}
			class="row"
			class:current
			class:collapsed
			aria-current={current ? 'page' : undefined}
		>
			<span class="icon"><i class={glyph} style:font-size="18px" aria-hidden="true"></i></span>
			<span class="label">{label}</span>
		</a>
	{/snippet}
</Tooltip>

<style>
	/* The navigation's own colours (themes.css): items read in the text
	   colour with hint icons, a quiet fill under the pointer, and the page
	   you are on drawn solid — a brand block with white words. */
	.row {
		position: relative;
		display: flex;
		align-items: center;
		gap: var(--space-2);
		height: var(--nav-item-height);
		padding: 0 var(--space-2);
		overflow: hidden;
		border-radius: var(--radius-control);
		color: var(--nav-text);
		font-size: var(--text-base);
		font-weight: 400;
		white-space: nowrap;
		text-decoration: none;
		transition:
			background-color var(--speed-fast),
			color var(--speed-fast);
	}

	.icon {
		display: inline-flex;
		flex: none;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		color: var(--nav-icon);
		transition: color var(--speed-fast);
	}

	.label {
		flex: 1 1 0;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		transition: opacity var(--speed);
	}

	.row:hover {
		background: var(--nav-hover);
	}

	.row:hover .icon {
		color: var(--nav-text);
	}

	.row:focus-visible {
		outline: 2px solid var(--color-info);
		outline-offset: -2px;
	}

	/* The page you are on: a solid brand block with white words and a white
	   icon, not a heavier weight alone, so it stands apart from every row. */
	.row.current {
		background: var(--nav-current);
		color: var(--nav-current-text);
		font-weight: 500;
	}

	.row.current .icon {
		color: var(--nav-current-text);
	}

	.row.collapsed {
		gap: 0;
		width: var(--nav-item-height);
		margin-inline: auto;
		padding: 0;
		justify-content: center;
	}

	.collapsed .label {
		flex: 0 0 0;
		opacity: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.row,
		.icon,
		.label {
			transition: none;
		}
	}
</style>
