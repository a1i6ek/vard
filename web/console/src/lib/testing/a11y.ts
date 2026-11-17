import axe from 'axe-core';
import { mount, tick, unmount, type Component } from 'svelte';
import { expect } from 'vitest';

// jsdom has no matchMedia, which the theme reads as soon as its module loads; a test page
// prefers nothing. Imported before the components, so it is there when they load.
window.matchMedia ??= (query: string) =>
	({
		matches: false,
		media: query,
		onchange: null,
		addEventListener: () => {},
		removeEventListener: () => {},
		dispatchEvent: () => false
	}) as unknown as MediaQueryList;

/**
 * Mounts a component in the test's document and fails on anything axe finds. Colour contrast
 * is left out: jsdom lays nothing out, so it cannot be measured here.
 */
export async function expectAccessible<Props extends Record<string, unknown>>(
	component: Component<Props>,
	props: Props
): Promise<void> {
	const target = document.createElement('main');
	document.body.append(target);
	const mounted = mount(component, { target, props });

	try {
		const { violations } = await axe.run(target, {
			rules: { 'color-contrast': { enabled: false } }
		});
		const found = violations.map(
			(violation) =>
				`${violation.id}: ${violation.help} (${violation.nodes.map((node) => node.html).join(', ')})`
		);
		expect(found, `axe found:\n${found.join('\n')}`).toEqual([]);
	} finally {
		await unmount(mounted);
		target.remove();
	}
}

const controls =
	'input:not([type="hidden"]):not([aria-hidden="true"]), textarea, select:not([aria-hidden="true"]), [role="combobox"], [role="switch"]';

/**
 * Mounts a component and fails unless every control in it is described by the text: a hint
 * or description a screen reader is never told about is only seen (WCAG 2.2 1.3.1, 4.1.2).
 */
export async function expectDescribed<Props extends Record<string, unknown>>(
	component: Component<Props>,
	props: Props,
	text: string
): Promise<void> {
	const target = document.createElement('main');
	document.body.append(target);
	const mounted = mount(component, { target, props });
	// A part may learn after mounting which others are there (Ark's field its helper text).
	await tick();
	await new Promise((settled) => setTimeout(settled, 0));

	try {
		const found = [...target.querySelectorAll<HTMLElement>(controls)];
		expect(found.length, 'no control to describe').toBeGreaterThan(0);
		for (const control of found) {
			const description = (control.getAttribute('aria-describedby') ?? '')
				.split(/\s+/)
				.filter(Boolean)
				.map((id) => document.getElementById(id)?.textContent?.trim() ?? '')
				.join(' ');
			expect(description, `${control.outerHTML.slice(0, 120)} is not described`).toContain(text);
		}
	} finally {
		await unmount(mounted);
		target.remove();
	}
}
