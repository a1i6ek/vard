<script lang="ts" generics="Value extends string">
	import { Menu } from '@ark-ui/svelte/menu';
	import { Portal } from '@ark-ui/svelte/portal';
	import type { IconName } from './icon';

	type Option = { value: Value; label: string };

	type Props = {
		/** What is filtered: "Kind". */
		label: string;
		icon?: IconName;
		options: Option[];
		value: Value[];
		onChange: (value: Value[]) => void;
	};

	let { label, icon, options, value, onChange }: Props = $props();

	const chosen = $derived(new Set(value));

	function toggle(option: Value, on: boolean) {
		onChange(on ? [...value, option] : value.filter((it) => it !== option));
	}
</script>

<!-- Several choices of one kind, picked in a menu that stays open while they
     are ticked; the button says how many are on. -->
<Menu.Root closeOnSelect={false} positioning={{ placement: 'bottom-start', gutter: 6 }}>
	<Menu.Trigger
		class="control filter-menu"
		data-size="md"
		data-variant="subtle"
		data-palette="neutral"
		data-active={value.length > 0 || undefined}
	>
		{#if icon}<i class={icon} aria-hidden="true"></i>{/if}
		<span>{label}</span>
		{#if value.length > 0}<span class="count">{value.length}</span>{/if}
		<i class="ri-arrow-down-s-line chevron" aria-hidden="true"></i>
	</Menu.Trigger>

	<Portal>
		<Menu.Positioner>
			<Menu.Content class="filter-menu-content" aria-label={label}>
				{#each options as option (option.value)}
					<Menu.CheckboxItem
						value={option.value}
						checked={chosen.has(option.value)}
						onCheckedChange={(on) => toggle(option.value, on)}
					>
						<span class="tick" data-on={chosen.has(option.value) || undefined} aria-hidden="true">
							{#if chosen.has(option.value)}<i class="ri-check-line"></i>{/if}
						</span>
						<Menu.ItemText>{option.label}</Menu.ItemText>
					</Menu.CheckboxItem>
				{/each}

				{#if value.length > 0}
					<Menu.Separator />
					<Menu.Item value="__clear" onSelect={() => onChange([])}>
						<i class="ri-close-line" aria-hidden="true"></i>
						Clear
					</Menu.Item>
				{/if}
			</Menu.Content>
		</Menu.Positioner>
	</Portal>
</Menu.Root>

<style>
	:global(.filter-menu.control) {
		gap: var(--space-1);
		font-weight: 500;
	}

	:global(.filter-menu.control[data-active]) {
		color: var(--color-text);
		font-weight: 700;
	}

	.count {
		display: inline-grid;
		place-items: center;
		min-width: 20px;
		height: 20px;
		padding: 0 var(--nudge-4);
		border-radius: var(--radius-round);
		background: var(--color-primary);
		color: var(--color-primary-text);
		font-size: var(--text-xs);
		font-variant-numeric: tabular-nums;
	}

	.chevron {
		color: var(--color-text-hint);
	}

	:global(.filter-menu-content) {
		width: 16rem;
	}

	.tick {
		display: grid;
		flex: none;
		place-items: center;
		width: 18px;
		height: 18px;
		border: 2px solid var(--color-input-border);
		border-radius: var(--radius-check);
		color: var(--color-brand-text);
		font-size: var(--text-sm);
	}

	.tick[data-on] {
		border-color: var(--color-brand);
		background: var(--color-brand);
	}
</style>
