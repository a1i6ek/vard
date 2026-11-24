<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		/** The control's id, which the label names. */
		for: string;
		label: string;
		/** The control holds a value, so the label stays raised. */
		filled: boolean;
		invalid?: boolean;
		disabled?: boolean;
		/** The control: an input, styled by this box. */
		children: Snippet;
		/** What sits at the right end, such as a password's eye. */
		end?: Snippet;
	};

	let {
		for: control,
		label,
		filled,
		invalid = false,
		disabled = false,
		children,
		end
	}: Props = $props();
</script>

<!-- A filled box with no border, the label inside it. At rest, in an empty
     field, the label sits where the value is typed and reads as the prompt;
     with focus or a value it rises to the top, smaller, and the value takes
     its line. Focus is a 2px brand ring drawn inside the edge, so nothing
     around the field moves; blame is the same ring in red. -->
<div class="box" class:filled class:invalid class:disabled class:has-end={end !== undefined}>
	<label for={control}>{label}</label>
	{@render children()}
	{#if end}<span class="end">{@render end()}</span>{/if}
</div>

<style>
	/* The geometry, one set of numbers: the raised label 8px down (12px type
	   on a 16px line), the value's 20px line 24px down with 8px under it, so
	   the box is --field-height (52px); the resting label (14px on a 20px
	   line) sits centred, 16px down. */
	.box {
		--value-top: 24px;

		position: relative;
		min-width: 0;
		border-radius: var(--radius-field);
		background: var(--color-field);
		transition:
			background-color var(--speed),
			box-shadow var(--speed);
	}

	.box:hover {
		background: var(--color-field-hover);
	}

	.box:focus-within {
		background: var(--color-field-focus);
		box-shadow: inset 0 0 0 var(--nudge-2) var(--color-brand);
	}

	.box.invalid,
	.box.invalid:focus-within {
		box-shadow: inset 0 0 0 var(--nudge-2) var(--color-danger);
	}

	.box.disabled {
		background: var(--color-field);
		opacity: 0.5;
	}

	label {
		position: absolute;
		top: 16px;
		left: var(--nudge-12);
		z-index: 2;
		max-width: calc(100% - var(--space-6));
		overflow: hidden;
		color: var(--color-text-hint);
		font-size: var(--text-base);
		line-height: var(--leading-md);
		white-space: nowrap;
		text-overflow: ellipsis;

		/* Pressing the words focuses the control under them, as pressing
		   anywhere else in the box does. */
		pointer-events: none;
		transition:
			top var(--speed) ease,
			font-size var(--speed) ease,
			line-height var(--speed) ease;
	}

	/* Raised: a value is there, the field has focus, or a password manager
	   filled it in before anyone typed. */
	.filled label,
	.box:focus-within label,
	.box:has(:global(:autofill)) label {
		top: 8px;
		font-size: var(--text-xs);
		line-height: var(--leading-xs);
	}

	.invalid label {
		color: var(--color-danger);
	}

	.box :global(input) {
		position: relative;
		z-index: 1;
		display: block;
		width: 100%;
		height: var(--field-height);
		margin: 0;
		padding: var(--value-top) var(--nudge-12) var(--nudge-8);
		border: none;
		border-radius: inherit;
		background: transparent;
		color: var(--color-text);
		font-size: var(--text-base);
		font-weight: 500;
		line-height: var(--leading-md);
		outline: none;
	}

	.has-end :global(input) {
		padding-right: calc(var(--space-6) + var(--nudge-4));
	}

	/* A placeholder is an example of what to type, so it shows only once
	   the label has moved out of its way. */
	.box :global(input::placeholder) {
		color: var(--color-text-disabled);
		opacity: 0;
		transition: opacity var(--speed);
	}

	.box:focus-within :global(input::placeholder) {
		opacity: 1;
	}

	/* A browser's autofill paints its own fill over the field; this keeps
	   the field's own. */
	.box :global(input:autofill) {
		-webkit-text-fill-color: var(--color-text);
		caret-color: var(--color-text);
		transition: background-color 100000s 0s;
	}

	.box.disabled :global(input) {
		cursor: not-allowed;
	}

	.end {
		position: absolute;
		top: 50%;
		right: var(--nudge-8);
		z-index: 2;
		display: flex;
		transform: translateY(-50%);
	}

	@media (prefers-reduced-motion: reduce) {
		.box,
		label {
			transition: none;
		}
	}
</style>
