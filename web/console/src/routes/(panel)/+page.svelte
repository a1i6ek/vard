<script lang="ts">
	import { goto, refreshAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating } from '$app/state';
	import type { OverviewDays } from '#lib/api/index.js';
	import { countOf } from '#lib/utils/format.js';
	import { pageTitle } from '#lib/brand.js';
	import { countDelta, rateDelta, successRate } from '#lib/components/activity/deltas.js';
	import { logsHref } from '#lib/components/activity/filters.js';
	import ActivityChart from '#lib/components/activity/ActivityChart.svelte';
	import ActivityFeed from '#lib/components/activity/ActivityFeed.svelte';
	import SignInHealth from '#lib/components/activity/SignInHealth.svelte';
	import TopActors from '#lib/components/activity/TopActors.svelte';
	import SessionList from '#lib/components/profile/SessionList.svelte';
	import {
		EmptyState,
		IconButton,
		LinkButton,
		Note,
		PageHeader,
		Panel,
		SegmentedControl,
		Skeleton,
		StatCard,
		Tag
	} from '#lib/components/ui/index.js';
	import { can, canAnywhere } from '#lib/permissions.js';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const admin = $derived(data.admin);
	const overview = $derived(data.overview);

	const ranges: { value: `${OverviewDays}`; label: string }[] = [
		{ value: '7', label: '7 days' },
		{ value: '14', label: '14 days' },
		{ value: '30', label: '30 days' },
		{ value: '90', label: '90 days' }
	];

	async function pick(days: string) {
		const path = resolve('/(panel)');
		// resolve() has already applied any base path; the query is only
		// appended to what it returned.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		await goto(days === '14' ? path : `${path}?days=${days}`, { reset: false });
	}

	const logsPage = resolve('/(panel)/logs');

	const rangeStart = $derived(overview?.daily[0]?.day ?? '');

	const links = $derived({
		day: (day: string) => logsHref(logsPage, { from: day, to: day }),
		actor: (actor: string) => logsHref(logsPage, { actor, from: rangeStart }),
		refused: logsHref(logsPage, { kind: 'refused', from: rangeStart }),
		access: logsHref(logsPage, { kind: 'access', from: rangeStart })
	});

	let refreshing = $state(false);

	async function refresh() {
		refreshing = true;
		try {
			await Promise.all([refreshAll(), new Promise((done) => setTimeout(done, 400))]);
		} finally {
			refreshing = false;
		}
	}

	const totals = $derived({
		events: overview ? overview.daily.reduce((sum, day) => sum + day.events, 0) : 0
	});

	const count = (n: number) => n.toLocaleString();

	function note(...parts: (string | false)[]) {
		return parts.filter(Boolean).join(' · ');
	}

	// A range picked while this page is open: its numbers are on their way,
	// so the panels show their shape rather than the old range's figures.
	const loading = $derived(
		navigating.to?.route.id === '/(panel)' && navigating.from?.route.id === '/(panel)'
	);

	const kpis = $derived.by(() => {
		if (!overview) return null;

		const counts = overview.counts;
		const now = overview.sign_ins;
		const before = overview.previous.sign_ins;
		const refused = now.failed + now.blocked;
		const refusedBefore = before.failed + before.blocked;
		const rate = successRate(now);
		const attempts = now.succeeded + refused;

		return {
			users: {
				note: note(
					counts.new_users > 0 && `+${count(counts.new_users)} new`,
					`${count(counts.active_users)} active`
				),
				delta: countDelta(counts.new_users, overview.previous.new_users, 'up', data.days)
			},
			rate: {
				value: rate === null ? '—' : `${rate}%`,
				note: attempts === 0 ? 'no sign-ins yet' : `of ${countOf(attempts, 'attempt')}`,
				delta: rateDelta(rate, successRate(before), data.days)
			},
			refused: {
				value: refused,
				note: note(
					counts.locked_admins > 0 &&
						`${counts.locked_admins} admin${counts.locked_admins === 1 ? '' : 's'} locked`,
					counts.locked_admins === 0 && 'wrong passwords and blocks'
				),
				delta: countDelta(refused, refusedBefore, 'down', data.days),
				tone: (counts.locked_admins > 0 ? 'warning' : 'neutral') as 'warning' | 'neutral'
			},
			sessions: note(countOf(counts.admins, 'admin')),
			applications: note(
				`${count(counts.enabled_applications)} enabled`,
				countOf(counts.user_roles, 'role')
			),
			apis: note(countOf(counts.api_scopes, 'scope'))
		};
	});
</script>

<svelte:head>
	<title>{pageTitle('Activity')}</title>
</svelte:head>

<div class="page">
	<PageHeader
		crumbs={['Dashboard', 'Activity']}
		description={overview
			? `What has been happening over the last ${data.days} days. Every number opens the entries behind it.`
			: undefined}
	>
		{#snippet secondary()}
			<IconButton
				icon="ri-refresh-line"
				label="Refresh"
				onclick={refresh}
				loading={refreshing}
				disabled={refreshing}
			/>
		{/snippet}

		{#snippet actions()}
			{#if overview}
				<SegmentedControl label="Range" options={ranges} value={`${data.days}`} onChange={pick} />
				<LinkButton href={resolve('/(panel)/logs')} variant="subtle">
					<i class="ri-file-list-3-line" aria-hidden="true"></i>
					Logs
				</LinkButton>
			{/if}
		{/snippet}
	</PageHeader>

	{#if overview && kpis}
		{@const counts = overview.counts}
		<div class="stats" aria-busy={loading}>
			<StatCard
				label="Users"
				value={counts.users}
				icon="ri-group-line"
				note={kpis.users.note}
				delta={kpis.users.delta}
				href={can(admin, 'users.read') ? resolve('/(panel)/users') : undefined}
			/>

			<StatCard
				label="Sign-in success"
				value={kpis.rate.value}
				icon="ri-shield-check-line"
				note={kpis.rate.note}
				delta={kpis.rate.delta}
				href={links.access}
			/>

			<StatCard
				label="Refused sign-ins"
				value={kpis.refused.value}
				icon="ri-error-warning-line"
				note={kpis.refused.note}
				delta={kpis.refused.delta}
				tone={kpis.refused.tone}
				href={links.refused}
			/>

			<StatCard
				label="Active sessions"
				value={counts.active_sessions}
				icon="ri-computer-line"
				note={kpis.sessions}
				href={admin.is_super_admin ? resolve('/(panel)/admins') : undefined}
			/>

			<StatCard
				label="Applications"
				value={counts.applications}
				icon="ri-apps-line"
				note={kpis.applications}
				href={canAnywhere(admin, 'applications.read')
					? resolve('/(panel)/applications')
					: undefined}
			/>

			<StatCard
				label="APIs"
				value={counts.apis}
				icon="ri-code-box-line"
				note={kpis.apis}
				href={can(admin, 'apis.read') ? resolve('/(panel)/apis') : undefined}
			/>
		</div>

		<div class="row">
			<Panel title="Activity · last {data.days} days" icon="ri-bar-chart-box-line">
				{#snippet meta()}
					<Tag>{countOf(totals.events, 'event')}</Tag>
				{/snippet}
				{#if loading}
					<Skeleton height="14rem" />
				{:else if totals.events === 0}
					<EmptyState icon="ri-bar-chart-box-line" title="Nothing happened in these days">
						Sign-ins and changes are counted here day by day.
					</EmptyState>
				{:else}
					<ActivityChart daily={overview.daily} href={links.day} />
				{/if}
			</Panel>

			<Panel title="Sign-ins · last {data.days} days" icon="ri-shield-check-line">
				{#if loading}
					<Skeleton lines={4} />
				{:else if overview.sign_ins.succeeded + overview.sign_ins.failed + overview.sign_ins.blocked === 0}
					<EmptyState icon="ri-shield-check-line" title="No sign-ins in these days">
						Every sign-in, refused or not, is counted here.
					</EmptyState>
				{:else}
					<SignInHealth
						signIns={overview.sign_ins}
						locked={counts.locked_admins}
						hrefs={{ succeeded: links.access, failed: links.refused, blocked: links.refused }}
					/>
				{/if}
			</Panel>
		</div>

		<div class="row">
			<Panel title="Recent activity" icon="ri-history-line" flush>
				{#snippet meta()}
					<LinkButton href={resolve('/(panel)/logs')} variant="ghost" size="sm">
						View all
					</LinkButton>
				{/snippet}
				<ActivityFeed events={overview.activity} />
			</Panel>

			<div class="side">
				<Panel title="Most active · last {data.days} days" icon="ri-user-star-line" flush>
					{#if loading}
						<div class="padded"><Skeleton lines={3} /></div>
					{:else}
						<TopActors actors={overview.top_actors} href={links.actor} />
					{/if}
				</Panel>

				<Panel title="Your sessions" icon="ri-computer-line" flush>
					<SessionList sessions={data.sessions} limit={4} />
				</Panel>
			</div>
		</div>
	{:else}
		<div class="narrow">
			<Panel title="Your sessions" icon="ri-computer-line" flush>
				<SessionList sessions={data.sessions} limit={8} />
			</Panel>

			<Note>
				Your roles do not include reading activity, so the numbers and the log are not shown here.
			</Note>
		</div>
	{/if}
</div>

<style>
	.page {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* As many figures to a row as fit, never so narrow they wrap. */
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(11.5rem, 1fr));
		gap: var(--space-3);
	}

	.padded {
		padding: var(--space-4);
	}

	.row {
		display: grid;
		grid-template-columns: minmax(0, 1.7fr) minmax(0, 1fr);
		align-items: stretch;
		gap: var(--space-4);
	}

	.side {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		min-width: 0;
	}

	.narrow {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		max-width: 32rem;
	}

	@media (max-width: 64rem) {
		.row {
			grid-template-columns: minmax(0, 1fr);
		}
	}

	@media (max-width: 34rem) {
		.row,
		.side,
		.page {
			gap: var(--space-3);
		}
	}
</style>
