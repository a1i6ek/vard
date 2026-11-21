<script lang="ts">
	import { useTranslator } from '#lib/i18n/index.js';

	const t = useTranslator();
</script>

<span class="brand">
	<span class="mark"
		><i class="ri-shield-keyhole-line" style:font-size="16px" aria-hidden="true"></i></span
	>
	<span>{t('account.nav')}</span>
</span>

<style lang="scss">
	.brand {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		font-weight: 700;
		font-size: var(--text-lg);
		color: var(--color-text);
	}

	.mark {
		display: grid;
		place-items: center;
		width: 30px;
		height: 30px;
		border-radius: var(--radius-mark);
		background: var(--color-accent);
		color: var(--color-accent-text);
	}
</style>
