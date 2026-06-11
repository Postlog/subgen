# Changelog

Изменения subgen — одна запись на PR, обратно-хронологически. Нетривиальные изменения
ссылаются на ADR в [`docs/decisions/`](docs/decisions/). Правило и формат —
в [`AGENTS.md`](AGENTS.md) (раздел «Документирование изменений»). Версий/тегов нет:
сервис не релизится, деплой непрерывный.

## 2026-06-11 — Валидация запросов — в сервисном слое, не в OpenAPI (#19)

Из `openapi/*.yaml` убраны все value-constraints (`minLength`/`minItems`/`minimum`) —
ogen больше не генерит серверные валидаторы значений (общий невнятный 400). Валидация —
в сервисном слое sentinel-ошибками (`entity.ErrValidation*`), хендлеры тонкие. Заведён
`internal/service/nodes` (валидация узла + save/delete); node-валидация и `web.ValidateNode`
переехали туда. Ссылочную целостность инбаунда **не предчекаем** — её держит FK БД
(RESTRICT), репозиторий переводит нарушение в `entity.ErrInboundReferenced`. Пустой URL
provider-check — не спец-кейс (как и кривой URL → `RulesetCheckUnreachable`). Суррогатные id
(PK) **не** валидируем (несуществующий id → not-found). Тексты сообщений хендлеров сделаны
публичными и импортируются в apitest (без дублирования). В тестах `gomock.Any()` оставлен
только для контекста — остальные аргументы проверяются точно (матчеры для random uuid/subId).
`required`/`type`/`format` оставлены (форма контракта). См.
[ADR-0003](docs/decisions/0003-validation-in-code.md).

## 2026-06-11 — Упорядоченный раннер миграций (#18)

Ручные `*.manual.sql` заменены раннером `migrations.Apply` (`repository.Open` зовёт его
вместо `ExecContext(Schema)`): `0001-init.sql` — иммутабельный базлайн, далее `NNNN-*.sql`
по имени, факт применения — в `schema_migrations`, каждая миграция в транзакции,
fail-fast + лог. Connection-PRAGMA (вкл. `journal_mode=WAL`) переехали в DSN. Раздел про
миграции в `AGENTS.md` переписан. См. [ADR-0002](docs/decisions/0002-ordered-migration-runner.md).

## 2026-06-11 — Конвенция документирования: CHANGELOG + ADR (#16)

Заведены `CHANGELOG.md` (этот файл) и каталог ADR `docs/decisions/`; правило записано в
`AGENTS.md`. Выбран формат «одна запись на PR, без версий».
См. [ADR-0001](docs/decisions/0001-adopt-changelog-and-adr.md).
