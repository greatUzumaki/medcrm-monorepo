---
name: go-endpoint
description: Use when adding or changing an HTTP endpoint in the Go backend (apps/api) — OpenAPI spec, oapi-codegen strict handler, service, sqlc queries, RBAC, tenant scoping, audit, problem+json errors, pagination and integration tests.
---

# Эндпоинт в Go-бэкенде

## Порядок
1. `packages/contracts/openapi.yaml` — path, схемы запроса/ответа, коды ошибок (`400/401/403/404/409/422/429` → `Problem`), `x-permission: appointment:create`.
2. SQL в `apps/api/db/queries/<domain>.sql` (скилл `db-change`).
3. `make -C apps/api gen` → появятся методы `StrictServerInterface` и `store.Queries`.
4. `internal/<domain>/service.go` — логика. `internal/<domain>/handler.go` — тонкий адаптер.
5. `pnpm gen:contracts` — web получит типы/zod/хуки.

## Handler (strict server)
```go
func (h *Handler) CreateAppointment(ctx context.Context, req api.CreateAppointmentRequestObject) (api.CreateAppointmentResponseObject, error) {
	sess := authz.MustSession(ctx)                       // положен middleware, иначе 401 раньше
	if err := authz.Require(sess, authz.AppointmentCreate); err != nil {
		return nil, err                                  // httpx маппит в 403 problem+json
	}
	a, err := h.svc.Create(ctx, sess, *req.Body)
	if err != nil {
		return nil, err
	}
	return api.CreateAppointment201JSONResponse(toDTO(a)), nil
}
```
- `sess` = `{UserID, ClinicID, Role, RequestID, IP}` из cookie-сессии. `clinic_id` из body/query/path **не принимать**.
- Ответ — явный маппинг в DTO. Никогда не сериализовать `store.*` row напрямую (утечёт шифротекст, `deleted_at`, внутренние флаги).

## Service
```go
func (s *Service) Create(ctx context.Context, sess authz.Session, in api.AppointmentCreate) (Appointment, error) {
	var out Appointment
	err := s.db.WithTenant(ctx, sess.ClinicID, func(q *store.Queries) error {
		free, err := q.IsSlotFree(ctx, store.IsSlotFreeParams{...})
		if err != nil {
			return fmt.Errorf("check slot: %w", err)
		}
		if !free {
			return ErrSlotTaken                          // -> 409
		}
		row, err := q.InsertAppointment(ctx, ...)
		if err != nil {
			return fmt.Errorf("insert appointment: %w", err)
		}
		out = fromRow(row)
		return audit.Write(ctx, q, sess, audit.AppointmentCreate, row.ID)
	})
	return out, err
}
```
Аудит в той же транзакции. Бизнес-правила — в service, не в SQL-триггерах и не в handler.

## Ошибки
Доменные: `var ErrSlotTaken = errs.Conflict("slot_taken", "Время уже занято")`. `httpx.ErrorHandler` превращает в `application/problem+json` с `type`, `title`, `status`, `request_id`. Неизвестная ошибка → 500, детали только в лог (без ПДн). Объект чужой клиники → 404.

## Списки
Cursor-пагинация (`cursor`, `limit ≤ 100`, по умолчанию 50), keyset `(created_at, id)`, никаких `OFFSET` на больших таблицах. Поиск по ПДн — blind index / нормализованный телефон-хеш, не `ILIKE` по шифротексту.

## Идемпотентность
Оплаты, отправка сообщений, вебхуки: заголовок `Idempotency-Key`, таблица `idempotency_keys (clinic_id, key, response_hash, created_at)`.

## Обязательные тесты (`handler_test.go`, testcontainers Postgres, `-race`)
Table-driven: happy path · невалидный ввод → 400 · без сессии → 401 · роль без права → 403 · объект другой клиники → 404 · аудит-запись есть · в логах нет ПДн (захват slog-хендлера в тесте).
