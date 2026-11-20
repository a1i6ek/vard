<script lang="ts">
	import type { APIExchangeRule, APIIntrospector } from '#lib/api/index.js';
	import { Button, Checkbox, IconButton, Note, Select, Tag } from '#lib/components/ui/index.js';

	type Props = {
		rules: APIExchangeRule[];
		/** Every application the administrator can see, as the introspection list has them. */
		applications: APIIntrospector[];
		/** The API's scope names. */
		scopes: string[];
		editable: boolean;
		saving: boolean;
		onSave: (rules: APIExchangeRule[]) => void;
	};

	let { rules, applications, scopes, editable, saving, onSave }: Props = $props();

	const nameOf = (id: string) =>
		applications.find((app) => app.id === id)?.name ?? 'An application you cannot see';

	/** Only a client that proves who it is may act for somebody else. */
	const actors = $derived(
		applications
			.filter((app) => app.confidential)
			.map((app) => ({ value: app.id, label: app.name }))
	);
	const subjects = $derived(applications.map((app) => ({ value: app.id, label: app.name })));

	let actor = $state('');
	let subject = $state('');
	let chosen = $state<string[]>([]);

	const taken = $derived(
		rules.some((rule) => rule.application_id === actor && rule.subject_application_id === subject)
	);
	const ready = $derived(actor !== '' && subject !== '' && chosen.length > 0 && !taken);

	function toggle(scope: string, on: boolean) {
		chosen = on ? [...chosen, scope] : chosen.filter((s) => s !== scope);
	}

	function add() {
		if (!ready) return;
		onSave([
			...rules,
			{ application_id: actor, subject_application_id: subject, scopes: [...chosen].sort() }
		]);
		actor = '';
		subject = '';
		chosen = [];
	}

	function remove(index: number) {
		onSave(rules.filter((_, i) => i !== index));
	}
</script>

{#if rules.length === 0}
	<Note>No application may exchange tokens for this API.</Note>
{:else}
	<ul class="rules">
		{#each rules as rule, index (rule.application_id + rule.subject_application_id)}
			<li>
				<div class="rule">
					<span>
						<strong>{nameOf(rule.application_id)}</strong> may act for the users of
						<strong>{nameOf(rule.subject_application_id)}</strong>
					</span>
					<span class="scopes">
						{#each rule.scopes as scope (scope)}
							<Tag small><code>{scope}</code></Tag>
						{/each}
					</span>
				</div>
				{#if editable}
					<IconButton
						icon="ri-delete-bin-line"
						label="Remove the rule for {nameOf(rule.application_id)}"
						disabled={saving}
						onclick={() => remove(index)}
					/>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if editable}
	{#if actors.length === 0}
		<Note>
			No confidential application to allow. Register the service that acts as a web or machine
			application first.
		</Note>
	{:else if scopes.length === 0}
		<Note>This API has no scopes yet: add one before allowing an exchange.</Note>
	{:else}
		<form
			class="add"
			onsubmit={(event) => {
				event.preventDefault();
				add();
			}}
		>
			<div class="pickers">
				<Select label="Application acting" bind:value={actor} options={actors} />
				<Select label="For tokens issued to" bind:value={subject} options={subjects} />
			</div>
			<fieldset>
				<legend>Scopes it may carry, of what the token already holds</legend>
				<div class="choices">
					{#each scopes as scope (scope)}
						<Checkbox
							label={scope}
							checked={chosen.includes(scope)}
							onChange={(on) => toggle(scope, on)}
						/>
					{/each}
				</div>
			</fieldset>
			{#if taken}
				<Note>That pair already has a rule: remove it first to change its scopes.</Note>
			{/if}
			<div>
				<Button type="submit" icon="ri-add-line" disabled={!ready || saving} loading={saving}>
					Add rule
				</Button>
			</div>
		</form>
	{/if}
{/if}

<style>
	.rules {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	li {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-surface);
	}

	.rule {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: var(--space-1);
		min-width: 0;
	}

	.scopes {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1);
	}

	.add {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		margin-top: var(--space-3);
		padding: var(--space-3);
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-surface);
	}

	.pickers {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		gap: var(--space-3);
	}

	fieldset {
		margin: 0;
		padding: 0;
		border: 0;
	}

	legend {
		margin-bottom: var(--space-2);
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.choices {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2) var(--space-4);
	}
</style>
