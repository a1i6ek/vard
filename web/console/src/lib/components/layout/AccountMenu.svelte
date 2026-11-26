<script lang="ts">
	import { goto, refreshAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { useQueryClient } from '@tanstack/svelte-query';
	import { adminApi, type Admin } from '#lib/api/index.js';
	import ProfileDialog from '#lib/components/profile/ProfileDialog.svelte';
	import RoleCodes from '#lib/components/profile/RoleCodes.svelte';
	import {
		Avatar,
		ConfirmDialog,
		DropdownMenu,
		MenuItem,
		MenuSeparator
	} from '#lib/components/ui/index.js';

	let { admin }: { admin: Admin } = $props();

	const queryClient = useQueryClient();

	const name = $derived(admin.full_name.trim() || admin.email);

	let settingsOpen = $state(false);
	let confirmingSignOut = $state(false);
	let signingOut = $state(false);

	async function signOut() {
		signingOut = true;
		try {
			await adminApi.logout();
		} finally {
			// Whatever the server answered, nothing loaded for this
			// administrator is left for whoever signs in next.
			queryClient.clear();
			await refreshAll();
			await goto(resolve('/login'), { replace: true });
		}
	}
</script>

<DropdownMenu label="Account: {name}" shape="avatar" width="16rem">
	{#snippet trigger()}
		<Avatar {name} src={admin.avatar_url} size="xs" />
	{/snippet}

	<header>
		<Avatar {name} src={admin.avatar_url} />
		<div class="who">
			<strong title={name}>{name}</strong>
			{#if admin.email !== name}<span title={admin.email}>{admin.email}</span>{/if}
			<RoleCodes {admin} />
		</div>
	</header>

	<MenuSeparator />

	<MenuItem
		value="settings"
		icon="ri-user-settings-line"
		label="Account settings"
		onSelect={() => (settingsOpen = true)}
	/>
	<MenuItem
		value="sign-out"
		icon="ri-logout-box-r-line"
		label="Sign out"
		danger
		onSelect={() => (confirmingSignOut = true)}
	/>
</DropdownMenu>

<ProfileDialog {admin} bind:open={settingsOpen} />

<ConfirmDialog
	bind:open={confirmingSignOut}
	title="Sign out?"
	description="You will need to sign in again to manage the console."
	tone="danger"
	icon="ri-logout-box-r-line"
	confirmLabel="Sign out"
	busy={signingOut}
	onConfirm={signOut}
/>

<style>
	header {
		display: flex;
		align-items: flex-start;
		gap: var(--space-2);
		padding: var(--space-2);
	}

	.who {
		display: flex;
		flex-direction: column;
		gap: var(--nudge-2);
		min-width: 0;
		line-height: 1.3;
	}

	.who strong,
	.who span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.who span {
		color: var(--color-text-hint);
		font-size: var(--text-xs);
	}
</style>
