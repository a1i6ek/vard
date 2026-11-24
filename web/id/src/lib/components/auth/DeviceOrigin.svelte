<script lang="ts">
	import type { DevicePage } from '#lib/api/index.js';
	import { useTranslator } from '#lib/i18n/index.js';
	import { describeDevice, timeAgo } from '#lib/utils/format.js';

	type Props = { origin: DevicePage['requested_from'] };

	let { origin }: Props = $props();

	const t = useTranslator();
</script>

<!-- Where the code was started, so one somebody else started — and read out
     to the person to get them to allow it — reads as somebody else's. -->
<section class="origin" aria-labelledby="origin-title">
	<h2 id="origin-title">{t('device.origin_title')}</h2>
	<dl>
		<dt>{t('device.origin_device')}</dt>
		<dd>{describeDevice(origin.user_agent, t)}</dd>
		{#if origin.ip}
			<dt>{t('device.origin_address')}</dt>
			<dd><code>{origin.ip}</code></dd>
		{/if}
		<dt>{t('device.origin_when')}</dt>
		<dd>{timeAgo(origin.at, t)}</dd>
	</dl>
	<p>{t('device.origin_warning')}</p>
</section>

<style>
	.origin {
		display: grid;
		gap: var(--space-2);
		padding: var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-box);
		background: var(--color-surface-alt);
	}

	h2 {
		margin: 0;
		font-size: var(--text-sm);
	}

	dl {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: var(--nudge-4) var(--space-3);
		margin: 0;
		font-size: var(--text-sm);
	}

	dt {
		color: var(--color-text-hint);
	}

	dd {
		margin: 0;
		overflow-wrap: anywhere;
	}

	p {
		margin: 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}
</style>
