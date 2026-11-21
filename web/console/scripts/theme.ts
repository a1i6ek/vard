/**
 * Writes the shared styles' generated files from lib/theme, for both apps:
 * web/packages/styles/scss/abstracts/_tokens.scss, themes/_light.scss,
 * themes/_dark.scss and base/_fonts.scss.
 *
 *   bun run theme          write them
 *   bun run theme:check    fail if either is behind the theme (run by check)
 */
import { readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import prettier from 'prettier';
import { renderDark, renderFonts, renderLight, renderVariables } from '../src/lib/theme/render';
import { theme } from '../src/lib/theme/theme';

const styles = new URL('../../packages/styles/scss/', import.meta.url);

const outputs = [
	{ file: new URL('abstracts/_tokens.scss', styles), css: renderVariables(theme) },
	{ file: new URL('themes/_light.scss', styles), css: renderLight(theme) },
	{ file: new URL('themes/_dark.scss', styles), css: renderDark(theme) },
	{ file: new URL('base/_fonts.scss', styles), css: renderFonts(theme) }
];

const checking = process.argv.includes('--check');
const stale: string[] = [];

for (const { file, css } of outputs) {
	const path = fileURLToPath(file);
	const options = await prettier.resolveConfig(path);
	const formatted = await prettier.format(css, { ...options, filepath: path });

	const current = await readFile(path, 'utf8').catch(() => '');
	if (current === formatted) continue;

	if (checking) stale.push(path);
	else await writeFile(path, formatted);
}

if (stale.length > 0) {
	console.error(
		`These files do not match lib/theme/theme.ts — the theme changed, or they were edited by hand:\n  ${stale.join('\n  ')}\nRun \`bun run theme\`.`
	);
	process.exit(1);
}
