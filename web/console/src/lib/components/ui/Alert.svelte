<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		tone?: 'danger' | 'warning' | 'info' | 'success';
		children: Snippet;
	};

	let { tone = 'danger', children }: Props = $props();

	const icons = {
		danger: 'ri-error-warning-line',
		warning: 'ri-error-warning-line',
		info: 'ri-information-line',
		success: 'ri-checkbox-circle-line'
	};
</script>

<!-- An error is announced as soon as it appears; a notice is just read in its
     place. -->
<div class="alert {tone}" role={tone === 'danger' ? 'alert' : 'status'}>
	<i class={icons[tone]} aria-hidden="true"></i>
	<div class="body">{@render children()}</div>
</div>

<!-- A section message, as the Atlassian Design System's: the tone's subtle
     fill and its icon colour, no border. -->
<style>
	.alert {
		--tone: var(--ds-icon-danger);
		--tone-surface: var(--ds-background-danger);

		display: flex;
		align-items: flex-start;
		gap: var(--ds-space-100);
		padding: var(--ds-space-150) var(--ds-space-200);
		border-radius: var(--ds-radius-medium);
		background: var(--tone-surface);
		color: var(--ds-text);
		font-size: var(--text-base);
		line-height: var(--leading-md);
	}

	.alert > i {
		flex-shrink: 0;
		color: var(--tone);
		font-size: var(--icon-lg);
		line-height: var(--leading-md);
	}

	.body {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.warning {
		--tone: var(--ds-icon-warning);
		--tone-surface: var(--ds-background-warning);
	}

	.info {
		--tone: var(--ds-icon-information);
		--tone-surface: var(--ds-background-information);
	}

	.success {
		--tone: var(--ds-icon-success);
		--tone-surface: var(--ds-background-success);
	}
</style>
