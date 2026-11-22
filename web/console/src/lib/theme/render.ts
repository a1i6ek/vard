import type { Theme, Tokens } from './types';

const HEADER = (source: string) =>
	`// Generated from web/console/src/lib/theme/theme.ts by \`make generate\` — do not edit.\n// ${source}\n\n`;

const DARK_CHOSEN = ":root[data-theme='dark']";
const DARK_SYSTEM = ":root:not([data-theme='light'])";

function declarations(tokens: Partial<Tokens>, indent = '\t'): string {
	return Object.entries(tokens)
		.filter((entry): entry is [string, string] => entry[1] !== undefined)
		.map(([name, value]) => `${indent}--${name}: ${value};`)
		.join('\n');
}

function block(selector: string, body: string): string {
	return `${selector} {\n${body}\n}`;
}

function scaleTokens(theme: Theme): Tokens {
	return Object.fromEntries(
		theme.scales.flatMap(({ prefix, tokens }) =>
			Object.entries(tokens).map(([name, value]) => [prefix ? `${prefix}-${name}` : name, value])
		)
	);
}

/** The light palette and every scale, on the root: themes/_light.scss. */
export function renderLight(theme: Theme): string {
	const root = block(
		':root',
		[
			declarations(scaleTokens(theme)),
			declarations(theme.colors.light),
			`\tcolor-scheme: light;`
		].join('\n\n')
	);

	const responsive = theme.responsive.map(({ minWidth, tokens }) =>
		block(`@media (min-width: ${minWidth})`, block('\t:root', declarations(tokens, '\t\t')))
	);

	const layers = theme.layers.sets.map(({ prefix, selectors }) =>
		block(
			selectors.join(',\n'),
			Object.entries(theme.layers.swapped)
				.map(([page, name]) => `\t--${page}: var(--${prefix}-${name});`)
				.join('\n')
		)
	);

	return (
		HEADER('The scales and the light palette, on the root.') +
		[root, ...responsive, ...layers].join('\n\n') +
		'\n'
	);
}

/** The dark palette, chosen and by the system: themes/_dark.scss. */
export function renderDark(theme: Theme): string {
	const darkBody = [declarations(theme.colors.dark), '\tcolor-scheme: dark;'].join('\n\n');
	const darkChosen = block(DARK_CHOSEN, darkBody);
	const darkSystem = block(
		'@media (prefers-color-scheme: dark)',
		block(
			`\t${DARK_SYSTEM}`,
			darkBody
				.split('\n')
				.map((line) => (line ? `\t${line}` : line))
				.join('\n')
		)
	);

	return (
		HEADER('The dark palette: chosen, and by the system when nothing is.') +
		[darkChosen, darkSystem].join('\n\n') +
		'\n'
	);
}

/** Every token as a Sass variable naming its custom property, and the
 *  breakpoints as numbers a media query can read: abstracts/_tokens.scss,
 *  which writes no CSS. The values stay custom properties, so a theme is
 *  switched by the attribute on <html>, with no script. */
export function renderVariables(theme: Theme): string {
	const names = [...Object.keys(scaleTokens(theme)), ...Object.keys(theme.colors.light)];
	const variables = [...new Set(names)].map((name) => `$${name}: var(--${name});`);
	const breakpoints = Object.entries(theme.breakpoints).map(
		([name, width]) => `\t'${name}': ${width},`
	);

	return (
		HEADER('Every token, by its custom property, and the breakpoints.') +
		variables.join('\n') +
		'\n\n$breakpoints: (\n' +
		breakpoints.join('\n') +
		'\n);\n'
	);
}

export function renderFonts(theme: Theme): string {
	const { baseUrl, subsets, faces } = theme.fonts;

	const rules = faces.flatMap((face) =>
		face.weights.flatMap((weight) =>
			face.subsets.map((subset) =>
				block(
					'@font-face',
					[
						`\tfont-family: '${face.family}';`,
						`\tfont-style: ${face.style};`,
						`\tfont-weight: ${weight};`,
						'\tfont-display: swap;',
						`\tsrc: url('${baseUrl}${face.file(subset, weight)}') format('woff2');`,
						`\tunicode-range: ${subsets[subset]};`
					].join('\n')
				)
			)
		)
	);

	return (
		HEADER('The typefaces the panel bundles (src/lib/assets/fonts).') + rules.join('\n\n') + '\n'
	);
}
