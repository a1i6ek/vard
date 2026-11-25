<script lang="ts">
	import { resolve } from '$app/paths';
	import { organizationApi, type Admin, type OrganizationBrand } from '#lib/api/index.js';
	import { BRAND } from '#lib/brand.js';
	import { IconButton, Logo, ThemeToggle } from '#lib/components/ui/index.js';
	import { useShell } from '#lib/state/shell.svelte.js';
	import AccountMenu from './AccountMenu.svelte';
	import CommandPalette from './CommandPalette.svelte';

	type Props = { admin: Admin; organization: OrganizationBrand };

	let { admin, organization }: Props = $props();

	const shell = useShell();

	// The organization's app logo, or nothing, in which case the header
	// draws the panel's own mark (an accent tile with its icon).
	const logo = $derived(organizationApi.logoUrl(organization.logo_version));

	const label = $derived(`${BRAND.name} console`);
</script>

<header class:mini={shell.collapsed}>
	<div class="menu-control">
		<IconButton
			icon={shell.menuOpen ? 'ri-close-line' : 'ri-menu-line'}
			label={shell.menuOpen ? 'Close the menu' : 'Open the menu'}
			size="sm"
			placement="right"
			onclick={() => shell.setMenu(!shell.menuOpen)}
		/>
	</div>

	<div class="brand-column">
		<a class="brand" href={resolve('/(panel)')} aria-label={label}>
			<Logo src={logo} icon={BRAND.icon} size="sm" />

			<span class="title">
				<span class="divider" aria-hidden="true">/</span>
				<span class="product">{BRAND.name}</span>
			</span>
		</a>
	</div>

	<div class="fold-control">
		<IconButton
			icon={shell.collapsed ? 'ri-sidebar-unfold-line' : 'ri-sidebar-fold-line'}
			label={shell.collapsed ? 'Expand the sidebar' : 'Collapse the sidebar'}
			size="sm"
			onclick={shell.toggle}
		/>
	</div>

	<div class="end">
		<CommandPalette />
		<ThemeToggle size="sm" variant="ghost" />
		<span class="rule" aria-hidden="true"></span>
		<AccountMenu {admin} />
	</div>
</header>

<style>
	header {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		z-index: 10;
		display: flex;
		align-items: center;
		gap: var(--space-2);
		height: var(--header-height);
		padding-right: var(--page-gutter);
		border-bottom: 1px solid var(--color-border);
		background: var(--color-surface);
		overscroll-behavior: none;
	}

	.brand-column {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		align-self: stretch;
		width: var(--sidebar-width);
		min-width: 0;
		overflow: hidden;
		border-right: 1px solid var(--color-border);
		background: var(--nav-surface);
	}

	.brand {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
		height: 100%;

		/* Centres the 34px logo in a folded, 56px sidebar, so it stays put. */
		padding: 0 var(--space-2) 0 var(--nudge-11);
		color: var(--color-text);
		text-decoration: none;
		white-space: nowrap;
	}

	.brand:focus-visible {
		outline: 2px solid var(--color-info);
		outline-offset: -4px;
	}

	/* The product's own name, smaller and quieter, behind a slash. */
	.title {
		display: flex;
		align-items: baseline;
		gap: var(--nudge-6);
		min-width: 0;
		line-height: 1;
	}

	.divider,
	.product {
		flex-shrink: 0;
		font-size: var(--text-2xs);
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}

	.divider {
		color: var(--color-text-disabled);
	}

	.product {
		color: var(--color-text-hint);
	}

	.end {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: var(--nudge-2);
		margin-left: auto;
	}

	.rule {
		width: 1px;
		height: 20px;
		margin: 0 var(--space-2);
		background: var(--color-border);
	}

	@media (min-width: 55.0625rem) {
		.menu-control {
			display: none;
		}

		.mini .title {
			opacity: 0;
		}
	}

	@media (max-width: 55rem) {
		header {
			gap: var(--space-1);
			padding-left: var(--space-2);
		}

		.fold-control {
			display: none;
		}

		.brand-column {
			width: auto;
			flex-shrink: 1;
			border-right-color: transparent;
			background: none;
		}

		.brand {
			gap: var(--space-2);
			padding: 0 var(--space-1);
		}

		.rule {
			margin: 0 var(--space-1);
		}
	}
</style>
