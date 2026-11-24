import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://docs.werbot.com',
  integrations: [
    starlight({
      title: 'Werbot',
      favicon: '/favicon.svg',
      customCss: ['./src/styles/custom.css'],
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/werbot/werbot' }],
      sidebar: [
        { label: 'Start', items: [{ slug: 'install' }, { slug: 'configuration' }, { slug: 'upgrading' }] },
        {
          label: 'Access',
          items: [{ slug: 'jit' }, { slug: 'policies' }, { slug: 'ssh-hardening' }, { slug: 'mfa' }, { slug: 'sso' }, { slug: 'cli' }],
        },
        { label: 'Agents', items: [{ slug: 'agents' }] },
        {
          label: 'Sessions',
          items: [{ slug: 'web-terminal' }, { slug: 'connect' }, { slug: 'vnet' }, { slug: 'connectors' }],
        },
        {
          label: 'Operations',
          items: [{ slug: 'gateway' }, { slug: 'web' }, { slug: 'scaling' }, { slug: 'failure-modes' }, { slug: 'security' }],
        },
        { label: 'Migration', items: [{ slug: 'migrate-from-boundary' }] },
      ],
    }),
  ],
});
