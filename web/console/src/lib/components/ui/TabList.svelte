<script module lang="ts">
	import type { IconName } from './icon';

	export type Tab = {
		value: string;
		label: string;
		icon?: IconName;
		count?: number;
		disabled?: boolean;
	};
</script>

<script lang="ts">
	type Props = {
		tabs: Tab[];
		value: string;
		label: string;
		/** Names the tabs' panels when there are some (Tabs); a strip that only
		    changes what one list shows has none. */
		panelId?: (value: string) => string;
		tabId?: (value: string) => string;
		onchange?: (value: string) => void;
	};

	let { tabs, value = $bindable(), label, panelId, tabId, onchange }: Props = $props();

	let triggers: HTMLButtonElement[] = $state([]);

	function choose(next: string) {
		value = next;
		onchange?.(next);
	}

	function keydown(event: KeyboardEvent, index: number) {
		const enabled = tabs.map((tab, i) => (tab.disabled ? -1 : i)).filter((i) => i >= 0);
		const at = enabled.indexOf(index);

		let next: number | undefined;
		if (event.key === 'ArrowRight') next = enabled[(at + 1) % enabled.length];
		if (event.key === 'ArrowLeft') next = enabled[(at - 1 + enabled.length) % enabled.length];
		if (event.key === 'Home') next = enabled[0];
		if (event.key === 'End') next = enabled[enabled.length - 1];

		if (next === undefined) return;

		event.preventDefault();
		choose(tabs[next].value);
		triggers[next]?.focus();
	}
</script>

<div class="list" role="tablist" aria-label={label}>
	{#each tabs as tab, index (tab.value)}
		<button
			bind:this={triggers[index]}
			type="button"
			role="tab"
			id={tabId?.(tab.value)}
			aria-controls={panelId?.(tab.value)}
			aria-selected={tab.value === value}
			tabindex={tab.value === value ? 0 : -1}
			disabled={tab.disabled}
			class={{ selected: tab.value === value }}
			onclick={() => choose(tab.value)}
			onkeydown={(event) => keydown(event, index)}
		>
			{#if tab.icon}
				<i class={tab.icon} aria-hidden="true"></i>
			{/if}
			{tab.label}
			{#if tab.count !== undefined}
				<span class="count">{tab.count}</span>
			{/if}
		</button>
	{/each}
</div>

<style>
	/* The Atlassian Design System's tabs: a 2px rule under the strip, subtle
	   words, and the chosen tab's words and underline in the selected blue. */
	.list {
		display: flex;
		gap: var(--ds-space-050);
		overflow-x: auto;
		box-shadow: inset 0 calc(-1 * var(--ds-border-width-selected)) 0 var(--ds-border);
		scrollbar-width: none;
	}

	.list::-webkit-scrollbar {
		display: none;
	}

	button {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: var(--ds-space-075);
		flex-shrink: 0;
		height: 36px;
		padding: 0 var(--ds-space-100);
		border: none;
		border-radius: var(--ds-radius-medium) var(--ds-radius-medium) 0 0;
		background: transparent;
		color: var(--ds-text-subtle);
		font: inherit;
		font-size: var(--text-base);
		font-weight: 500;
		white-space: nowrap;
		cursor: pointer;
		transition: color var(--speed-fast);
	}

	button::after {
		position: absolute;
		right: var(--ds-space-100);
		bottom: 0;
		left: var(--ds-space-100);
		height: var(--ds-border-width-selected);
		border-radius: var(--ds-radius-xsmall);
		background: transparent;
		content: '';
		transition: background-color var(--speed-fast);
	}

	button:hover:not(:disabled, .selected) {
		color: var(--ds-text);
	}

	button:hover:not(:disabled, .selected)::after {
		background: var(--ds-border-bold);
	}

	button.selected {
		color: var(--ds-text-selected);
	}

	button.selected::after {
		background: var(--ds-border-selected);
	}

	button:disabled {
		color: var(--ds-text-disabled);
		cursor: not-allowed;
	}

	button:focus-visible {
		outline: var(--ds-border-width-focused) solid var(--ds-border-focused);
		outline-offset: calc(-1 * var(--ds-border-width-focused));
	}

	.count {
		display: inline-grid;
		place-items: center;
		min-width: 20px;
		height: 16px;
		padding: 0 var(--nudge-4);
		border-radius: var(--ds-radius-small);
		background: var(--ds-background-neutral);
		color: var(--ds-text-subtle);
		font-size: var(--text-xs);
		font-weight: 700;
	}

	button.selected .count {
		background: var(--ds-background-selected);
		color: var(--ds-text-selected);
	}
</style>
