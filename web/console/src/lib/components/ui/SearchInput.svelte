<script lang="ts">
	import type { Size } from './control';

	type Props = {
		value: string;
		label: string;
		placeholder?: string;
		size?: Exclude<Size, 'lg'>;
		onsubmit?: () => void;
		id?: string;
		name?: string;
		oninput?: (event: Event) => void;
		onkeydown?: (event: KeyboardEvent) => void;
	};

	let {
		value = $bindable(''),
		label,
		placeholder,
		size = 'md',
		onsubmit,
		id,
		name,
		oninput,
		onkeydown
	}: Props = $props();

	const uid = $props.id();
</script>

{#snippet field()}
	<i class="ri-search-line" aria-hidden="true"></i>
	<input
		id={id ?? uid}
		{name}
		type="search"
		{placeholder}
		bind:value
		{oninput}
		{onkeydown}
		aria-label={label}
	/>
{/snippet}

{#if onsubmit}
	<form
		class="search"
		data-size={size}
		role="search"
		onsubmit={(event) => {
			event.preventDefault();
			onsubmit();
		}}
	>
		{@render field()}
	</form>
{:else}
	<label class="search" data-size={size}>
		{@render field()}
	</label>
{/if}

<style>
	.search {
		--search-height: var(--control-height);

		flex: 1 1 auto;
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
		height: var(--search-height);
		max-height: var(--search-height);
		padding: 0 var(--space-2);
		border: var(--ds-border-width) solid var(--color-input-border);
		border-radius: var(--radius-field);
		background: var(--color-field);
		color: var(--color-text-hint);
		transition:
			background-color var(--speed-fast),
			border-color var(--speed-fast),
			box-shadow var(--speed-fast);
	}

	.search:hover {
		border-color: var(--color-input-border-hover);
		background: var(--color-field-hover);
	}

	.search:focus-within {
		border-color: var(--color-focus);
		background: var(--color-field-focus);
		box-shadow: inset 0 0 0 var(--ds-border-width) var(--color-focus);
	}

	.search[data-size='sm'] {
		--search-height: var(--control-height-sm);

		padding: 0 var(--space-2);
	}

	input {
		flex: 1;
		min-width: 0;
		border: none;
		background: transparent;
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-base);
	}

	input::placeholder {
		color: var(--color-placeholder);
	}

	.search[data-size='sm'] input {
		font-size: var(--text-sm);
	}

	input:focus {
		outline: none;
	}
</style>
