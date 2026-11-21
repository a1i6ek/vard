<script lang="ts">
	import type { ApplicationProfile } from '#lib/api/index.js';
	import { FormSection, Input, Note, SwitchField, Textarea } from '#lib/components/ui/index.js';
	import type { ApplicationForm } from '../form.svelte';

	type Props = { form: ApplicationForm };

	let { form }: Props = $props();

	/** Where each profile may be sent back to, as the server holds it. */
	const redirectHints: Record<ApplicationProfile, string> = {
		web: 'One per line. https, or http on localhost.',
		spa: 'One per line. https, or http on localhost.',
		native:
			'One per line. A claimed https address (Universal Links, App Links) or a private-use scheme such as com.example.app:/callback.',
		desktop:
			'One per line. A loopback address such as http://127.0.0.1/callback, which matches on any port, or a private-use scheme. Not localhost.',
		machine: ''
	};
</script>

<FormSection
	title="Redirects"
	description="Where users may be sent back to. Each URI is matched exactly, with no wildcards."
>
	<Textarea
		label="Redirect URIs"
		bind:value={form.redirects}
		rows={3}
		placeholder="https://shop.example.com/auth/callback"
		hint={redirectHints[form.input.profile]}
	/>

	{#if form.input.profile === 'native'}
		<SwitchField
			label="Allow development redirects"
			description="Lets this app register Expo Go's exp:// addresses while it is developed."
			bind:checked={form.input.allow_development_redirects}
		/>
		{#if form.input.allow_development_redirects}
			<Note boxed>
				exp:// addresses name a developer's machine on their network. Remove them and turn this off
				before the app is released.
			</Note>
		{/if}
		<Textarea
			label="iOS app IDs, for passkeys"
			bind:value={form.iosAppIds}
			rows={2}
			placeholder="ABCDE12345.com.example.app"
			hint="One per line: the team ID, a dot, the bundle ID. Published in apple-app-site-association (webcredentials), so the app may use the sign-in pages' passkeys."
		/>
		<Textarea
			label="Android apps, for passkeys"
			bind:value={form.android}
			rows={2}
			placeholder="com.example.app 14:6D:E9:83:C5:73:06:50:D8:EE:B9:95:2F:34:FC:64:16:A0:83:42:E6:1D:BE:A8:8A:04:96:B2:3F:CF:44:E5"
			hint="One per line: the package, then the SHA-256 fingerprint of each certificate it is signed with. Published in assetlinks.json."
		/>
	{/if}

	<Textarea
		label="Post-logout redirect URIs"
		bind:value={form.logoutRedirects}
		rows={2}
		placeholder="https://shop.example.com/"
		hint="One per line. Where the app may send users after they sign out."
	/>

	{#if form.input.profile === 'web'}
		<Input
			label="Back-channel logout URI"
			type="url"
			bind:value={form.input.backchannel_logout_uri}
			maxlength={512}
			placeholder="https://shop.example.com/auth/backchannel-logout"
			hint="Optional. The server posts a logout token here when someone signed in to this app signs out, so the app can end its own session too."
		/>
	{/if}
</FormSection>
