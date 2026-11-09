# OpenMedAI (кодовое имя репо — medcrm)

Бренд **OpenMedAI**, домен **openmedai.ru** (`apps/landing/src/site.ts`, `astro.config.mjs`). Внутренние имена пакетов `@medcrm/*` не переименовываем.

CRM для частных клиник со встроенным ИИ. Пациенты, расписание/записи, врачи, электронная медкарта (ЭМК), счета/оплаты, коммуникации (SMS/Telegram/email), ИИ-ассистент (саммари приёма, транскрибация, черновики заключений, семантический поиск по карте, no-show прогноз).

Обрабатываем **специальную категорию ПДн** (здоровье, ст. 10 152-ФЗ) и **врачебную тайну** (ст. 13 323-ФЗ). Безопасность — требование №1, производительность — №2, фичи — №3.

Язык UI — русский. Код, идентификаторы, коммиты — английский.

## Стек

| Слой | Технология |
|---|---|
| Backend | **Go** (актуальный стабильный, ≥ 1.25), `net/http` + chi, oapi-codegen (strict server), pgx v5 + sqlc, goose (миграции), River (фоновые задачи на Postgres), slog, OpenTelemetry + Prometheus |
| Контракт | `packages/contracts/openapi.yaml` (OpenAPI 3.1) — единственный источник правды; Go-интерфейсы через oapi-codegen, TS-типы/zod/хуки через orval |
| БД | PostgreSQL 17 + pgvector, Row Level Security, шифрование чувствительных полей на уровне приложения |
| Web (CRM) | Next.js 16 App Router, React 19, Tailwind v4, shadcn/ui, TanStack Query + Table, react-hook-form + zod (сгенерированные), next-intl (ru) |
| Landing | Astro 7, статика, scoped CSS + токены (`apps/landing/src/styles/global.css`, светлая/тёмная тема), i18n ru/en/zh/es, vanilla `<script>` без фреймворка |
| Auth | Свои сессии в Go: opaque-токен в `HttpOnly` cookie, хеш в БД, argon2id, TOTP 2FA для персонала, RBAC |
| ИИ | `apps/api/internal/ai`: OpenAI-совместимый клиент с настраиваемым `base_url` (РФ-провайдер / self-hosted), структурированный вывод по JSON Schema, SSE-стриминг |
| Тесты | Go: `testing` + testcontainers-go (реальный Postgres); TS: Vitest; e2e: Playwright; нагрузка: k6 |
| Качество | golangci-lint, gosec, govulncheck, Biome, tsc, gitleaks, lefthook |

## Структура

```
apps/
  api/                       Go-бэкенд
    cmd/api/                 HTTP-сервер
    cmd/worker/              River-воркеры (напоминания, транскрибация, эмбеддинги)
    internal/
      platform/              config, db (pool, withTenant), httpx, log, crypto, audit, authz, ratelimit
      api/                   сгенерированный oapi-codegen код (не править руками)
      store/                 сгенерированный sqlc код (не править руками)
      <domain>/              handler.go, service.go — patient, appointment, emr, billing, ai, ...
    db/queries/*.sql         SQL для sqlc
    db/migrations/*.sql      goose-миграции
  web/                       Next.js: UI для персонала клиники
  landing/                   Astro: маркетинговый сайт (без доступа к ПДн пациентов)
packages/
  contracts/                 openapi.yaml + сгенерированные TS (gen/)
  ui/                        дизайн-система: токены, shadcn, мед-компоненты
  config/                    tsconfig, biome
```

## Команды

```bash
docker compose up -d                 # postgres+pgvector, mailpit
make -C apps/api gen                 # oapi-codegen + sqlc после изменения openapi.yaml или queries
make -C apps/api run                 # API :4000
make -C apps/api test                # go test -race ./...
make -C apps/api lint                # golangci-lint (вкл. gosec) + govulncheck
make -C apps/api migrate             # goose up
pnpm gen:contracts                   # orval: TS-типы, zod, query-хуки из openapi.yaml
pnpm -F web dev                      # :3000
pnpm -F @medcrm/landing dev          # :4321
pnpm -F @medcrm/landing og           # перегенерировать OG-картинки (public/og/<lang>.png) после смены hero-текста
pnpm -F @medcrm/landing test:browsers  # smoke: Chromium + WebKit (Safari) + Firefox × темы × мобилка × локали (нужен preview)
pnpm typecheck && pnpm lint && pnpm test
```

Гейт «готово»: `make -C apps/api lint test` + `pnpm typecheck lint test` зелёные.

## Жёсткие правила (не нарушать без явного запроса)

### ПДн и врачебная тайна — подробно в скилле `pdn-152fz`
1. **Никаких ПДн в логах, ошибках, трейсах, метриках, Sentry, аналитике, URL.** Логируем `patient_id`, не ФИО/телефон/диагноз. Типы ПДн реализуют `slog.LogValuer` → `[REDACTED]`.
2. **Multi-tenant изоляция.** Каждая бизнес-таблица: `clinic_id` + RLS. Запросы только через `db.WithTenant(ctx, func(q *store.Queries) error {...})`. `clinic_id` берётся только из сессии.
3. **Аудит** чтения/изменения/экспорта/печати ЭМК — `audit.Write` в той же транзакции.
4. **RBAC на сервере** в каждом хендлере. Скрытие кнопки в UI — не защита.
5. **Шифрование** чувствительных полей (паспорт, СНИЛС, полис, свободный текст диагнозов) — `crypto.Seal/Open` (envelope, версия ключа), поиск — blind index (HMAC).
6. **Минимизация.** Не собирать и не возвращать поля «на всякий случай». DTO содержит только нужное экрану.
7. **Данные в РФ.** БД, бэкапы, логи, объектное хранилище, LLM с сырыми ПДн — только в РФ.
8. Реальные данные пациентов **никогда** не попадают в репо, тесты, seed, скриншоты, промпты Claude.

### ИИ — скилл `ai-feature`
1. ИИ не ставит диагнозы и не назначает лечение. Вывод — черновик, подтверждает врач. В UI пометка «Сгенерировано ИИ».
2. Внешний (вне периметра) провайдер получает только псевдонимизированный вход (`ai.Redact`).
3. Провенанс: модель, версия промпта, id источников, токены, кто подтвердил.
4. Промпты — файлы `internal/ai/prompts/<task>.v<N>.md`, eval-набор на каждую задачу.

### Go
- `context.Context` первым аргументом везде, таймауты на всё внешнее (БД, HTTP, LLM).
- Ошибки: `fmt.Errorf("...: %w", err)`, доменные ошибки — sentinel/типы в `<domain>`, маппинг в HTTP — только в `httpx`. Наружу — `application/problem+json` без внутренних деталей.
- Никаких глобальных мутабельных переменных, `init()` с логикой, `panic` в бизнес-коде.
- SQL только через sqlc. Конкатенация SQL запрещена.
- Сгенерированный код (`internal/api`, `internal/store`, `packages/contracts/gen`) не править — менять источник и `gen`.
- Зависимость — только если stdlib/существующей нет и выгода больше пары десятков строк.

### Общее
- Деньги — `int64` копейки. Время — `timestamptz` UTC, отображение в TZ клиники.
- Next.js: Server Components по умолчанию, `'use client'` только где интерактив.
- TS без `any`/`as`-обходов/`@ts-ignore`. Комментарии только про «почему».

## Workflow

1. Нетривиальная фича → короткий план: слои, миграции, права, аудит, какие ПДн затронуты.
2. Порядок: `openapi.yaml` → миграция → queries → `gen` → service → handler → web → тесты. Скилл `fullstack-feature`.
3. Любое изменение, касающееся ПДн / auth / ИИ / логов / экспорта / файлов → агент `phi-guard` до завершения.
4. Горячие пути (расписание, поиск пациентов, списки) → скилл `go-reliability`, бенчмарк/k6 при сомнении.

## Скиллы проекта (`.claude/skills/`)

| Скилл | Когда |
|---|---|
| `fullstack-feature` | новая фича сквозь все слои |
| `go-endpoint` | новый/изменённый HTTP-эндпоинт в `apps/api` |
| `go-reliability` | производительность, нагрузка, таймауты, пул, graceful shutdown, воркеры |
| `db-change` | схема, миграция, RLS, индексы, шифрованные колонки, sqlc-запросы |
| `ui-craft` | экраны и компоненты CRM в `apps/web` / `packages/ui` |
| `landing-astro` | `apps/landing` |
| `ai-feature` | LLM, эмбеддинги, транскрибация |
| `pdn-152fz` | всё, что касается ПДн: сбор, согласия, хранение, передача, удаление, инциденты, права субъекта |
| `secure-go` | безопасный код бэкенда: auth, сессии, валидация, крипто, SSRF, загрузки, заголовки |
| `secure-frontend` | безопасность web/landing: CSP, XSS, cookies, сторонние скрипты, утечки в клиенте |

Агенты (`.claude/agents/`): `phi-guard` (ревью ПДн/безопасности), `ui-reviewer` (UX/a11y).
