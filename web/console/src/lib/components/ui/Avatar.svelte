<script lang="ts">
	import { initials } from '#lib/utils/format.js';
	import Thumb from './Thumb.svelte';

	type Props = {
		/** Whose it is: their initials stand in for a missing picture. */
		name: string;
		src?: string;
		size?: 'xs' | 'sm' | 'md';
	};

	let { name, src, size = 'sm' }: Props = $props();
</script>

<Thumb {src} text={initials(name)} {size} shape="circle" tone="accent" />
