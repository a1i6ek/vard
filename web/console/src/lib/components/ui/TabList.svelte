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
	/* Browser tabs: the strip is ruled along its bottom edge, each tab is a
	   rounded-top tile that sits on the rule, and the chosen one rises above
	   it — surface fill, hairline sides, a brand bar along its top — so it
	   reads as the open tab of a browser or of an editor. */
	.list {
		display: flex;
		gap: var(--nudge-2);
		overflow-x: auto;
		padding-top: var(--nudge-4);
		box-shadow: inset 0 calc(-1 * var(--nudge-1)) 0 var(--color-border);
		scrollbar-width: none;
	}

	.list::-webkit-scrollbar {
		display: none;
	}

	button {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		flex-shrink: 0;
		height: 40px;
		margin-bottom: calc(-1 * var(--nudge-1));
		padding: 0 var(--space-3);
		border: 1px solid transparent;
		border-bottom: none;
		border-radius: var(--radius-lg) var(--radius-lg) 0 0;
		background: transparent;
		color: var(--color-text-hint);
		font: inherit;
		font-size: var(--text-base);
		font-weight: 500;
		white-space: nowrap;
		cursor: pointer;
		transition:
			background-color var(--speed-fast),
			color var(--speed-fast);
	}

	button:hover:not(:disabled, .selected) {
		background: var(--color-secondary);
		color: var(--color-text);
	}

	button.selected {
		z-index: 1;
		border-color: var(--color-border);
		background: var(--color-surface);
		box-shadow: inset 0 var(--nudge-2) 0 var(--color-brand);
		color: var(--color-text);
		font-weight: 700;
	}

	button:disabled {
		cursor: not-allowed;
		opacity: 0.5;
	}

	button:focus-visible {
		outline: 2px solid var(--color-info);
		outline-offset: -2px;
	}

	.count {
		display: inline-grid;
		place-items: center;
		min-width: 20px;
		height: 20px;
		padding: 0 var(--nudge-6);
		border-radius: var(--radius-round);
		background: var(--color-secondary);
		color: var(--color-text-hint);
		font-size: var(--text-xs);
		font-weight: 500;
	}
</style>
