// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { rehypeBasePath } from './rehype/base-path.js';

const base = '/';

// https://astro.build/config
export default defineConfig({
	site: 'https://nhlmg93.github.io',
	base,
	markdown: {
		rehypePlugins: [[rehypeBasePath, base]],
	},
	integrations: [
		starlight({
			title: 'shux',
			description: 'An experimental terminal multiplexer with resurrection and Neovim-style Lua configuration.',
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/nhlmg93/shux',
				},
			],
			sidebar: [
				{
					label: 'Introduction',
					items: [{ label: 'Welcome', slug: 'index' }],
				},
				{
					label: 'Getting started',
					items: [
						{ label: 'Install and build', slug: 'getting-started/install' },
						{ label: 'First run', slug: 'getting-started/first-run' },
					],
				},
				{
					label: 'Concepts',
					items: [
						{ label: 'Daemon and clients', slug: 'concepts/daemon' },
						{ label: 'Sessions, windows, and panes', slug: 'concepts/layout' },
					],
				},
				{
					label: 'Using shux',
					items: [
						{ label: 'Keybindings', slug: 'using/keybindings' },
						{ label: 'Scrolling', slug: 'using/scrolling' },
					],
				},
				{
					label: 'Configuration',
					items: [
						{ label: 'Overview', slug: 'configuration/overview' },
						{ label: 'Options', slug: 'configuration/options' },
						{ label: 'Keymaps', slug: 'configuration/keymaps' },
					],
				},
				{
					label: 'Plugins',
					items: [{ label: 'Overview', slug: 'plugins/overview' }],
				},
				{
					label: 'Resurrection',
					items: [{ label: 'Recovery model', slug: 'resurrection/overview' }],
				},
				{
					label: 'CLI',
					items: [{ label: 'Commands', slug: 'cli/commands' }],
				},
				{
					label: 'Help',
					items: [{ label: 'FAQ', slug: 'faq' }],
				},
			],
		}),
	],
});
