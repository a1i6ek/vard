import { describe, expect, it } from 'vitest';

import { renderDark, renderFonts, renderLight, renderVariables } from './render';
import { theme } from './theme';

describe('renderLight and renderDark', () => {
	const css = renderLight(theme) + renderDark(theme);
	const darkChosen = renderDark(theme);

	it('declares every light colour on the root', () => {
		for (const name of Object.keys(theme.colors.light)) {
			expect(css).toContain(`--${name}: `);
		}
	});

	it('writes the dark palette twice, chosen and by the system, with the same values', () => {
		for (const [name, value] of Object.entries(theme.colors.dark)) {
			const declaration = `--${name}: ${value};`;
			expect(darkChosen.split(declaration).length - 1).toBe(2);
		}
	});

	it('swaps the page surfaces inside every floating layer', () => {
		expect(css).toContain('--color-surface: var(--popover-surface);');
		expect(css).toContain('--color-surface: var(--dialog-surface);');
	});

	it('gives a dark colour only a name the light palette has', () => {
		const light = new Set(Object.keys(theme.colors.light));
		expect(Object.keys(theme.colors.dark).filter((name) => !light.has(name))).toEqual([]);
	});
});

describe('renderFonts', () => {
	it('writes one face for every weight of every subset', () => {
		const faces = theme.fonts.faces.reduce(
			(count, face) => count + face.weights.length * face.subsets.length,
			0
		);
		expect(renderFonts(theme).split('@font-face').length - 1).toBe(faces);
	});
});

describe('renderVariables', () => {
	const scss = renderVariables(theme);

	it('names every colour by its custom property, so the theme still switches without a script', () => {
		for (const name of Object.keys(theme.colors.light)) {
			expect(scss).toContain(`$${name}: var(--${name});`);
		}
	});

	it('writes every breakpoint as a number a media query can read', () => {
		for (const [name, width] of Object.entries(theme.breakpoints)) {
			expect(scss).toContain(`'${name}': ${width},`);
		}
	});
});
