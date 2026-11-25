<script lang="ts">
	import type { TwoStep, User } from '#lib/api/index.js';
	import { useTranslator } from '#lib/i18n/index.js';
	import { Avatar, Badge } from '../ui/index.js';

	type Props = {
		user: User;
		/** Null when it could not be read: the badge is left out rather than guessed. */
		twoStep: TwoStep | null;
	};

	let { user, twoStep }: Props = $props();

	const t = useTranslator();

	const name = $derived(`${user.first_name} ${user.last_name}`.trim());
</script>

<!-- Who this account is, at a glance: the name, the address it signs in with,
     and whether that address and the second step are in place. -->
<header class="intro">
	<Avatar {user} size="lg" />
	<div class="who">
		<h1>{name || t('account.your_account')}</h1>
		<p class="email">{user.email}</p>
		<div class="badges">
			{#if user.email_verified}
				<Badge size="sm" tone="success" icon="ri-check-line">{t('profile.verified')}</Badge>
			{:else}
				<Badge size="sm" tone="warning">{t('profile.not_verified')}</Badge>
			{/if}
			{#if twoStep?.enabled}
				<Badge size="sm" tone="success" icon="ri-shield-check-line">
					{t('profile.two_step_on')}
				</Badge>
			{:else if twoStep?.offered}
				<Badge size="sm">{t('profile.two_step_off')}</Badge>
			{/if}
		</div>
	</div>
</header>

<style>
	.intro {
		display: flex;
		align-items: center;
		gap: var(--space-4);
	}

	.who {
		display: flex;
		flex-direction: column;
		gap: var(--nudge-4);
		min-width: 0;
	}

	h1 {
		margin: 0;
		font-size: var(--text-2xl);
		line-height: 1.2;
		overflow-wrap: anywhere;
	}

	.email {
		margin: 0;
		color: var(--color-text-hint);
		overflow-wrap: anywhere;
	}

	.badges {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1);
		margin-top: var(--nudge-2);
	}
</style>
