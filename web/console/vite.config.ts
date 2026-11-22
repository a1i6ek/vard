import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
	// id 5173, console 5174. Under `make dev` the browser opens these: the dev
	// server renders the pages, and src/hooks.server.ts passes the API's paths
	// on to the API, as the built server does in production.
	server: {
		port: 5174,
		strictPort: true
	},

	// Every stylesheet may @use the app's own Sass by name (src/lib/scss):
	// `@use 'abstracts' as *` in a component for tokens, breakpoints and
	// mixins.
	css: {
		preprocessorOptions: {
			scss: { loadPaths: [fileURLToPath(new URL('./src/lib/scss', import.meta.url))] }
		}
	},

	// The query library ships its components as .svelte files. Node cannot
	// import those, so while rendering on the server Vite has to compile it
	// with everything else rather than leaving it to the runtime.
	ssr: {
		noExternal: ['@tanstack/svelte-query']
	},

	plugins: [
		sveltekit({
			// <style lang="scss"> through Sass, so a component can @use the
			// app's abstracts; without it the block is sent as written
			// and the browser drops every rule that uses a mixin.
			preprocess: vitePreprocess(),

			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// A Node server (`node build`) that renders each page before sending
			// it, title and all, and passes the API's paths on (hooks.server.ts).
			adapter: adapter(),

			// Where browsers reach the app, given when it is built (ORIGIN): its
			// own origin, which SvelteKit's form check and every absolute URL
			// use. Without it adapter-node guesses from the request's headers and
			// takes https when none say, which plain http cannot pass.
			paths: { origin: process.env.ORIGIN || undefined },

			// Every script is the app's own: the ones SvelteKit writes into a
			// page carry a fresh nonce, the rest are files from 'self', so an
			// injected <script> does not run. Styles stay inline-able (Svelte's
			// transitions and component style attributes need it); images may be
			// logos and avatars on any https address an administrator gave. The
			// policy is sent as a header, so it can forbid framing too.
			csp: {
				mode: 'auto',
				directives: {
					'default-src': ['self'],
					'script-src': ['self'],
					'style-src': ['self', 'unsafe-inline'],
					'img-src': ['self', 'data:', 'https:'],
					'font-src': ['self'],
					'connect-src': ['self'],
					'object-src': ['none'],
					'base-uri': ['none'],
					'form-action': ['self'],
					'frame-ancestors': ['none']
				}
			}
		})
	],
	// Components mounted in a test need Svelte's browser build; the
	// accessibility specs mount them in jsdom.
	resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
	test: {
		expect: { requireAssertions: true },
		environment: 'node',
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
