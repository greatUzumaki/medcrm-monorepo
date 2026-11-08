---
name: landing-astro
description: Use when working on the marketing site in apps/landing (Astro) — pages, hero/feature/pricing sections, blog/MDX content, SEO, demo-request forms, performance. Not for the clinic CRM UI (that is ui-craft).
---

# Лендинг (Astro)

Цель — конверсия в заявку на демо и доверие клиник. Здесь, в отличие от CRM, выразительный дизайн уместен: крупная типографика, скриншоты продукта, motion. Но с бюджетом производительности.

## Структура
```
apps/landing/src/
  pages/            # index.astro, pricing.astro, blog/[...slug].astro, legal/*
  layouts/Base.astro  # <head>: SEO, OG, JSON-LD, шрифты
  components/sections/  # Hero, Features, AiShowcase, Pricing, Testimonials, Faq, Cta
  content/blog/     # MDX, коллекции с zod-схемой в content.config.ts
```

## Правила
- **0 JS по умолчанию.** `.astro` компоненты; island (`client:visible`, реже `client:load`) только для реально интерактивного: форма, калькулятор, переключатель тарифа. Islands на React, из `packages/ui` можно брать примитивы.
- Токены и шрифты — те же, что в `packages/ui` (бренд един с продуктом).
- Изображения — `astro:assets` `<Image>`/`<Picture>`, AVIF/WebP, явные размеры. Скриншоты CRM — только с синтетическими пациентами.
- Анимации — CSS / View Transitions, `prefers-reduced-motion` уважать.
- Бюджет: Lighthouse ≥ 95 по всем, LCP < 2.0s, CLS < 0.05, JS на главной < 50 KB.

## SEO
Уникальные `title`/`description` на страницу, canonical, OG-картинка, `@astrojs/sitemap`, robots.txt, JSON-LD (`Organization`, `SoftwareApplication`, `FAQPage`, `Article` для блога). Семантика: один `h1`, логичная иерархия.

## Форма заявки на демо
- island-форма → `POST` на `apps/api` `/public/leads` (zod-схема из `@medcrm/contracts`), rate limit + honeypot/captcha.
- Чекбокс согласия на обработку ПДн (152-ФЗ) с ссылкой на политику — обязателен, без предзаполнения. Ссылка на политику в футере.
- Лиды — отдельная таблица, не смешивать с пациентами клиник.
- Аналитика (Яндекс Метрика и т.п.) — только после согласия на cookies; в события не передавать введённые данные формы.

## Контент
Тексты — русский, без медицинских обещаний («вылечим», «гарантируем диагноз»). ИИ подаём как ассистента врача, не замену. Реклама медуслуг/ПО — соблюдать 38-ФЗ «О рекламе».

## i18n и темы
- Тексты только в `src/i18n/{ru,en,zh,es}.ts`; тип `Dict = typeof ru` — TS не даст пропустить ключ. ru в корне `/`, остальные `/<lang>/`. Автоопределение языка — inline-скрипт только на `/`.
- Цвета только токенами в `:root` через `light-dark(светлый, тёмный)` — одно объявление на цвет; тему выбирает `color-scheme` (система или `[data-theme]`). Не возвращать дублирующие тёмные блоки — на них уже дважды ловили баги.
- Текст на градиенте: `--iris-text` (крупный текст) / `--iris-fill` (белый мелкий текст). `--iris` — только декор.

## Подводные камни (проверено)
- **lightningcss склеивает `animation` + `animation-timeline` в невалидный шорткат** → scroll-driven анимация молча пропадает. Писать лонгхендами: `animation-name`, `animation-timing-function`, `animation-fill-mode`, `animation-timeline`.
- `::before { z-index:-1 }` внутри `isolation: isolate` рисуется **поверх** фона элемента. Градиентная рамка — `background: linear-gradient(var(--surface)) padding-box, var(--iris) border-box`.
- UA-стиль `[popover]` даёт `width: fit-content` — задавать ширину явно.
- Не анимировать `opacity` у LCP-элемента и у текста при scroll-анимациях (LCP откладывается, axe ловит провал контраста посреди анимации).
- `astro check` с TS 7 не работает — держать `typescript@^6`; `@emnapi/runtime` запинен в `pnpm-workspace.yaml` (бета падает на Node 26).
- Astro 7 `astro preview` уходит в демон: `astro preview stop` / `status`.
- pnpm 12 включает `minimumReleaseAge` (пакеты моложе суток не ставятся) — это защита supply chain, не отключать. Если lockfile не проходит: `pnpm clean --lockfile && pnpm install`.
- Атрибут с пустым значением (`data-x=""`) всё равно матчится селектором `[data-x]` — передавать `undefined`, чтобы атрибута не было.
- Ошибки формы: строка под ошибку зарезервирована всегда (`.err` с `min-height`), поля `align-content: start` — появление ошибок не должно двигать вёрстку (проверяет `test:browsers`).
- Элементы поверх canvas перехватывают курсор — интерактив спирали слушает `document`, а не контейнер.

## Проверка
1. `pnpm -F @medcrm/landing typecheck && pnpm -F @medcrm/landing build`
2. Нет кириллицы в en/zh/es: grep по `dist/{en,zh,es}` (кроме «Русский» в переключателе)
3. Lighthouse mobile на `/`, `/zh/`, `/privacy/`: цель 100/100/100/100, LCP < 2 s, CLS < 0.05
4. `pnpm test:browsers` (при запущенном preview) — Chromium/WebKit/Firefox; плюс глазами скриншоты: `node scripts/browsers.mjs <url> <dir>`
5. Изменился hero-текст → `pnpm og`, посмотреть `public/og/*.png`
6. a11y: контраст новых пар токенов (≥4.5 текст, ≥3 крупный текст и границы UI), клавиатура, фокус
