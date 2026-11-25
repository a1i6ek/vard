<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Dialog } from '@ark-ui/svelte/dialog';
	import { Portal } from '@ark-ui/svelte/portal';
	import IconButton from './IconButton.svelte';

	type Props = {
		open: boolean;
		title: string;
		/** A line under the title: when, or what kind. */
		subtitle?: string;
		children: Snippet;
		/** At the foot, for what can be done with what is shown. */
		actions?: Snippet;
		onclose?: () => void;
	};

	let { open = $bindable(false), title, subtitle, children, actions, onclose }: Props = $props();
</script>

<!-- A panel from the right edge, over the page and beside what was chosen in
     it, for reading one thing in full without leaving the list. -->
<Dialog.Root
	bind:open
	lazyMount
	unmountOnExit
	onOpenChange={(details) => {
		if (!details.open) onclose?.();
	}}
>
	<Portal>
		<Dialog.Backdrop />
		<Dialog.Positioner class="drawer">
			<Dialog.Content class="drawer">
				<header>
					<div class="heading">
						<Dialog.Title>{title}</Dialog.Title>
						{#if subtitle}<Dialog.Description>{subtitle}</Dialog.Description>{/if}
					</div>
					<IconButton icon="ri-close-line" label="Close" size="sm" onclick={() => (open = false)} />
				</header>

				<div class="body">{@render children()}</div>

				{#if actions}<footer>{@render actions()}</footer>{/if}
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog.Root>

<style>
	header {
		display: flex;
		align-items: flex-start;
		gap: var(--space-3);
		padding: var(--space-4) var(--space-4) var(--space-3) var(--space-5);
		border-bottom: 1px solid var(--color-border);
	}

	.heading {
		flex: 1;
		min-width: 0;
	}

	.heading :global([data-scope='dialog'][data-part='title']) {
		margin: 0;
		font-size: var(--text-lg);
		overflow-wrap: anywhere;
	}

	.heading :global([data-scope='dialog'][data-part='description']) {
		margin: var(--nudge-2) 0 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.body {
		flex: 1;
		min-height: 0;
		padding: var(--space-4) var(--space-5) var(--space-5);
		overflow-y: auto;
		overscroll-behavior: contain;
	}

	footer {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-5);
		border-top: 1px solid var(--color-border);
	}
</style>
