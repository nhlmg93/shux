// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'shux',
      favicon: '/favicon.svg',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/nathanhelmig/shux' },
      ],
      editLink: {
        baseUrl: 'https://github.com/nhelmig/shux/edit/main/docs/',
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            'introduction',
            'installation',
            'quick-start',
          ],
        },
        {
          label: 'Concepts',
          items: [
            'concepts/sessions',
            'concepts/windows',
            'concepts/panes',
            'concepts/recovery',
            'concepts/ghostty',
          ],
        },
        {
          label: 'Configuration',
          items: [
            'config/lua-config',
            'config/keybindings',
            'config/options',
          ],
        },
        {
          label: 'Plugins',
          items: [
            'plugins/overview',
            'plugins/writing-plugins',
          ],
        },
        {
          label: 'Reference',
          items: [
            'reference/commands',
            'reference/actions',
            'reference/protocol',
          ],
        },
      ],
    }),
  ],
});