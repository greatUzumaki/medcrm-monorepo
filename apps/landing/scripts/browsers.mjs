// Cross-browser smoke test: Chromium, WebKit (Safari engine), Firefox.
// Usage: node scripts/browsers.mjs [baseUrl] [screenshotDir]
// Needs a running server (pnpm build && pnpm preview) and `npx playwright install chromium webkit firefox`.
import { chromium, firefox, webkit } from 'playwright';
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

const base = process.argv[2] ?? 'http://127.0.0.1:4321';
const shots = process.argv[3];
if (shots) mkdirSync(shots, { recursive: true });

const engines = { chromium, webkit, firefox };
const locales = ['', 'en/', 'zh/', 'es/'];
const failures = [];
const fail = (where, msg) => failures.push(`${where}: ${msg}`);

for (const [name, engine] of Object.entries(engines)) {
  const browser = await engine.launch();
  for (const scheme of ['light', 'dark']) {
    for (const vp of [
      { tag: 'desktop', width: 1440, height: 900 },
      { tag: 'mobile', width: 390, height: 844 },
    ]) {
      const ctx = await browser.newContext({ viewport: vp, colorScheme: scheme, locale: 'ru-RU' });
      const page = await ctx.newPage();
      const errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));

      for (const loc of locales) {
        const where = `${name}/${scheme}/${vp.tag}/${loc || 'ru/'}`;
        await page.goto(base + '/' + loc, { waitUntil: 'load' });
        await page.waitForTimeout(1500); // helix boots on idle

        const r = await page.evaluate(() => {
          const h1 = document.querySelector('h1')?.getBoundingClientRect();
          return {
            overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
            h1: !!h1 && h1.width > 0 && h1.height > 0,
            helix: document.querySelector('[data-helix]')?.classList.contains('on') ?? false,
            bg: getComputedStyle(document.body).backgroundColor,
          };
        });
        if (r.overflow > 0) fail(where, `horizontal overflow ${r.overflow}px`);
        if (!r.h1) fail(where, 'h1 not visible');
        if (!r.helix) fail(where, 'helix canvas did not start');

        if (loc === '') {
          // Mobile menu: opens as a modal dialog, closes with Escape.
          if (vp.tag === 'mobile') {
            await page.click('[data-menu-open]');
            await page.waitForTimeout(800);
            const open = await page.evaluate(() => document.getElementById('mobile-menu')?.matches(':modal'));
            if (!open) fail(where, 'mobile menu did not open as modal');
            if (shots) await page.screenshot({ path: join(shots, `${name}-${scheme}-mobile-menu.png`) });
            await page.keyboard.press('Escape');
            await page.waitForTimeout(600);
            const closed = await page.evaluate(() => !document.getElementById('mobile-menu')?.open);
            if (!closed) fail(where, 'Escape did not close the menu');
          }

          if (shots) await page.screenshot({ path: join(shots, `${name}-${scheme}-${vp.tag}-hero.png`) });

          // Form: empty submit must show a field error and move focus to the first invalid field.
          await page.locator('#contact').scrollIntoViewIfNeeded();
          const tops = () =>
            page.evaluate(() => ['lead-email', 'lead-doctors', 'lead-consent'].map((id) => Math.round(document.getElementById(id).getBoundingClientRect().top + scrollY)));
          const before = await tops();
          await page.click('[data-lead-form] .submit');
          if (String(before) !== String(await tops())) fail(where, 'validation errors shifted the form layout');
          const formOk = await page.evaluate(
            () => document.getElementById('lead-name-err')?.textContent?.length > 0 && document.activeElement?.id === 'lead-name',
          );
          if (!formOk) fail(where, 'form validation did not show error / focus first field');

          if (shots && vp.tag === 'desktop') {
            for (const id of ['product', 'ai', 'security']) {
              await page.locator('#' + id).scrollIntoViewIfNeeded();
              await page.waitForTimeout(id === 'ai' ? 4500 : 700);
              await page.screenshot({ path: join(shots, `${name}-${scheme}-desktop-${id}.png`) });
            }
          }
        }
      }
      if (errors.length) fail(`${name}/${scheme}/${vp.tag}`, `console/page errors: ${[...new Set(errors)].join(' | ')}`);
      await ctx.close();
    }
  }
  await browser.close();
  console.log(`${name}: done`);
}

if (failures.length) {
  console.log(`\nFAIL (${failures.length})\n- ` + failures.join('\n- '));
  process.exit(1);
}
console.log('\nOK: all engines × themes × viewports × locales passed');
