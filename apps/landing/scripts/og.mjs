// Renders Open Graph images (1200×630) per locale into public/og/<lang>.png.
// Usage: pnpm og   (re-run after changing hero copy or brand)
import { chromium } from 'playwright';
import { mkdirSync } from 'node:fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { ru } from '../src/i18n/ru.ts';
import { en } from '../src/i18n/en.ts';
import { zh } from '../src/i18n/zh.ts';
import { es } from '../src/i18n/es.ts';

const dicts = { ru, en, zh, es };
const out = fileURLToPath(new URL('../public/og/', import.meta.url));
mkdirSync(out, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1200, height: 630 }, deviceScaleFactor: 1 });
await page.goto(pathToFileURL(fileURLToPath(new URL('./og/og.html', import.meta.url))).href);
await page.evaluate(() => document.fonts.ready);

for (const [lang, d] of Object.entries(dicts)) {
  await page.evaluate((data) => window.render(data), { lang, brand: 'MedCRM', title: d.hero.title, tagline: d.footer.tagline });
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: `${out}${lang}.png` });
  console.log(`og/${lang}.png`);
}
await browser.close();
