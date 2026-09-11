import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { sidebar } from './src/sidebar.mjs';
import { markdownHeaders } from './src/integrations/md-headers.mjs';
import { SITE_URL } from './src/site.mjs';

const siteUrl = SITE_URL;
const ogImagePath = '/og-image.png';
const ogImageUrl = `${siteUrl}${ogImagePath}`;
const ogImageAlt =
  'REP social card showing Runtime Environment Protocol for secure runtime environment variables in browser apps';

export default defineConfig({
  site: siteUrl,
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
            property: 'og:type',
            content: 'website',
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:site_name',
            content: 'REP — Runtime Environment Protocol',
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:image',
            content: ogImageUrl,
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:image:secure_url',
            content: ogImageUrl,
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:image:type',
            content: 'image/png',
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:image:width',
            content: '1200',
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:image:height',
            content: '630',
          },
        },
        {
          tag: 'meta',
          attrs: {
            property: 'og:image:alt',
            content: ogImageAlt,
          },
        },
        {
          tag: 'meta',
          attrs: {
            name: 'twitter:card',
            content: 'summary_large_image',
          },
        },
        {
          tag: 'meta',
          attrs: {
            name: 'twitter:image',
            content: ogImageUrl,
          },
        },
        {
          tag: 'meta',
          attrs: {
            name: 'twitter:image:alt',
            content: ogImageAlt,
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'image_src',
            href: ogImageUrl,
          },
        },
        {
          tag: 'meta',
          attrs: {
            name: 'keywords',
            content:
              'runtime environment variables, docker frontend config, inject env vars browser, runtime config react, environment variables containers, build once deploy anywhere, frontend environment variables docker, runtime configuration protocol',
          },
        },
      ],
      sidebar,
      components: {
        Head: './src/components/Head.astro',
      },
    }),
    markdownHeaders({ extraPaths: ['/llms.txt', '/llms-full.txt'] }),
  ],
});
