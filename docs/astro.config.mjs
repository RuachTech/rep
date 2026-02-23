import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://rep-protocol.dev',
  integrations: [
    starlight({
      title: 'REP',
      logo: {
        light: './src/assets/logo-light.jpg',
        dark: './src/assets/logo-dark.jpg',
        replacesTitle: true,
      },
      description:
        'Runtime Environment Protocol — Securely deliver environment variables to browser apps at runtime. No rebuilds, no secrets in bundles.',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/ruachtech/rep',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/ruachtech/rep/edit/main/docs/',
      },
      lastUpdated: true,
      pagination: true,
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      customCss: ['./src/styles/custom.css'],
      head: [
        {
          tag: 'meta',
          attrs: {
            property: 'og:image',
            content: '/og-image.png',
          },
        },
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Quick Start', slug: 'quick-start' },
            { label: 'Installation', slug: 'guides/installation' },
            { label: 'Local Development', slug: 'guides/development' },
          ],
        },
        {
          label: 'Core Concepts',
          items: [
            { label: 'How REP Works', slug: 'concepts/how-it-works' },
            {
              label: 'Variable Classification',
              slug: 'concepts/variable-classification',
            },
            { label: 'Security Model', slug: 'concepts/security-model' },
            { label: 'Wire Format', slug: 'concepts/wire-format' },
            { label: 'Hot Reload', slug: 'concepts/hot-reload' },
          ],
        },
        {
          label: 'Framework Guides',
          items: [
            { label: 'React', slug: 'frameworks/react' },
            { label: 'Vue', slug: 'frameworks/vue' },
            { label: 'Svelte', slug: 'frameworks/svelte' },
            { label: 'Angular', slug: 'frameworks/angular' },
            { label: 'Vanilla JS', slug: 'frameworks/vanilla' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Manifest File', slug: 'guides/manifest' },
            { label: 'Testing', slug: 'guides/testing' },
            {
              label: 'Migration',
              items: [
                { label: 'Overview', slug: 'guides/migration/overview' },
                { label: 'From Vite', slug: 'guides/migration/from-vite' },
                {
                  label: 'From Create React App',
                  slug: 'guides/migration/from-cra',
                },
                { label: 'From Next.js', slug: 'guides/migration/from-next' },
              ],
            },
          ],
        },
        {
          label: 'Deployment',
          items: [
            { label: 'Docker — Proxy Mode', slug: 'deployment/docker-proxy' },
            {
              label: 'Docker — Embedded Mode',
              slug: 'deployment/docker-embedded',
            },
            { label: 'Kubernetes', slug: 'deployment/kubernetes' },
            { label: 'Docker Compose', slug: 'deployment/docker-compose' },
            { label: 'CI/CD Pipeline', slug: 'deployment/ci-cd' },
          ],
        },
        {
          label: 'Examples',
          items: [
            {
              label: 'Zero-Config Quick Start',
              slug: 'examples/quick-no-manifest',
            },
            { label: 'Todo App (React)', slug: 'examples/todo-react' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'SDK API', slug: 'reference/sdk' },
            { label: 'Gateway Flags', slug: 'reference/gateway-flags' },
            {
              label: 'Gateway Endpoints',
              slug: 'reference/gateway-endpoints',
            },
            { label: 'Manifest Schema', slug: 'reference/manifest-schema' },
            { label: 'CLI Commands', slug: 'reference/cli' },
            {
              label: 'Adapter APIs',
              items: [
                { label: 'React', slug: 'reference/adapters/react' },
                { label: 'Vue', slug: 'reference/adapters/vue' },
                { label: 'Svelte', slug: 'reference/adapters/svelte' },
              ],
            },
            { label: 'Codemod', slug: 'reference/codemod' },
          ],
        },
        {
          label: 'Specification',
          items: [
            { label: 'Overview', slug: 'spec' },
            { label: 'REP-RFC-0001', slug: 'spec/rfc-0001' },
            { label: 'Security Model', slug: 'spec/security-model' },
            { label: 'Conformance', slug: 'spec/conformance' },
          ],
        },
        { label: 'Contributing', slug: 'contributing' },
      ],
    }),
  ],
});
