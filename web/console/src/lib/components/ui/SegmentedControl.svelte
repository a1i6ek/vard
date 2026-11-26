<script lang="ts" generics="Value extends string">
	type Props = {
		label: string;
		options: readonly { value: Value; label: string; reset?: boolean; disabled?: boolean }[];
		value: Value;
		onChange: (value: Value) => void;
	};

	let { label, options, value, onChange }: Props = $props();

	let group = $state<HTMLElement | null>(null);
	/** Where the chosen tile is drawn, measured from the chosen option; null
	    until measured, when the option paints its own tile instead. */
	let thumb = $state<{ left: number; width: number } | null>(null);

	function measure() {
		const chosen = group?.querySelector<HTMLElement>('[aria-checked="true"]');
		thumb = chosen ? { left: chosen.offsetLeft, width: chosen.offsetWidth } : null;
	}

	$effect(() => {
		void value;
		measure();
		// The fonts arriving, or the page reflowing, change the options' widths;
		// without a ResizeObserver, the first measure stands.
		if (!group || typeof ResizeObserver === 'undefined') return;
		const resized = new ResizeObserver(measure);
		resized.observe(group);
		return () => resized.disconnect();
	});

	/** The arrow keys, Home and End move the choice, as in any radio group,
	    passing over an option that is turned off. */
	function onKeydown(event: KeyboardEvent) {
		const usable = options.filter((option) => !option.disabled);
		const at = usable.findIndex((option) => option.value === value);
		const next = {
			ArrowRight: at + 1,
			ArrowDown: at + 1,
			ArrowLeft: at - 1,
			ArrowUp: at - 1,
			Home: 0,
			End: usable.length - 1
		}[event.key];
		if (next === undefined) return;

		event.preventDefault();
		const option = usable[(next + usable.length) % usable.length];
		// Focus follows the arrow at once, to the option moved to: the page may
		// take a moment to confirm the choice (a new range is loaded first).
		group?.querySelector<HTMLElement>(`[data-value="${CSS.escape(option.value)}"]`)?.focus();
		onChange(option.value);
	}
</script>

<!-- One choice of a few, shown together: a grey pill track, and a solid
     brand pill under the chosen option that slides to the next. A radio group to
     assistive technology and the keyboard: Tab reaches the chosen option, the
     arrows move the choice. -->
<div
	class="segmented"
	class:measured={thumb !== null}
	role="radiogroup"
	aria-label={label}
	tabindex="-1"
	bind:this={group}
	onkeydown={onKeydown}
>
	{#if thumb}
		<span
			class="thumb"
			style:transform={`translateX(${thumb.left}px)`}
			style:width={`${thumb.width}px`}
			aria-hidden="true"
		></span>
	{/if}

	{#each options as option, index (option.value)}
		{@const chosen = option.value === value}
		<button
			type="button"
			role="radio"
			aria-checked={chosen}
			tabindex={chosen ? 0 : -1}
			disabled={option.disabled}
			data-label={option.label}
			data-value={option.value}
			onclick={() => onChange(option.value)}
		>
			{option.label}
		</button>
		{#if option.reset && index < options.length - 1}
			<span class="rule" aria-hidden="true"></span>
		{/if}
	{/each}
</div>

<style>
	/* A bordered track whose chosen option wears the Atlassian Design
	   System's selected colours, on a block that slides under it. A radio
	   group to assistive technology and the keyboard: Tab reaches the chosen
	   option, the arrows move the choice. */
	.segmented {
		position: relative;
		display: flex;
		flex: none;
		align-items: center;
		gap: var(--nudge-2);
		height: var(--control-height);
		padding: var(--nudge-2);
		border: var(--ds-border-width) solid var(--ds-border);
		border-radius: var(--ds-radius-medium);
		background: transparent;
		isolation: isolate;
	}

	.segmented:focus {
		outline: none;
	}

	/* The chosen block, placed against the track's padding edge, so a left
	   of 0 is the first option's left. */
	.thumb {
		position: absolute;
		top: var(--nudge-2);
		bottom: var(--nudge-2);
		left: 0;
		z-index: -1;
		border-radius: var(--ds-radius-small);
		background: var(--ds-background-selected);
		box-shadow: inset 0 0 0 var(--ds-border-width) var(--ds-border-selected);
		transition:
			transform var(--speed-slow) cubic-bezier(0.3, 0.7, 0.2, 1),
			width var(--speed-slow) cubic-bezier(0.3, 0.7, 0.2, 1);
	}

	button {
		display: inline-flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		padding: 0 var(--space-3);
		border: none;
		border-radius: var(--ds-radius-small);
		background: transparent;
		color: var(--ds-text-subtle);
		font: inherit;
		font-size: var(--text-sm);
		font-weight: 500;
		white-space: nowrap;
		cursor: pointer;
		transition: color var(--speed-fast);
	}

	/* The bold label, laid out and hidden, so an option is as wide chosen as
	   not and choosing one moves nothing beside it. */
	button::after {
		content: attr(data-label);
		height: 0;
		overflow: hidden;
		font-weight: 700;
		visibility: hidden;
		pointer-events: none;
	}

	button:hover:not(:disabled, [aria-checked='true']) {
		color: var(--color-text);
	}

	button[aria-checked='true'] {
		color: var(--ds-text-selected);
		font-weight: 700;
	}

	/* Before the pill is measured (the page as the server rendered it), the
	   chosen option paints its own. */
	.segmented:not(.measured) button[aria-checked='true'] {
		background: var(--ds-background-selected);
		box-shadow: inset 0 0 0 var(--ds-border-width) var(--ds-border-selected);
	}

	button:disabled {
		color: var(--ds-text-disabled);
		cursor: not-allowed;
	}

	button:focus-visible {
		outline: 2px solid var(--color-focus);
		outline-offset: -2px;
	}

	.rule {
		align-self: stretch;
		width: 1px;
		margin: var(--nudge-8) var(--nudge-2);
		background: var(--color-border);
	}

	@media (max-width: 40rem) {
		.segmented {
			overflow-x: auto;
			scrollbar-width: none;
		}

		.segmented::-webkit-scrollbar {
			display: none;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.thumb,
		button {
			transition: none;
		}
	}
</style>
