// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: process.env.SITE_URL ?? 'https://medcrm.ru',
  trailingSlash: 'always',
  build: { inlineStylesheets: 'always' }, // one HTML request for first paint
  integrations: [
    sitemap({
      i18n: {
        defaultLocale: 'ru',
        locales: { ru: 'ru-RU', en: 'en-US', zh: 'zh-CN', es: 'es-ES' },
      },
    }),
  ],
});
