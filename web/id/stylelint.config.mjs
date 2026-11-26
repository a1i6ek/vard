// Stylelint for this app (make lint). Colours,
// spacing, radii, font sizes, line heights and shadows come from the theme's
// tokens: a raw value of any of them is refused outside the files that
// define the tokens. A fixed dimension (an icon box, a border's width) may
// still be written as it is.
const tokensOnly = {
	'color-no-hex': true,
	'color-named': 'never',
	'function-disallowed-list': ['rgb', 'rgba', 'hsl', 'hsla', 'hwb', 'lab', 'lch', 'oklab', 'oklch'],
	'declaration-property-unit-disallowed-list': {
		'/^(padding|margin|gap|row-gap|column-gap|inset)/': ['px'],
		'/^border(-[a-z]+)*-radius$/': ['px'],
		'font-size': ['px'],
		'line-height': ['px'],
		'box-shadow': ['px']
	}
};

/** @type {import('stylelint').Config} */
export default {
	extends: ['stylelint-config-standard'],
	rules: {
		...tokensOnly,
		// Vite inlines @import only in its bare-string form.
		'import-notation': null,
		// Svelte's own: a selector that reaches past the component.
		'selector-pseudo-class-no-unknown': [true, { ignorePseudoClasses: ['global'] }],
		// Media queries keep the min-/max-width the code is written in.
		'media-feature-range-notation': 'prefix',
		// Kebab case, and the flow editor library's own svelte-flow__ names.
		'selector-class-pattern': [
			'^([a-z][a-z0-9]*)(-[a-z0-9]+)*$|^svelte-flow__[a-z-]+$',
			{ message: 'Expected a kebab-case class' }
		],
		// Each component's styles are scoped to it, so the order of two of its
		// selectors rarely decides which wins across components, which is what
		// this rule guards; it misfires on nearly every component.
		'no-descending-specificity': null
	},
	overrides: [
		{ files: ['**/*.svelte'], customSyntax: 'postcss-html' },
		{
			// Where the tokens are made: the theme's generated stylesheet,
			// with font names as each platform spells them.
			files: ['src/lib/styles/themes.css', 'src/lib/styles/ads.css'],
			rules: {
				...Object.fromEntries(Object.keys(tokensOnly).map((rule) => [rule, null])),
				'value-keyword-case': null,
				'font-family-name-quotes': null,
				'color-hex-length': null,
				'custom-property-empty-line-before': null
			}
		}
	],
	ignoreFiles: ['**/node_modules/**', '**/build/**', '**/.svelte-kit/**']
};
