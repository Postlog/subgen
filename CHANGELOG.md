# Changelog

Изменения subgen — одна запись на PR, обратно-хронологически. Нетривиальные изменения
ссылаются на ADR в [`docs/decisions/`](docs/decisions/). Правило и формат —
в [`AGENTS.md`](AGENTS.md) (раздел «Документирование изменений»). Версий/тегов нет:
сервис не релизится, деплой непрерывный.

## 2026-06-11 — Строгие ссылки mihomo: RULE-SET → rule-provider по id (#17)

`RoutingRule` больше не хранит имя провайдера строкой в `value` — `RULE-SET` ссылается на
rule-provider по суррогатному id (`provider_id` FK); save-вход и domain/read разведены на
отдельные типы (draft с индексами vs domain с id), что убирает двойной смысл
`PolicyRef.GroupID`. Прод-БД мигрируется руками: `migrations/rule_provider_id.manual.sql`.
См. [ADR-0002](docs/decisions/0002-strict-mihomo-refs.md).

## 2026-06-11 — Конвенция документирования: CHANGELOG + ADR (#16)

Заведены `CHANGELOG.md` (этот файл) и каталог ADR `docs/decisions/`; правило записано в
`AGENTS.md`. Выбран формат «одна запись на PR, без версий».
См. [ADR-0001](docs/decisions/0001-adopt-changelog-and-adr.md).
