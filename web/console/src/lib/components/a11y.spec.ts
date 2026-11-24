// @vitest-environment jsdom
import { createRawSnippet } from 'svelte';
import { describe, it } from 'vitest';
import type { Admin, Application } from '#lib/api/index.js';
import { expectAccessible, expectDescribed } from '#lib/testing/a11y.js';
import IntrospectorList from './apis/IntrospectorList.svelte';
import ExchangeRuleList from './apis/ExchangeRuleList.svelte';
import ClientSetup from './applications/ClientSetup.svelte';
import LogoutDeliveryList from './applications/LogoutDeliveryList.svelte';
import ProfilePicker from './applications/ProfilePicker.svelte';
import Inspector from './flows/editor/Inspector.svelte';
import { TEMPLATES } from './flows/steps';
import LogoField from './organization/LogoField.svelte';
import RoleCodes from './profile/RoleCodes.svelte';
import DomainProofList from './sso/DomainProofList.svelte';
import SSOSignInTab from './sso/SSOSignInTab.svelte';
import { SSOForm } from './sso/form.svelte';
import Button from './ui/Button.svelte';
import DataTable from './ui/DataTable.svelte';
import EmptyState from './ui/EmptyState.svelte';
import DatePicker from './ui/DatePicker.svelte';
import Input from './ui/Input.svelte';
import PasswordInput from './ui/PasswordInput.svelte';
import SegmentedControl from './ui/SegmentedControl.svelte';
import Select from './ui/Select.svelte';
import SwitchField from './ui/SwitchField.svelte';
import TabList from './ui/TabList.svelte';
import Textarea from './ui/Textarea.svelte';

const text = (words: string) => createRawSnippet(() => ({ render: () => `<span>${words}</span>` }));

const admin = {
	id: '1',
	username: 'ada',
	email: 'ada@example.com',
	full_name: 'Ada Lovelace',
	first_name: 'Ada',
	last_name: 'Lovelace',
	avatar_url: '',
	status: 'active',
	roles: ['auditor', 'super_admin'],
	permissions: [],
	scoped_permissions: {},
	is_super_admin: true,
	mfa_enabled: true
} satisfies Admin;

describe('the panel is usable without seeing it', () => {
	it("an application's logout notices: delivered, owed and given up", () =>
		expectAccessible(LogoutDeliveryList, {
			deliveries: [
				{
					id: '1',
					session_id: 's1',
					user_id: 'u1',
					status: 'delivered',
					tries: 1,
					last_error: '',
					created_at: '2026-11-20T10:00:00Z',
					next_try_at: null,
					delivered_at: '2026-11-20T10:00:01Z'
				},
				{
					id: '2',
					session_id: 's2',
					user_id: 'u2',
					status: 'pending',
					tries: 2,
					last_error: 'the application answered 503',
					created_at: '2026-11-20T10:00:00Z',
					next_try_at: '2026-11-20T10:01:00Z',
					delivered_at: null
				},
				{
					id: '3',
					session_id: 's3',
					user_id: 'u3',
					status: 'failed',
					tries: 1,
					last_error: 'the application answered 400',
					created_at: '2026-11-20T10:00:00Z',
					next_try_at: null,
					delivered_at: null
				}
			]
		}));

	it("an API's token exchange rules: one rule, and the form to add another", () =>
		expectAccessible(ExchangeRuleList, {
			rules: [{ application_id: 'a', subject_application_id: 'b', scopes: ['orders:read'] }],
			applications: [
				{
					id: 'a',
					name: 'Billing',
					profile: 'machine',
					client_id: 'billing',
					confidential: true,
					listed: false
				},
				{
					id: 'b',
					name: 'Shop',
					profile: 'spa',
					client_id: 'shop',
					confidential: false,
					listed: false
				}
			],
			scopes: ['orders:read', 'orders:write'],
			editable: true,
			saving: false,
			onSave: () => {}
		}));

	it("an API's introspectors: one listed, one not, and a public client left out", () =>
		expectAccessible(IntrospectorList, {
			introspectors: [
				{
					id: '1',
					name: 'Orders backend',
					profile: 'machine',
					client_id: 'a1',
					confidential: true,
					listed: true
				},
				{
					id: '2',
					name: 'Shop',
					profile: 'web',
					client_id: 'b2',
					confidential: true,
					listed: false
				},
				{
					id: '3',
					name: 'Phone app',
					profile: 'native',
					client_id: 'c3',
					confidential: false,
					listed: false
				}
			],
			editable: true,
			saving: null,
			onChange: () => {}
		}));

	it('the logo picker with a logo uploaded', () =>
		expectAccessible(LogoField, {
			version: 'a1b2c3d4e5f60718',
			editable: true,
			onsaved: () => {}
		}));

	it('the logo picker showing the default, read only', () =>
		expectAccessible(LogoField, { version: '', editable: false, onsaved: () => {} }));

	it("a flow's two-step step, with a second factor required", () =>
		expectAccessible(Inspector, {
			draft: {
				...TEMPLATES[0].draft,
				name: 'Staff',
				slug: 'staff',
				steps: ['identifier', 'password', 'totp'],
				require_second_factor: true
			},
			kinds: [
				{
					step: 'totp',
					label: 'Two-step verification',
					description: 'An authenticator app or passkeys.',
					fixed: false,
					implemented: true
				}
			],
			selected: 'totp',
			editable: true,
			creating: false,
			savedDefault: false,
			applications: 1,
			onRemove: () => {}
		}));

	it('a text input', () => expectAccessible(Input, { label: 'Name', value: '' }));

	it("a connection's sign-in settings, trusting the provider's second factor", () =>
		expectAccessible(SSOSignInTab, {
			form: Object.assign(new SSOForm(), {
				name: 'Acme',
				domains: ['acme.com'],
				secondFactor: 'provider'
			}),
			readOnly: false
		}));

	it("a SAML connection's authentication contexts", () =>
		expectAccessible(SSOSignInTab, {
			form: Object.assign(new SSOForm(), {
				name: 'Globex',
				protocol: 'saml',
				domains: ['globex.com'],
				secondFactor: 'provider',
				mfaContexts: 'urn:example:mfa'
			}),
			readOnly: false
		}));

	it("a connection's domain proofs: proved and waiting for its record", () =>
		expectAccessible(DomainProofList, {
			proofs: [
				{
					domain: 'acme.com',
					proved: true,
					method: 'dns',
					record_name: '_x-challenge.acme.com',
					record_value: 'x-domain-verification=abc',
					verified_at: '2026-10-01T00:00:00Z',
					checked_at: '2026-10-01T00:00:00Z'
				},
				{
					domain: 'acme.org',
					proved: false,
					method: '',
					record_name: '_x-challenge.acme.org',
					record_value: 'x-domain-verification=def',
					verified_at: null,
					checked_at: null
				}
			],
			canWrite: true,
			oncheck: () => {}
		}));

	it('a text input with a hint and an error', () =>
		expectAccessible(Input, {
			label: 'Slug',
			value: 'a b',
			hint: 'Lower case',
			error: 'Not a slug'
		}));

	it('a password input', () =>
		expectAccessible(PasswordInput, { label: 'Password', value: '', hint: 'At least 10' }));

	it('a switch', () =>
		expectAccessible(SwitchField, {
			label: 'Default role',
			description: 'Given to every new user.',
			checked: true
		}));

	it('a button', () => expectAccessible(Button, { children: text('Save') }));

	it('tabs', () =>
		expectAccessible(TabList, {
			label: 'Kinds of role',
			value: 'global',
			tabs: [
				{ value: 'global', label: 'Global roles', icon: 'ri-global-line', count: 3 },
				{ value: 'application', label: 'Application roles', icon: 'ri-apps-line', count: 0 }
			]
		}));

	it('an empty state', () =>
		expectAccessible(EmptyState, { icon: 'ri-links-line', title: 'No connections yet' }));

	it("an administrator's roles", () => expectAccessible(RoleCodes, { admin }));
});

describe('a field tells a screen reader what its hint says', () => {
	const hint = 'Space separated.';

	it('a switch', () =>
		expectDescribed(SwitchField, { label: 'Sync roles', description: hint, checked: false }, hint));
	it('an input', () => expectDescribed(Input, { label: 'Scopes', value: '', hint }, hint));
	it('a password', () =>
		expectDescribed(PasswordInput, { label: 'Secret', value: '', hint }, hint));
	it('a text area', () => expectDescribed(Textarea, { label: 'Metadata', value: '', hint }, hint));
	it('a select', () =>
		expectDescribed(
			Select,
			{ label: 'Matching', value: 'link', options: ['link', 'deny'], hint },
			hint
		));
	it('a date', () => expectDescribed(DatePicker, { label: 'Expires', value: '', hint }, hint));
	it("an error, which takes the hint's place", () =>
		expectDescribed(Input, { label: 'Scopes', value: '', hint, error: 'Too many.' }, 'Too many.'));
	it("a select's error", () =>
		expectDescribed(
			Select,
			{ label: 'Matching', value: 'link', options: ['link', 'deny'], hint, error: 'Pick one.' },
			'Pick one.'
		));
	it("a date's error", () =>
		expectDescribed(
			DatePicker,
			{ label: 'Expires', value: '', hint, error: 'Too soon.' },
			'Too soon.'
		));

	it('the kinds of application to choose from', () =>
		expectAccessible(ProfilePicker, { value: 'spa', onchange: () => {} }));

	it('what to give a new application, its secret included', () =>
		expectAccessible(ClientSetup, {
			application: {
				name: 'Shop',
				profile: 'web',
				issuer: 'https://id.example.com',
				client_id: 'abc123',
				has_secret: true,
				grant_types: ['authorization_code', 'refresh_token'],
				redirect_uris: ['https://shop.example.com/callback'],
				scopes: ['openid', 'email'],
				token_auth_method: 'client_secret_basic',
				dpop_bound_access_tokens: false,
				require_pushed_authorization_requests: false
			} as unknown as Application,
			secret: 'a-secret-shown-once'
		}));

	it('an empty list, with what to do next', () =>
		expectAccessible(DataTable, {
			columns: [{ key: 'name', label: 'Name' }],
			rows: [],
			empty: 'No applications yet.',
			emptyHint: 'Register an application to let it sign its users in here.',
			emptyAction: text('New application'),
			row: text('')
		}));

	it('a range to choose, one of a few', () =>
		expectAccessible(SegmentedControl, {
			label: 'Range',
			options: [
				{ value: '7', label: '7 days' },
				{ value: '14', label: '14 days' },
				{ value: '30', label: '30 days', disabled: true }
			],
			value: '14',
			onChange: () => {}
		}));
});
