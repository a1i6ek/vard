<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import type { ControlProps } from './control';
	import Spinner from './Spinner.svelte';

	type Props = HTMLButtonAttributes &
		ControlProps & {
			/** Spans its container, one size up: a form's own buttons stand as
			    tall as its fields. */
			block?: boolean;
			children: Snippet;
		};

	let {
		size,
		variant = 'solid',
		colorPalette = 'neutral',
		loading = false,
		disabled = false,
		block = false,
		icon,
		type = 'button',
		children,
		...rest
	}: Props = $props();
</script>

<button
	{type}
	class="control"
	data-size={size ?? (block ? 'lg' : 'md')}
	data-variant={variant}
	data-palette={colorPalette}
	data-block={block || undefined}
	data-loading={loading || undefined}
	aria-busy={loading || undefined}
	disabled={disabled || loading}
	{...rest}
>
	{#if loading}
		<Spinner />
	{:else if icon}
		<i class={icon} aria-hidden="true"></i>
	{/if}

	{@render children()}
</button>
