---
name: secure-frontend
description: Use when writing or reviewing security-sensitive code in apps/web (Next.js) or apps/landing (Astro) — CSP and headers, XSS, auth/session handling in the client, third-party scripts and analytics, client-side storage, file preview, source maps, data leakage via URLs, caches or browser.
---

# Безопасность фронта (CRM + лендинг)

## Сессия в браузере
- Токен только в `HttpOnly` cookie, выставленной Go-бэкендом. JS его не видит и не хранит. Никаких токенов в `localStorage`/`sessionStorage`/URL.
- RSC/Route Handlers проксируют cookie к Go при серверных запросах; права всегда проверяет бэкенд, Next — только UX.
- Автовыход по idle-таймауту (синхронно с сервером) + предупреждение за минуту; при выходе — `queryClient.clear()`.

## ПДн в клиенте — не утекать
- **URL**: никаких ФИО/телефонов/диагнозов в path/query/hash (попадают в историю, логи прокси, Referer). Только id.
- **Хранилище**: ПДн не в `localStorage`/IndexedDB/cookies. Черновики осмотра — на сервере (автосохранение API), не в браузере.
- **TanStack Query**: ключи из id, не из ПДн; persister для кэша с ПДн не использовать; `gcTime` умеренный.
- **Next кэш**: страницы с ПДн — динамические, без `'use cache'`/ISR/статической генерации; ответы `Cache-Control: no-store`.
- **Логи/ошибки**: `console.*` с данными пациента запрещён (Biome правило); error boundary показывает общий текст.
- **Заголовок вкладки/уведомления браузера**: без ФИО и диагнозов (видно на экране/в шаринге).
- **Печать/экспорт**: только через серверный эндпоинт (аудит + водяной знак), не `window.print()` страницы с картой.

## XSS
- React экранирует — не ломать: `dangerouslySetInnerHTML` запрещён, кроме санитайзенного (`DOMPurify`) контента в одном обёрточном компоненте `SafeHtml`.
- Ответы ИИ и текст из медкарты рендерить как текст/markdown через безопасный рендерер без raw HTML.
- `href` из данных — только `https:`/`mailto:`/`tel:`, без `javascript:`.
- Astro: `set:html` только для собственного контента из коллекций, не из форм.

## CSP и заголовки
CRM (`apps/web` middleware/proxy, nonce на запрос):
```
Content-Security-Policy: default-src 'self'; script-src 'self' 'nonce-{N}' 'strict-dynamic';
  style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https://<files-host>;
  connect-src 'self' https://<api-host>; frame-ancestors 'none'; base-uri 'self';
  form-action 'self'; object-src 'none'; upgrade-insecure-requests
```
+ `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` (CRM) / `strict-origin-when-cross-origin` (лендинг), `Permissions-Policy: camera=(), geolocation=(), microphone=(self)` (микрофон — для записи приёма).
Лендинг: CSP без `unsafe-eval`, allowlist только для разрешённой аналитики.

## Сторонние скрипты
- **В CRM — ноль сторонних скриптов**: никакой Метрики/GA/чатов/виджетов/шрифтов с CDN. Шрифты self-hosted. Мониторинг ошибок — self-hosted (РФ) с вычисткой ПДн.
- Лендинг: аналитика только после согласия на cookies, без Webvisor/записи сессий на страницах с формами, данные форм в события не передавать. SRI для любого внешнего скрипта.

## Формы
- Валидация на клиенте — UX; на сервере — обязательна.
- Лендинг: honeypot + rate limit на бэкенде, явное отдельное согласие на обработку ПДн с ссылкой на политику, без предзаполнения.
- `autocomplete="off"` для полей паспорта/СНИЛС на общих компьютерах ресепшена; `autocomplete="one-time-code"` для TOTP.

## Файлы
- Просмотр сканов/PDF — через короткоживущий pre-signed URL, в `<iframe sandbox>` или нативном просмотрщике; без загрузки сторонних PDF-библиотек с CDN.
- Загрузка — проверка размера/типа на клиенте как UX, реальная проверка — на сервере.

## Сборка
- Production source maps не публикуются (загружаются только в self-hosted мониторинг).
- `NEXT_PUBLIC_*` — только неконфиденциальное. Секреты на клиенте отсутствуют по определению.
- Зависимости: `pnpm audit` в CI, lockfile коммитится, новые пакеты — с обоснованием.

## Анти-паттерны (сразу исправлять)
токен в `localStorage` · ПДн в URL/`document.title`/query key · `dangerouslySetInnerHTML` с данными · Метрика/виджет в CRM · ISR/`'use cache'` на странице пациента · `console.log(patient)` · публичные source maps
