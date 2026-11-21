<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { apisApi, type API, type APIExchangeRule } from '#lib/api/index.js';
	import { Alert, Note, notify } from '#lib/components/ui/index.js';
	import { apiExchangeRulesOptions, apiIntrospectorsOptions, keys } from '#lib/query/index.js';
	import ExchangeRuleList from './ExchangeRuleList.svelte';

	type Props = { api: API; editable: boolean };

	let { api, editable }: Props = $props();

	const queryClient = useQueryClient();

	const rules = createQuery(() => apiExchangeRulesOptions(api.id));
	// The introspection list is every application the administrator can see.
	const applications = createQuery(() => apiIntrospectorsOptions(api.id));

	const save = createMutation(() => ({
		mutationFn: (next: APIExchangeRule[]) => apisApi.setExchangeRules(api.id, next),
		onSuccess: async (answer) => {
			queryClient.setQueryData(keys.apis.exchangeRules(api.id), answer.rules);
			notify.success('Token exchange rules saved');
			await queryClient.invalidateQueries({ queryKey: keys.apis.logs(api.id) });
		},
		onError: (err: unknown) => {
			notify.error(err, 'Could not save the token exchange rules');
		}
	}));
</script>

<section class="exchange" aria-labelledby="exchange-title">
	<h2 id="exchange-title">Token exchange</h2>
	<p class="lead">
		A service may trade a token it was handed for one to this API, acting for the same user (RFC
		8693). Only a pair listed here may: the new token carries no scope the old one lacks, never
		outlives it, and ends when it is revoked. Every exchange is in the activity log.
	</p>

	{#if rules.isPending || applications.isPending}
		<Note>Loading the rules…</Note>
	{:else if rules.isError || applications.isError}
		<Alert>Could not load the token exchange rules.</Alert>
	{:else}
		<ExchangeRuleList
			rules={rules.data}
			applications={applications.data}
			scopes={(api.scopes ?? []).map((scope) => scope.name)}
			{editable}
			saving={save.isPending}
			onSave={(next) => save.mutate(next)}
		/>
	{/if}
</section>

<style lang="scss">
	.exchange {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin-top: var(--space-6);
	}

	h2 {
		margin: 0;
		font-size: var(--text-lg);
	}

	.lead {
		margin: 0 0 var(--space-2);
		max-width: 75ch;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		line-height: 1.5;
	}
</style>
