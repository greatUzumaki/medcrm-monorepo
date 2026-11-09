// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: process.env.SITE_URL ?? 'https://openmedai.ru',
  trailingSlash: 'always',
  build: { inlineStylesheets: 'always' }, // one HTML request for first paint
  integrations: [
    sitemap({
      // Same codes as <link hreflang> in Base.astro: one consistent signal for search engines.
      i18n: { defaultLocale: 'ru', locales: { ru: 'ru', en: 'en', zh: 'zh-Hans', es: 'es' } },
      // Legal pages are noindex (the Russian text is duplicated on every locale).
      filter: (url) => !/\/(privacy|consent)\/$/.test(url),
    }),
  ],
});
