<script lang="ts">
	import { AccountHeader, AccountNav } from '#lib/components/account/index.js';
	import { SecuredBy } from '#lib/components/layout/index.js';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
</script>

<div class="frame">
	<AccountHeader user={data.user} languages={data.languages} language={data.language} />

	<div class="page">
		<AccountNav />

		<main>
			{@render children()}
		</main>
	</div>

	<footer>
		<SecuredBy />
	</footer>
</div>

<style>
	.frame {
		--account-width: 68rem;
		--account-gutter: var(--space-5);

		display: grid;
		grid-template-rows: auto 1fr auto;
		min-height: 100dvh;
	}

	/* The sections beside the page: a narrow column that stays put while the
	   page scrolls. */
	.page {
		display: grid;
		grid-template-columns: 13rem minmax(0, 1fr);
		align-content: start;
		align-items: start;
		gap: var(--space-7);
		width: 100%;
		max-width: var(--account-width);
		margin: 0 auto;
		padding: var(--space-6) var(--account-gutter) var(--space-7);
	}

	main {
		min-width: 0;
	}

	footer {
		display: flex;
		justify-content: center;
		padding: var(--space-5) 0;
	}

	@media (max-width: 56rem) {
		.page {
			grid-template-columns: minmax(0, 1fr);
			gap: var(--space-5);
			padding-top: 0;
		}
	}

	@media (max-width: 40rem) {
		.frame {
			--account-gutter: var(--space-4);
		}
	}
</style>
