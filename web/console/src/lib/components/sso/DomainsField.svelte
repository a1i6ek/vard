<script lang="ts">
	import { Button, Input, Tag } from '#lib/components/ui/index.js';

	type Props = {
		domains: string[];
		readOnly?: boolean;
	};

	let { domains = $bindable([]), readOnly = false }: Props = $props();

	let typed = $state('');

	function normalize(domain: string): string {
		return domain.trim().toLowerCase().replace(/^@/, '').replace(/\.$/, '');
	}

	function add() {
		for (const piece of typed.split(/[\s,;]+/)) {
			const domain = normalize(piece);
			if (domain && !domains.includes(domain)) domains = [...domains, domain];
		}
		typed = '';
	}

	function remove(domain: string) {
		domains = domains.filter((one) => one !== domain);
	}

	/** Enter adds rather than submitting the dialog's form. */
	function keydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			add();
		}
	}
</script>

<div class="field">
	{#if domains.length > 0}
		<ul class="chips">
			{#each domains as domain (domain)}
				<li>
					<Tag
						mono
						onremove={readOnly ? undefined : () => remove(domain)}
						removeLabel="Remove {domain}"
					>
						{domain}
					</Tag>
				</li>
			{/each}
		</ul>
	{/if}

	{#if !readOnly}
		<div class="add">
			<Input
				label="Add a domain"
				bind:value={typed}
				placeholder="acme.com"
				hint="People with an address at these domains sign in through this connection, and it signs in nobody else. Leave it empty to let the provider make accounts at any address but those at another connection's domains, never linking to one that exists; people then reach it through its button only."
				onkeydown={keydown}
				autocomplete="off"
				spellcheck={false}
			/>
			<Button variant="subtle" onclick={add} disabled={typed.trim() === ''}>
				<i class="ri-add-line" aria-hidden="true"></i>
				Add
			</Button>
		</div>
	{/if}
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.add {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: start;
		gap: var(--space-2);
	}

	.add :global(button) {
		margin-top: var(--nudge-6);
	}
</style>
