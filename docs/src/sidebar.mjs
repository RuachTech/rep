/**
 * Single source of truth for documentation ordering.
 *
 * Consumed by the Starlight sidebar and by the `/llms.txt` route, so the
 * machine-readable index always mirrors what a human sees in the nav.
 */
export const sidebar = [
  {
    label: 'Getting Started',
    items: [
      { label: 'Quick Start', slug: 'quick-start' },
      { label: 'Installation', slug: 'guides/installation' },
      { label: 'Local Development', slug: 'guides/development' },
    ],
  },
  { label: 'For AI Agents', slug: 'agents' },
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
      { label: 'Todo App (React)', slug: 'examples/todo-react' },
      { label: 'Simple HTML (ESM.sh)', slug: 'examples/simple-html' },
      { label: 'Next.js — Proxy Mode', slug: 'examples/nextjs-proxy' },
      {
        label: 'Next.js CSR + Kubernetes',
        slug: 'examples/nextjs-csr-embedded',
      },
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
      {
        label: 'Build-Tool Plugins',
        items: [
          { label: 'Vite', slug: 'reference/plugins/vite' },
          { label: 'Next.js', slug: 'reference/plugins/next' },
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
];
