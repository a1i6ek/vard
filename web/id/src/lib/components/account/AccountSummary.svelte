<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ConnectedApplication, Session, TwoStep } from '#lib/api/index.js';
	import { useTranslator } from '#lib/i18n/index.js';
	import type { IconName } from '../ui/icon';

	type Props = {
		/** Each null when it could not be read: its card says only where to look. */
		twoStep: TwoStep | null;
		sessions: Session[] | null;
		applications: ConnectedApplication[] | null;
	};

	let { twoStep, sessions, applications }: Props = $props();

	const t = useTranslator();

	type Card = {
		path: 'security' | 'applications';
		icon: IconName;
		title: string;
		line: string;
		tone: 'good' | 'attention' | 'quiet';
	};

	const others = $derived((sessions ?? []).filter((session) => !session.current).length);

	const cards = $derived<Card[]>([
		{
			path: 'security',
			icon: 'ri-shield-keyhole-line',
			title: t('security.two_step_title'),
			line:
				twoStep === null
					? t('overview.unknown')
					: twoStep.enabled
						? t('overview.two_step_on')
						: twoStep.offered
							? t('overview.two_step_off')
							: t('security.two_step_not_offered'),
			tone: twoStep?.enabled ? 'good' : twoStep?.offered ? 'attention' : 'quiet'
		},
		{
			path: 'security',
			icon: 'ri-computer-line',
			title: t('security.sessions_title'),
			line:
				sessions === null
					? t('overview.unknown')
					: others === 0
						? t('overview.sessions_alone')
						: t('overview.sessions_more', { count: others }),
			tone: 'quiet'
		},
		{
			path: 'applications',
			icon: 'ri-apps-line',
			title: t('apps.title'),
			line:
				applications === null
					? t('overview.unknown')
					: applications.length === 0
						? t('overview.apps_none')
						: t('overview.apps_some', {
								count: applications.length,
								names: applications
									.slice(0, 3)
									.map((app) => app.name)
									.join(', ')
							}),
			tone: 'quiet'
		}
	]);
</script>

<!-- The other sections in one line each, so the profile page says what needs
     attention without opening them. -->
<ul class="cards" aria-label={t('overview.label')}>
	{#each cards as card (card.icon)}
		<li>
			<a href={resolve(`/(account)/${card.path}`)} class="card" data-tone={card.tone}>
				<span class="icon"><i class={card.icon} aria-hidden="true"></i></span>
				<span class="text">
					<span class="title">{card.title}</span>
					<span class="line">{card.line}</span>
				</span>
				<i class="ri-arrow-right-s-line arrow" aria-hidden="true"></i>
			</a>
		</li>
	{/each}
</ul>

<style>
	.cards {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
		gap: var(--space-3);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.card {
		display: flex;
		align-items: flex-start;
		gap: var(--space-3);
		height: 100%;
		padding: var(--space-4);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-box);
		background: var(--color-surface);
		color: var(--color-text);
		transition:
			border-color var(--speed-fast),
			background-color var(--speed-fast);
	}

	.card:hover {
		border-color: var(--color-border-strong);
		background: var(--color-surface-alt);
		text-decoration: none;
	}

	.icon {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		width: 36px;
		height: 36px;
		border-radius: var(--radius-control-sm);
		background: var(--color-field);
		color: var(--color-text-hint);
		font-size: var(--icon-size);
	}

	[data-tone='good'] .icon {
		background: var(--surface-success);
		color: var(--color-success);
	}

	[data-tone='attention'] .icon {
		background: var(--surface-warning);
		color: var(--color-warning);
	}

	.text {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: var(--nudge-2);
		min-width: 0;
	}

	.title {
		font-weight: 700;
	}

	.line {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		overflow-wrap: anywhere;
	}

	.arrow {
		align-self: center;
		color: var(--color-text-hint);
		font-size: var(--icon-size);
	}
</style>
