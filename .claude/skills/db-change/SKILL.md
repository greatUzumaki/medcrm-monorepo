---
name: db-change
description: Use when changing the Postgres schema or queries for the Go backend — goose migrations, sqlc queries, RLS policies, indexes, encrypted PHI columns with blind indexes, pgvector. Ensures tenant isolation and zero-downtime migrations.
---

# Изменение БД (goose + sqlc + Postgres)

## Миграция — `apps/api/db/migrations/<timestamp>_<name>.sql`
Создать: `make -C apps/api migration name=add_waitlist`.
```sql
-- +goose Up
CREATE TABLE waitlist (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),          -- PG18: uuidv7(); на PG17 — gen_random_uuid() или из Go
  clinic_id   uuid NOT NULL REFERENCES clinics(id),
  patient_id  uuid NOT NULL REFERENCES patients(id),
  note_enc    bytea,                                      -- шифротекст, см. ниже
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE INDEX ON waitlist (clinic_id, created_at, id);
CREATE INDEX ON waitlist (patient_id);

ALTER TABLE waitlist ENABLE ROW LEVEL SECURITY;
ALTER TABLE waitlist FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON waitlist
  USING (clinic_id = current_setting('app.clinic_id')::uuid)
  WITH CHECK (clinic_id = current_setting('app.clinic_id')::uuid);

-- +goose Down
DROP TABLE waitlist;
```
Правила:
- `clinic_id` + RLS (`ENABLE` и `FORCE`) на **каждой** таблице с данными клиники. Приложение работает под ролью `app` без `BYPASSRLS` и не владельцем таблиц; миграции — под ролью `migrator`.
- `clinic_id` первым в составных индексах. FK всегда с индексом.
- Деньги `bigint` (копейки), время `timestamptz`, статусы — `text` + `CHECK` или enum.
- Медданные — `deleted_at` (мягкое удаление), физическое — только процедурой уничтожения (скилл `pdn-152fz`).

## Tenant-контекст (уже в `internal/platform/db`)
`WithTenant` открывает tx и делает `SELECT set_config('app.clinic_id', $1, true)` (параметризованно, `SET LOCAL` не принимает параметры). Без этого RLS вернёт 0 строк / ошибку — это и есть защита.

## Чувствительные поля
- Колонка `<field>_enc bytea` — `crypto.Seal(keyVersion, plaintext, aad=clinic_id||table||id)`. AAD не даёт переставить шифротекст между записями.
- Поиск: `<field>_bidx bytea` = HMAC-SHA256(нормализованное значение, отдельный ключ). Индекс только по `_bidx`.
- Открытый текст ПДн не индексировать, не класть в `jsonb` метаданных, не дублировать в аудит.

## Запросы — `apps/api/db/queries/<domain>.sql`
```sql
-- name: ListWaitlist :many
SELECT id, patient_id, created_at FROM waitlist
WHERE deleted_at IS NULL AND (created_at, id) > (sqlc.arg(after_ts), sqlc.arg(after_id))
ORDER BY created_at, id LIMIT sqlc.arg(lim);
```
- `SELECT` явных колонок, не `*`. `clinic_id` в WHERE не нужен (RLS), но и не вредит для планировщика.
- После изменений: `make -C apps/api gen`, затем `EXPLAIN ANALYZE` для горячих запросов.

## pgvector
`embedding vector(N)` + `CREATE INDEX ... USING hnsw (embedding vector_cosine_ops)`. Чанки несут `clinic_id`, `patient_id`, `source_id` — RLS действует.

## Zero-downtime
- NOT NULL колонка: add nullable → backfill батчами (River) → `SET NOT NULL` через `CHECK ... NOT VALID` + `VALIDATE`.
- Индекс на большой таблице: `CREATE INDEX CONCURRENTLY` в отдельной миграции с `-- +goose NO TRANSACTION`.
- rename/drop: expand → переключить код → contract, в разных релизах.
- **Применённые миграции не редактировать** (хук блокирует) — только новая.

## Seed
Только синтетика (`cmd/seed`, gofakeit с ru-локалью). Реальные выгрузки в репо — запрещены.
