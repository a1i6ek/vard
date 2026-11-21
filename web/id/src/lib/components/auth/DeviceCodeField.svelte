<script lang="ts">
	import { formatUserCode } from '#lib/utils/device.js';
	import { TextField } from '../ui/index.js';

	type Props = { label: string; value: string; hint?: string; error?: string };

	let { label, value = $bindable(''), hint, error }: Props = $props();
</script>

<!-- The code in the two groups of four the device shows, whatever is typed
     or pasted: the dash comes back by itself. Not one-time-code: a phone
     would offer a code from a text message, which this never is. -->
<div class="code">
	<TextField
		{label}
		{hint}
		{error}
		bind:value={() => value, (typed) => (value = formatUserCode(typed))}
		autocomplete="off"
		autocapitalize="characters"
		spellcheck={false}
		maxlength={9}
		placeholder="BCDF-GHJK"
	/>
</div>

<style lang="scss">
	.code :global(input) {
		font-family: var(--font-mono);
		font-size: var(--text-2xl);
		letter-spacing: 0.15em;
		text-align: center;
	}
</style>
