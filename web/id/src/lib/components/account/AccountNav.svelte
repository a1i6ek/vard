<script module lang="ts">
	import type { IconName } from '../ui/icon';

	type Section = { path: '' | 'security' | 'applications'; key: string; icon: IconName };

	const sections: Section[] = [
		{ path: '', key: 'account.profile', icon: 'ri-user-line' },
		{ path: 'security', key: 'account.security', icon: 'ri-lock-line' },
		{ path: 'applications', key: 'account.applications', icon: 'ri-apps-line' }
	];
</script>

<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { useTranslator } from '#lib/i18n/index.js';

	const t = useTranslator();
</script>

<!-- The account's sections: a column beside the page on a wide screen, a row
     of tabs under the header on a narrow one. -->
<nav aria-label={t('account.nav')}>
	{#each sections as section (section.path)}
		<a
			href={resolve(section.path)}
			aria-current={page.url.pathname === resolve(section.path) ? 'page' : undefined}
		>
			<i class={section.icon} aria-hidden="true"></i>
			{t(section.key)}
		</a>
	{/each}
</nav>

<style>
	nav {
		position: sticky;
		top: calc(64px + var(--space-6));
		display: flex;
		flex-direction: column;
		gap: var(--nudge-2);
	}

	a {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		height: var(--control-height);
		padding: 0 var(--space-3);
		border-radius: var(--radius-control);
		color: var(--color-text-hint);
		font-weight: 500;
		white-space: nowrap;
		transition:
			background-color var(--speed-fast),
			color var(--speed-fast);
	}

	a i {
		font-size: var(--icon-size);
	}

	a:hover {
		background: var(--color-field);
		color: var(--color-text);
		text-decoration: none;
	}

	a[aria-current='page'] {
		background: var(--color-field);
		color: var(--color-text);
		font-weight: 700;
	}

	a[aria-current='page'] i {
		color: var(--color-primary);
	}

	@media (max-width: 56rem) {
		nav {
			position: static;
			flex-direction: row;
			gap: var(--space-1);
			margin: 0 calc(-1 * var(--account-gutter));
			padding: 0 var(--account-gutter);
			overflow-x: auto;
			border-bottom: 1px solid var(--color-border);
			overscroll-behavior: none;
			scrollbar-width: none;
		}

		a {
			height: auto;
			padding: var(--space-3);
			border-bottom: 2px solid transparent;
			border-radius: 0;
		}

		a:hover,
		a[aria-current='page'] {
			background: transparent;
		}

		a[aria-current='page'] {
			border-bottom-color: var(--color-primary);
		}
	}

	/* On a phone the three names fit without their icons. */
	@media (max-width: 30rem) {
		a {
			padding: var(--space-3) var(--space-2);
		}

		a i {
			display: none;
		}
	}
</style>
