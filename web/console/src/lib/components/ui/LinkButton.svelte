<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAnchorAttributes } from 'svelte/elements';
	import type { ControlProps } from './control';

	type Props = HTMLAnchorAttributes &
		Omit<ControlProps, 'loading'> & {
			/** Resolved by the caller. */
			href: string;
			block?: boolean;
			children: Snippet;
		};

	let {
		href,
		size,
		variant = 'solid',
		colorPalette = 'neutral',
		disabled = false,
		block = false,
		icon,
		children,
		...rest
	}: Props = $props();
</script>

<!-- Somewhere to go, not something to do, shaped like a button. -->
<!-- eslint-disable svelte/no-navigation-without-resolve -->
<a
	href={disabled ? undefined : href}
	class="control"
	data-size={size ?? (block ? 'lg' : 'md')}
	data-variant={variant}
	data-palette={colorPalette}
	data-block={block || undefined}
	aria-disabled={disabled || undefined}
	role={disabled ? 'link' : undefined}
	{...rest}
>
	{#if icon}
		<i class={icon} aria-hidden="true"></i>
	{/if}

	{@render children()}
</a>
