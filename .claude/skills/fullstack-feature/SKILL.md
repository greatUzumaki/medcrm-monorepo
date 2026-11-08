---
name: fullstack-feature
description: Use when building a new MedCRM feature that spans the OpenAPI contract, Postgres, Go backend and Next.js UI (e.g. "добавь лист ожидания", "сделай модуль счетов"). Gives the vertical-slice order, file locations and the done-checklist.
---

# Вертикальный срез фичи

Порядок фиксирован — каждый слой опирается на предыдущий.

## 0. План (в чат, коротко)
- Сущности, новые/изменённые таблицы
- Права (`internal/platform/authz/permissions.go`) и какие роли их получают
- Аудит: какие действия пишутся
- **ПДн**: какие поля, цель обработки, нужно ли шифрование, кто видит → скилл `pdn-152fz`
- ИИ? → скилл `ai-feature`. Горячий путь? → `go-reliability`

## 1. Контракт — `packages/contracts/openapi.yaml`
Пути, схемы (`AppointmentCreate`, `Appointment`), ошибки через общий `Problem`, `x-permission`. Ограничения прямо в схеме (`maxLength`, `format: uuid`, `minimum`) — из них генерируются и Go-валидация, и zod.

## 2. БД — скилл `db-change`
Миграция → RLS → queries.

## 3. Генерация
`make -C apps/api gen && pnpm gen:contracts`

## 4. Backend — скилл `go-endpoint`
`internal/<domain>/service.go` + `handler.go`, регистрация в `cmd/api` (strict handler уже требует реализации метода — компилятор не даст забыть).

## 5. Web — скилл `ui-craft`
- Маршрут: `apps/web/src/app/(clinic)/<domain>/page.tsx`
- Данные: сгенерированные orval-хуки из `@medcrm/contracts`; в RSC — серверный fetch с проброшенной cookie
- Формы: сгенерированная zod-схема + `zodResolver`
- Optimistic update — только для обратимых действий (не для медзаписей)

## 6. Тесты
- Go: service unit + handler integration (401/403/404-чужая клиника/аудит/нет ПДн в логах)
- Web: Vitest для логики, Playwright e2e для критичного флоу

## Done-чеклист
- [ ] `make -C apps/api lint test` и `pnpm typecheck lint test` зелёные
- [ ] tenant-изоляция и права покрыты тестами
- [ ] аудит для медданных
- [ ] UI: loading / empty / error / forbidden, проверено в браузере
- [ ] нет ПДн в логах, URL, query keys, клиентском хранилище
- [ ] агент `phi-guard` прогнан, если затронуты ПДн/auth/ИИ/экспорт/файлы
