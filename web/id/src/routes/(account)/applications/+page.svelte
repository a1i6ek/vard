<script lang="ts">
	import { ConnectedApplication } from '#lib/components/account/index.js';
	import { EmptyState, PageHeading, PageTitle, Stack } from '#lib/components/ui/index.js';
	import { useTranslator } from '#lib/i18n/index.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const t = useTranslator();
</script>

<PageTitle parts={[t('apps.head'), t('account.nav')]} />

<Stack gap="lg">
	<PageHeading title={t('apps.title')} description={t('apps.subtitle')} />

	{#each data.applications as application (application.client_id)}
		<ConnectedApplication {application} timezone={data.timezone} />
	{:else}
		<EmptyState icon="ri-apps-line" size="lg" title={t('apps.empty')}>
			{t('apps.empty_body')}
		</EmptyState>
	{/each}
</Stack>
