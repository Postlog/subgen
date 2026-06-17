# Changelog

Изменения subgen — одна запись на PR, обратно-хронологически. Нетривиальные изменения
ссылаются на ADR в [`docs/decisions/`](docs/decisions/). Правило и формат —
в [`AGENTS.md`](AGENTS.md) (раздел «Документирование изменений»). Версий/тегов нет:
сервис не релизится, деплой непрерывный.

## 2026-06-17 — Подписка: попап со ссылками из бэкенда (raw URL + clashmi-диплинк) (#116)

Колонка «Подписка» в списке пользователей теперь открывает попап со списком копируемых
ссылок, а не одну кнопку Mihomo: сейчас это сырой URL подписки Mihomo и диплинк
`clashmi://install-config?url=<enc>&name=<title>&overwrite=false`. Состав ссылок и их
тайтлы целиком приходят с бэка — новый сервис `internal/service/sublinks` владеет
каталогом, фронт ничего не хардкодит (добавить движок/приложение = одна строка каталога);
в попапе показываются только тайтл и кнопка «Копировать» (значение приватное). Форма `sub`
в `GET /admin/api/users` сменилась с `{id,url}` на `{links:[{title,value}]}`; `name`
clashmi-диплинка = profile title эффективного (кастомного, иначе базового) конфига
пользователя. См. [ADR-0008](docs/decisions/0008-subscription-link-catalog.md).

Заодно в этом PR: блок «Параметры подписки» поднят первым на вкладке «Конфиг Mihomo»;
убраны остатки githooks (таргет `make hooks` и локальный `core.hooksPath` — каталог
`.githooks/` был удалён ранее); по ревью — нейминг интерфейсов во всех `contract.go`
приведён к имени конкретной зависимости (repo → `<сущность>Repo`, service →
`<сущность>Service`, client → `<сущность>Client`) вместо ролевых имён
(`subLinker`/`configResolver`/`creator`/…); правило закреплено в `AGENTS.md`.

## 2026-06-16 — Логические правила mihomo (AND/OR/NOT) с рекурсивным tree-UI (#114)

Маршрутное правило теперь умеет логические операторы `AND`/`OR`/`NOT` с произвольно
вложенными под-правилами. Правило сделано рекурсивным (`RoutingRule.Children` — той же
структуры, `Target` опционален: у под-правила его нет), без отдельной сущности «условие»;
хранение — самоссылочная `mihomo_routing_rules` (`parent_id`), не JSON-блоб. Рендер выдаёт
вложенный синтаксис дословно (`AND,((NETWORK,UDP),(DST-PORT,443)),REJECT-DROP`). Добавлены
четыре матчера паритета с вики (`SRC-IP-ASN`, `SRC-IP-SUFFIX`, `PROCESS-PATH-WILDCARD`,
`PROCESS-NAME-WILDCARD`) и `sub-rules` в `GeneratedKeys` (оператор больше не может задать
секцию в base YAML). Под-правила не несут `no-resolve` (mihomo их не парсит). UI —
рекурсивный конструктор-дерево; SUB-RULE не реализован. Схема — миграция
`migrations/0004-mihomo-rule-children.notx.sql`. См.
[ADR-0006](docs/decisions/0006-recursive-routing-rules.md).

## 2026-06-11 — Строгие ссылки mihomo: RULE-SET → rule-provider по id (#17)

`RoutingRule` больше не хранит имя провайдера строкой в `value` — `RULE-SET` ссылается на
rule-provider по суррогатному id (`provider_id` FK); save-вход и domain/read разведены на
отдельные типы (draft с индексами vs domain с реальными id), что убирает двойной смысл
`PolicyRef.GroupID`. Опциональные поля (`value`/`interval`/`tolerance`/`lazy`/`noResolve`) —
указатели. Схема мигрируется раннером (`migrations/0003-strict-mihomo-refs.notx.sql` —
rebuild с FK off вне транзакции). См. [ADR-0005](docs/decisions/0005-strict-mihomo-refs.md).

## 2026-06-11 — Пользователь: опциональное описание для админки (#15)

У пользователя появилось опциональное свободнотекстовое описание (`*string`, nillable;
видно только в админ-UI): задаётся при создании/редактировании, показывается иконкой с
тултипом в таблице. Колонка `users.description` (nullable) добавляется миграцией
`migrations/0002-users-description.sql` через раннер. Сервисные входы вынесены в структуры
`entity.UserCreateParams` / `entity.UserEditParams` (убрал `entity.ConnectionSelection`).
См. [ADR-0004](docs/decisions/0004-optional-user-description.md).

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
