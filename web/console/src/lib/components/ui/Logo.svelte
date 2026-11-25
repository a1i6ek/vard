<script lang="ts">
	import type { IconName } from './icon';

	type Props = {
		src?: string;
		icon: IconName;
		size?: 'sm' | 'md';
	};

	let { src, icon, size = 'sm' }: Props = $props();

	let failed = $state('');

	const picture = $derived(src && src !== failed ? src : '');

	const glyph = { sm: 16, md: 20 };
</script>

<span class="logo {size}" class:picture aria-hidden="true">
	{#if picture}
		<img src={picture} alt="" onerror={() => (failed = picture)} />
	{:else}
		<i class={icon} style:font-size="{glyph[size]}px"></i>
	{/if}
</span>

<style>
	.logo {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		aspect-ratio: 1;
		border-radius: var(--radius-lg);
		background: var(--color-accent);
		color: var(--color-accent-text);
	}

	.sm {
		width: 34px;
	}

	.md {
		width: var(--control-height);
	}

	.picture {
		background: transparent;
		color: inherit;
	}

	img {
		width: 100%;
		height: 100%;
		object-fit: contain;
	}
</style>
