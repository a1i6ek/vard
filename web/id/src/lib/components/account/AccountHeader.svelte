<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { signIn, type PublicLanguage, type User } from '#lib/api/index.js';
	import { useTranslator } from '#lib/i18n/index.js';
	import { displayName } from '#lib/utils/format.js';
	import { Brand, LanguagePicker, ThemeToggle } from '../layout/index.js';
	import { Avatar, Button } from '../ui/index.js';

	type Props = {
		user: User;
		languages: PublicLanguage[];
		language: string;
	};

	let { user, languages, language }: Props = $props();

	const t = useTranslator();

	let signingOut = $state(false);

	async function signOut() {
		signingOut = true;
		try {
			await signIn.logout();
		} finally {
			// One navigation that reloads the data: invalidating first would
			// make the account layout's load redirect to sign in, racing this goto.
			await goto(resolve('login'), { refreshAll: true, replace: true });
		}
	}
</script>

<header>
	<div class="bar">
		<a class="home" href={resolve('')} aria-label={t('account.your_account')}><Brand /></a>

		<div class="end">
			<LanguagePicker {languages} current={language} />
			<ThemeToggle />
			<div class="who">
				<Avatar {user} />
				<span class="text">
					<strong>{displayName(user)}</strong>
					<span>{user.email}</span>
				</span>
			</div>
			<Button
				variant="subtle"
				size="sm"
				icon="ri-logout-box-r-line"
				loading={signingOut}
				onclick={signOut}
				aria-label={t('action.sign_out')}
			>
				<span class="signout">{t('action.sign_out')}</span>
			</Button>
		</div>
	</div>
</header>

<style>
	header {
		position: sticky;
		top: 0;
		z-index: 10;
		border-bottom: 1px solid var(--color-border);
		background: color-mix(in srgb, var(--color-surface), transparent 8%);
		backdrop-filter: blur(10px);
		overscroll-behavior: none;
	}

	.bar {
		width: 100%;
		max-width: var(--account-width);
		margin: 0 auto;
		padding-left: var(--account-gutter);
		padding-right: var(--account-gutter);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		height: 64px;
	}

	.home:hover {
		text-decoration: none;
	}

	.end {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
	}

	.who {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
		padding: 0 var(--space-2);
	}

	.text {
		display: flex;
		flex-direction: column;
		min-width: 0;
		line-height: 1.25;
	}

	.text strong,
	.text span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.text span {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	@media (max-width: 40rem) {
		.text,
		.signout {
			display: none;
		}
	}
</style>
