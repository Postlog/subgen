# AGENTS.md — стиль и правила кода subgen

Документ описывает структуру и стилистические правила сервиса `subgen`. Сервис
**приведён** к этой раскладке (entity / clients / repository / service / handlers,
`contract.go`+mockgen, table-driven тесты). Любой новый код пиши по этим правилам;
трогаешь старый — подтягивай его к ним в том же изменении.

Композиционный корень — `cmd/service/main.go`: загружает config, открывает
репозитории, конструирует клиентов/сервисы и per-action хендлеры с их зависимостями
и собирает `gorilla/mux`-роутер. **Оракула `App` нет** — данные идут снизу вверх
(`repository`/`clients` → `service` → `handler`), кеш узкий (внутри конкретного
сервиса), без глобальных atomic-снапшотов. HTTP-хендлеры — по пакету на действие в
`internal/handlers/<action>`, общий HTTP-инструментарий — `internal/handlers/web`.

Дизайн и эксплуатация subgen — в [`docs/subgen.md`](docs/subgen.md) и
[`README.md`](README.md). Инфраструктурные факты, от которых зависит **код** (контракт
3x-ui API, секреты, dev-гочи) — в разделе «Инфраструктура, 3x-ui API и секреты» ниже.
Остальной документ — про **код** Go-сервиса.

> subgen — самостоятельный продукт (подписочный сервер mihomo/Clash.Meta), выделенный
> из монорепо парка [`Postlog/vpn-toolchain`](https://github.com/Postlog/vpn-toolchain);
> топология парка, узлы и наблюдаемость живут там.

Эталоны структуры (смотреть как образец):
`go.avito.ru/av/service-listing-admin`, `go.avito.ru/av/service-mnz-sf`.

## Инфраструктура, 3x-ui API и секреты

Это не «про стиль», но код subgen завязан на эти факты — держи их в голове, правя
клиента/провижининг/деплой. Полный дизайн — в [`docs/subgen.md`](docs/subgen.md).

### Секреты — никогда в git

Пароли панелей, `SUBGEN_SECRET` (HMAC), админ-креды, client UUID/ключи и отрендеренные
per-client конфиги **не коммитятся**. Bootstrap-секреты — в `.env` (gitignored, рядом с
`.env.example`); `db/` (SQLite-стор) тоже gitignored. В git уходят только примеры.

### 3x-ui API (обе панели парка — 3.2.6) — `internal/clients/xui`

- **Auth = Bearer API-токен.** `Authorization: Bearer <token>` (выдать: `x-ui setting
  -getApiToken` или Settings → API tokens в панели). Токен-вызовы обходят CSRF, логин/кука
  не нужны — это машинный путь subgen. Браузерный логин на 3.x требует CSRF-токен
  (`<meta name="csrf-token">` → `X-CSRF-Token`) — для сервиса не используем.
- **Управление клиентами переехало** в `/panel/api/clients/*` (`add`, `update/:email`,
  `del/:email`). Старый `/panel/api/inbounds/addClient` на 3.2.x отдаёт **404**. Тело add:
  `{"client":{…,"tgId":0},"inboundIds":[…]}` — `tgId` это **int** (0), не строка, иначе 400.
- **Модель идентичности клиента (важно).** Клиент 3x-ui = один `uuid` (VLESS-credential);
  `email` и `subId` — метки на этом uuid. **Один клиент может висеть на многих инбаундах** —
  передай несколько id в `inboundIds`; uuid/email/subId остаются одни на все. Грабли,
  набитые трудом: (a) `del/:email` резолвит email→один uuid и снимает его со всех инбаундов,
  но если **один email на двух инбаундах с разными uuid** (провижинились отдельными `add`,
  каждый минтил новый uuid) — падает `Client Not Found In Inbound For ID: <uuid>` и **не
  удаляет ничего**; (b) `subId` привязан к одному email — переиспользование subId на двух
  email → `subId already in use`. Поэтому пользователь провижинится как **один клиент на
  панель** (один uuid, email = ник, общий subId), привязанный ко всем своим инбаундам
  **одним** `add`; правка = ре-байнд того же клиента (сохраняем uuid). Per-inbound
  delete-роута в этой сборке **нет** (`/panel/api/inbounds/:id/delClient/:id` → 404); только
  `del/:email`.
- `settings`/`streamSettings` приходят как **JSON-объекты** (на 3.x; до 3.x были
  JSON-строкой внутри JSON) — клиент разбирает оба через `json.RawMessage`.
- Теги новых инбаундов — `in-<port>-<net>` (напр. `in-8443-tcp`).
- DNS RU1 починен на уровне хоста — кастомный resolver-воркэраунд в xui-клиенте убран,
  используется системный резолвер.

### Dev-гочи (когда дёргаешь панели/узлы с Mac оператора)

- **На узлах нет `sqlite3`.** Чтобы заглянуть в `/etc/x-ui/x-ui.db` — копируй локально или
  ходи по HTTP API 3x-ui.
- **У Mac оператора есть HTTP(S)-прокси** (`HTTPS_PROXY`), рубящий нестандартные порты (напр.
  61001). Для curl/go против панелей префикси
  `env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy`. (subgen на узле не задет —
  его HTTP-транспорт прокси не ставит.)
- Прод-узел живой и общий с реальными юзерами: **сначала read-only разведка**, и
  **подтверждай наружу-видимые/необратимые действия** (деплой, рестарт Xray, правки клиентов).

### Деплой

Docker (не systemd). Прод-деплой — ручной GitHub Actions workflow
(`.github/workflows/deploy.yml`, `workflow_dispatch`): образ собирается на runner'е (узел
RAM-голодный — Go/реестр на узле не нужны), шлётся по SSH (`docker save | ssh | docker load`),
`.env` рендерится из секретов Environment `production`, `docker compose up -d`. Подробности —
`README.md` / `docs/subgen.md`. Legacy systemd-деплой удалён (был `systemd/`). SIGHUP-релоада нет —
конфиг течёт снизу вверх из стора на каждый запрос. **Миграции БД — только руками** (схема
`CREATE … IF NOT EXISTS`).

## Целевые правила (инверсия слоёв)

Ниже — жёсткие правила второго прохода. Они уточняют разделы ниже; при конфликте
приоритет у этих формулировок. Любой новый код им следует; трогаешь старый — подтягивай.

### Клиенты внешних API — тонкие адаптеры

- Клиент = тонкая прослойка-адаптер к `entity`. **Ноль бизнес-логики.** Никакого
  внутреннего состояния, кроме нужного для подключения к ресурсу (`http.Client`,
  таймауты). Имя узла, публичный хост, конкретный токен — **не** состояние клиента.
- Никаких «толстых» методов: метод не дергает общий список (`ListInbounds`) и не
  вычисляет из него что-то под конкретную задачу (поиск id по порту, uuid по email) —
  это бизнес-логика, её место в сервисе. Клиент отдаёт сырые доменные данные.
- Если разные узлы задают разные креды подключения — **креды передаются параметром
  метода**, не в конструктор. Один клиент на процесс, цель вызова — аргумент.
- **Один метод — один `.go`-файл** (`list_inbounds.go`, `add_client.go`, `del_client.go`).
- На каждый метод (= файл) — отдельный unit-тест; HTTP мокается (`httptest.Server` /
  `http.RoundTripper`-мок), без живой сети.

### Config — только статика, не течёт по слоям

- `config` = пакет с `Load() (Config, error)`: читает окружение/`.env` (через теги,
  библиотекой) и валидирует. Точка.
- У `Config` — 0 методов (максимум простые геттеры). Она «особая»: в `entity` её можно
  не класть.
- `Config` **не прокидывается** по слоям. В конструкторы сервисов/хендлеров идут только
  конкретные примитивные поля (никакого `New(cfg *config.Config)`). Взаимодействие со
  структурой `Config` — максимум в `main`.
- **Никакого конфига из БД** (`FromStore`-подобного кода быть не должно — это нарушение
  data-flow: операционные данные идут снизу вверх из репозиториев).
- **Никакого seed.** Нет данных в хранилище — значит нет данных; дефолтных
  конфигов/правил/провайдеров в коде нет.

### entity — самодокументируемые типы

- Признаки — типами и константами, не «магическими» строками.
  Эталон — `mihomo.PolicyRef`/`PolicyKind` и `RuleType`/
  `ProxyGroupType` (в пакете `internal/mihomo`): цель роутинга резолвится по
  типизированному `Kind`, **никогда** по подстроке вроде
  `strings.HasPrefix(name, "<...>")`.
- Сильная типизация: client id → `github.com/google/uuid.UUID`. Где встроенный тип
  неудобен — обосновать в комментарии (напр. `Node.PanelBaseURL` остаётся `string`:
  гоняется через SQLite-текст/HTML-формы и только конкатенируется с path — `url.URL`
  тут ничего не даёт).
- **Ссылки на сущности — по числовому id, не по имени/порту.** Имя (узла и т.п.)
  мутабельно и годится только для отображения и label-имён (`<node>-<inbound>`). Всё, что
  пересекает границу (API ↔ фронт) или используется как ключ поиска/диффа — это id
  (`node_inbounds.id` для выбора подключения, `node.id` для группировки). Исключение —
  сопоставление `inbound_port` с **внешним** 3x-ui инбаундом: порт — идентификатор на
  стороне 3x-ui, не наш id.
- **Сравнение содержимого строк запрещено** (`strings.Contains(name, "...")` и т.п.).
  Нужен признак — заводи булев флаг или типизированную константу. Исключения допустимы,
  но только с обоснованием в комментарии.

### Ошибки — sentinel, без интерполяции

- Доменные ошибки — sentinel-константы в `entity`:
  `var ErrNameTaken = errors.New("name already taken")`. Возвращай их
  (`return entity.ErrNameTaken`), **не** подставляй имя/значение в текст ошибки — оно уже
  есть в контексте вызывающего.
- Нижние слои — обёрнутые технические ошибки (`fmt.Errorf("dep.Method: %w", err)`), без
  человеко-читаемого текста.
- **Ни одного `fmt.Errorf` с русским/человеко-текстом в `repository`/`service`/`clients`.**
  Понятные сообщения (в т.ч. русские) — только константы на слое хендлера.
- **Уникальность — из ответа БД, без пред-чек SELECT-ов.** Дубль ловится по типизированному
  коду констрейнта (`internal/repository/dberr.IsUniqueViolation`: `errors.As` →
  `*sqlite.Error.Code()` ∈ {`SQLITE_CONSTRAINT_UNIQUE` 2067, `SQLITE_CONSTRAINT_PRIMARYKEY`
  1555}; modernc включает extended-коды на каждом соединении — **никакого сравнения строк**),
  и репозиторий переводит его в доменный sentinel (`entity.ErrNameTaken` / `ErrNodeNameTaken` /
  `ErrInboundDuplicate` / `ErrRuleProviderNameTaken`). `users.NameTaken`-подобных пред-проверок
  быть не должно. PK даёт 1555, обычный UNIQUE — 2067; детектор матчит оба.

### Handlers — роутер, типизированные зависимости, структурный лог

- Роутинг — `gorilla/mux`: метод и path-параметры (`/sub/{kind}/{token}`) задаёт роутер
  (`.Methods("GET")`, `mux.Vars`), без ручного разбора `r.URL`/`r.Method`.
- Зависимость хендлера — конкретный интерфейс на нужные данные.
  **Анти-паттерн `cfgReader{ Cfg() *config.Config }` запрещён** — нужно конкретное поле,
  прокидывай конкретное поле.
- `slog` на уровне хендлера: сообщение в форме `"handler <name>: <event>"`; переменные —
  **только полями** лога, не в тексте сообщения
  (`slog.Warn("handler node_delete: delete failed", "id", id, "err", err)`). Нижние слои
  не логируют.
- Бэк — **чистые JSON-ручки** (`/admin/api/*`) + отдача статики; серверных шаблонов
  нет. Фронт — минимальный SPA на Vue 3 (global build, без сборки) в
  `internal/handlers/web/static/` (`index.html` + `app.js` + `app.css`), данные
  тянутся фетчем. **Отдача статики (`render.go`):** по умолчанию из **embed**-копии
  (`//go:embed static`, самодостаточный прод-образ), либо **живьём с диска**, если задан
  `SUBGEN_STATIC_DIR` (путь к каталогу относительно cwd) — тогда правки CSS/JS видны по
  reload без пересборки Go (локальная разработка; `assetFS()` выбирает источник).
  **Либы:** локально-вшитые (`vue.global.prod.js`, `Sortable.min.js`, `js-yaml.min.js`)
  + **с CDN** — Monaco (`monaco-editor@0.52.2`, AMD-loader). Внимание к порядку скриптов: UMD-либы (`js-yaml`) грузятся **до**
  Monaco-loader'а, иначе их UMD увидит `define.amd` и зарегистрируется модулем вместо
  выставления глобала. Поле base-YAML — компонент `yaml-editor` на **Monaco**
  (`loadMonaco()` лениво поднимает движок с CDN, `defineSubgenTheme` — тёмная тема под
  палитру; язык `yaml`, подсветка/Tab/текущая строка из коробки). **Валидация синтаксиса
  на лету — `jsyaml.load`** с debounce: ошибка кладётся маркером в Monaco
  (`setModelMarkers`, squiggle+hover) + строка статуса (line:col + reason).
  **Тема админки повторяет 3x-ui v3+** (React + Ant Design 6 dark) — это «скин» Ant-токенов
  поверх Bootstrap в `app.css`: bg `#1a1b1f` / card `#23252b` (radius 12) / header `#15161a`
  / modal `#2d2f37`, primary Ant-blue `#1668dc`, бордеры `rgba(255,255,255,.06–.12)`,
  **системные шрифты** (никаких webfont'ов). Любой новый UI держи в этих токенах.
  Read-ручки (`users_api`/`nodes_api`/`config_api`) отдают JSON;
  мутации (`user_*`/`node_*`/`config_save`) принимают форму и отвечают `{ok,msg|err}`.

### Композиция и слои

- **Нет оракула `App`.** Композиция зависимостей и сборка роутера — в `cmd/service`.
- Данные идут **снизу вверх**: `repository`/`clients` → `service` → `handler`. Нижний
  слой не знает о верхнем.
- **Кеш — узкий слой** вокруг конкретного репозитория/клиента (или внутри конкретного
  сервиса), не глобальный снапшот всего.
- `ruleset/mirror`, `fleet/build` и подобная логика — это сервисный слой
  (`internal/service/*`); генерация mihomo-YAML (резолв `PolicyRef`, сборка
  proxy-groups/rules) — `internal/mihomo/render`. Не «магия сбоку».

### Инфраструктура

- Рейтлимита нет (отказались). TLS-cert-релоадер — `internal/cert`. Деплой — Docker
  (не systemd).

## Структура каталогов

```
cmd/service/main.go            — сам сервис (entrypoint)
cmd/<tool>/main.go             — прочие бинари (CLI-утилиты, воркеры, cron)
internal/config/               — загрузка/валидация конфига (env + .env)
internal/clients/<dep>/        — клиенты к внешним сетевым зависимостям (xui, …)
internal/repository/<entity>/  — репозитории, разбиты по сущностям (users, nodes, …)
internal/service/<area>/       — сервисный слой (бизнес-логика)
internal/handlers/<do_some>/handler.go — HTTP-хендлеры (один пакет на действие)
internal/entity/               — общие kernel-типы домена (вход/выход слоёв), без I/O
internal/mihomo/               — поддомен mihomo-конфига (схема + decode/validate), без I/O и net/http
internal/mihomo/render/        — генерация mihomo-YAML из схемы + subscriber
migrations/init.sql            — миграции БД (CREATE … IF NOT EXISTS, применяется на старте)
migrations/*.manual.sql        — разовые ручные миграции (НЕ применяются автоматически)
```

Правила слоёв:
- **Поток зависимостей сверху вниз:** `handlers → service → repository | clients`.
  Нижний слой не знает о верхнем. Хендлер зависит от сервиса, сервис — от
  репозиториев/клиентов.
- **Один пакет на действие/сущность.** Хендлер действия — отдельный пакет
  `internal/handlers/do_some/`; репозиторий сущности — `internal/repository/users/`.
- **Кеш — это слой репозитория** для конкретной сущности (тот же контракт, что и
  у «настоящего» репозитория; кеш оборачивает/реализует его). Не отдельная
  «магия» сбоку.
- **`internal/entity`** — общие kernel-структуры домена (`Node`, `Inbound`, `User`,
  `Proxy`, `Subscriber`, `Panel*`, `Fleet`, `Connection`); без сетевых вызовов и без I/O.
- **`internal/mihomo`** — выделенный поддомен mihomo-конфига: модель схемы
  (`RoutingRule`/`ProxyGroup`/`PolicyRef`/`RuleProvider` + каталоги), её
  decode/validate (форма→типы, sentinel-ошибки) и `render/` (YAML). Это
  **осознанное исключение** из «единого плоского `entity`»: схема mihomo-конфига —
  отдельный связный домен, который притекает и в БД (`mihomo_`-таблицы), и в
  admin-схему, и в рендер. Жёсткие правила пакета: `mihomo` **не импортирует**
  `entity` и `net/http`; ссылки на инбаунд/группу — только по `int64`-id (поэтому
  цикла нет); человеко-текста ошибок в `mihomo` нет — только sentinel-константы
  (`ErrGroupCycle`, `ErrMatchNotLast`, …), маппинг в русский текст — на хендлере
  (`web.UserMessage`). `render/` — единственный, кому можно импортировать и
  `entity` (Proxy/Subscriber), и `mihomo`.

Раскладка уже приведена к этой цели: клиенты — `internal/clients/xui` (тонкий
адаптер, один метод — один файл); репозитории —
`internal/repository/{users,nodes,routing,configs}` (один метод — один файл; `configs`
— тип-агностичный якорь владения конфигом, см. ниже); сервисы —
`internal/service/{fleet,ruleset,provisioning}`
(`fleet` владеет TTL-кешем флита; `ruleset` — миррор провайдеров); хендлеры —
`internal/handlers/<action>`; TLS-релоадер — `internal/cert`; композиция и
`gorilla/mux`-роутер — в `cmd/service`. Пакетов `internal/server`/`App`,
`internal/{model,cache,ruleset}` и `config.FromStore`/seed больше нет.
**Бандла `repository.Store` тоже нет** — `repository.Open()` возвращает `*sql.DB`,
а per-entity репозитории (`users.New(db)`, …) собираются в композиционном корне.

**mihomo-конфиг (роутинг) — структурные данные, не строки.** Доменные типы в
`internal/mihomo`: `RoutingRule`, `ProxyGroup`(добавить элемент), и единый типизированный
**`PolicyRef`** {`PolicyKind` direct|reject|…|inbound|group, `InboundID`,
`GroupID`} — общий для цели правила и элемента группы; `RuleType`/`ProxyGroupType` —
типы-константы. **Никаких магических строк** — резолв в имя прокси
по типу/id делает per-subscriber резолвер `internal/mihomo/render/policy.go`
(проставляет `entity.Proxy.InboundID` в `fleet/build.go`); недоступные клиенту
инбаунды выкидываются, пустая группа → `DIRECT`. `repository/routing` пишет всё
атомарно (`SaveMihomoConfig(configID, …)`: группы+элементы+правила+провайдеры+base);
таблицы mihomo-конфига — с префиксом `mihomo_`, **скоупятся по `config_id`** (FK на
`subscription_configs` CASCADE; на `node_inbounds` — RESTRICT). Ссылки на
группу на границе HTTP/save — по **индексу массива** (реальные id наружу не выходят);
decode формы (`mihomo.DecodeConfig(raw json.RawMessage)` — хендлер достаёт сырой
body) и её валидация (вкл. ацикличность графа групп) живут в `internal/mihomo`
(`decode.go`/`validate.go`) и возвращают sentinel-ошибки.
Фронт — два визуальных конструктора (группы добавить правила) с общим `policy-picker` и
drag-n-drop (вендорный SortableJS) в `internal/handlers/web/static`. **Фронт ничего
не хардкодит** — включая **таксономию ссылок**: на что может указывать цель
правила / элемент группы, объявляет схема per-type. Каталоги живут в `mihomo`
(`RuleTypeCatalog`/`ProxyGroupTypeCatalog`/`BuiltinPolicyKinds`/`RuleProviderBehaviors`/
`RuleProviderFormats`/`GeneratedKeys` + `PolicyCategory`/`PolicyCategories` — единый
источник) и отдаются хендлером `config_schema` (`GET /admin/api/config/mihomo/schema`,
сортировка опций по имени — в хендлере) по секциям: `actions` (built-in
с лейблами), `ruleProvider`,
`proxyGroup.types[]` (опции + `items` — категории элементов), `rules.types[]` (опции +
`destinations` — категории цели), `generatedKeys`. **Категория ссылки** —
`actions`/`inbounds`/`groups`; `policy-picker` рисует только объявленные (категория
`inbounds` = все инбаунды флита, с лейблами).
Все mihomo-ручки — под `/admin/api/config/mihomo` (read / `…/schema` / `…/save` /
`…/customs` / `…/custom/create` / `…/custom/delete`).

**Базовый + per-user кастомные конфиги; движок — типизированный, не предполагается
единственным.** Владение конфигом — обобщённый якорь `subscription_configs(id,
user_id, kind, created_at)`: `user_id NULL` = базовый (на всех), иначе персональный
кастом; `kind` — `entity.ConfigKind` (движок: `mihomo` сейчас, далее xray/sing-box) —
**типизированная константа, не магическая строка**. На каждый `kind` свой базовый +
максимум один кастом на юзера (unique-индекс `COALESCE(user_id,0), kind`). Контент
движка (`mihomo_*`) висит на якоре через `config_id`. Слои:
- **`internal/repository/configs`** — тип-агностичный якорь (Base/User ConfigID,
  Ensure, List, Create, Delete), параметризован `entity.ConfigKind`, **ничего про
  mihomo не знает**. Клон контента базового в новый кастом — делегируется
  content-репозиторию через узкий `cloner`-контракт (`routing.CloneConfig`,
  в общей tx). Кастом = **снимок**: после клона независим, правки базового не долетают.
- **`internal/repository/routing`** — контент mihomo, все чтения/`SaveMihomoConfig`
  скоупятся `configID`; `AllRuleProviders` (по всем конфигам) — для миррора.
- **Подписка** — маршрут `/sub/{kind}/{token}`; `kind` валидируется по реестру
  рендереров `map[entity.ConfigKind]sub.EngineRenderer` (собирается в `cmd/service`,
  сейчас зарегистрирован mihomo). Хендлер: токен → юзер (`users.IDBySubID`) → его
  кастом для `kind`, иначе базовый → `EngineRenderer.Render(sub, configID)`. Чтения
  mihomo-контента спрятаны **внутрь** `sub.MihomoRenderer`, общий хендлер
  engine-generic. Добавить xray = новый `EngineRenderer` + `xray_*` таблицы +
  content-репозиторий + одна строка регистрации; якорь/роутер/admin-API не меняются.
- **Admin** — вкладка Config несёт область (`?user=<id>` на read; `userId` в save-теле);
  фронт — селектор «Пользователи: Все | <ник>» + «Добавить кастомный конфиг…» (клон),
  баннер кастома с «Удалить». URL движка в пути (`/config/mihomo/*`).

**Миграции БД — только вручную** (схема `CREATE … IF NOT EXISTS`, in-code миграций
не пишем; структурные изменения мигрируются руками). Разовые ручные миграции —
`migrations/*.manual.sql` (не применяются автоматически; напр.
`per_user_configs.manual.sql` — rebuild таблиц под `config_id`; **в SQLite перед
rebuild через rename ставь `PRAGMA legacy_alter_table=ON`**, иначе `RENAME` переписывает
FK в чужих таблицах и оставляет висячие ссылки).

## Зависимости: `contract.go` + mockgen

Зависимости сущности (хендлера, сервиса, …) объявляются **приватным интерфейсом
в том пакете, где используются**, в файле `contract.go`, с директивой генерации
мока. Интерфейс описывает ровно те методы, что нужны этому пакету (interface
segregation), и именуется по роли зависимости (`itemPlatformClient`,
`curlGenerator`, `usersRepository`).

```go
// contract.go
//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package $GOPACKAGE
package curl_generation_core

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

type itemPlatformClient interface {
	Get(ctx context.Context, in entity.ItemGetIn) (map[int64]entity.Item, error)
}

type curlGenerator interface {
	Generate(ctx context.Context, pl entity.CURLGeneratorPayload) ([]entity.CURL, error)
}
```

- `contract.go` + mockgen — для **сервисов, entity и клиентов** (клиент — по
  своему generated SDK). Моки лежат рядом (`contract_mocks.go`), генерируются
  `go generate ./...` (mockgen подключается как `go tool`).
- В тестах сервиса/entity используются **только локальные `Mock*`** из своего
  `contract.go`. **Не** тащить `clients.MockClient` / `clients.New(mock)` чужого
  пакета в тест сервиса — у сервиса свой приватный контракт на клиента.
- Конструктор принимает зависимости интерфейсами: `func New(c itemPlatformClient, …) *Service`.

## Клиенты внешних API: DTO → domain

Клиент в `internal/clients/<dep>/` держит **приватные wire-DTO** с json-тегами под
ответ зависимости «как есть» (со всеми квирками: вложенность, строко-в-строке,
чужие имена полей) и **маппит их в доменные типы** (`internal/entity`) на выходе.
Наружу клиент отдаёт только `entity.*`, а DTO/декодинг — приватная деталь пакета
(anti-corruption boundary). Пример: `clients/xui` анмаршалит в приватный
`inbound`/`streamSettings` (settings приходят JSON-строкой внутри JSON — `decode()`
их разворачивает) и конвертит в `entity.PanelInbound`. Так домен не знает про
формат панели, а сервис-слой тривиально мокается по `contract.go`.

## Публичный API

- Тестируются и вызываются снаружи **только экспортируемые методы**. Приватные
  хелперы/рендеры проверяются **через** публичный метод, который их использует.

## Врапинг ошибок и логирование

- **Врапинг — всегда.** При вызове зависимости оборачивай:
  `fmt.Errorf("<имя поля/зависимости>.<Метод>: %w", err)` — напр.
  `fmt.Errorf("economicEntitiesClient.GetByUserIDs: %w", err)`. Так стек читается
  по цепочке вызовов.
- Ошибки **приватных методов того же пакета** пробрасывай **без повторного**
  wrap (они уже обёрнуты внутри).
- **Тексты ошибок для пользователя формируются только на слое представления
  (handler).** Репозиторий/сервис возвращают «технические» обёрнутые ошибки и
  **не** генерируют человеко-читаемых текстов. Понятные сообщения — константы в
  пакете хендлеров (см. `error_messages.go` у эталонов:
  `MessageEntityNotFound`, `MessageInternalError`, …).
- **Логирование — `slog`, на уровне хендлера.** Хендлер логирует ошибку (с
  контекстом запроса) и отдаёт пользователю понятный текст. Нижние слои не
  логируют — только возвращают обёрнутую ошибку.

## Юнит-тесты

Строгая table-driven структура. Один `Test{Type}_{Method}` на **метод** (не
дробить на `Test*_Success` / `Test*_Error`).

```go
func TestClient_Method(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name string
		// вход
		buildMock func(mock *MockClient) // buildMocks(...) если зависимостей несколько
		result    SomeOut
		err       error
		wantErr   bool // только если err — не sentinel
	}{
		{name: "empty"},
		{name: "success.one_user", buildMock: func(m *MockClient) { /* EXPECT */ }, result: SomeOut{ /*…*/ }},
		{name: "error.downstream", buildMock: func(m *MockClient) { m.EXPECT().X(gomock.Any()).Return(nil, targetErr) }, err: targetErr},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mock := NewMockClient(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(mock)
			}

			c := New(mock)
			result, err := c.Method(context.Background(), /* in */)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, result)
		})
	}
}
```

| Правило | Суть |
|---|---|
| `contract.go` + mockgen | Интерфейсы зависимостей — в `contract.go` пакета; `//go:generate go tool mockgen -source=contract.go` для сервисов, entity и клиентов |
| Публичный API | Тестируем только экспортируемые методы; приватные хелперы — через публичный |
| Table-driven | Один `Test{Type}_{Method}` на метод; `tt []struct{ name, … }`; **не** дробить на `*_Success`/`*_Error` |
| Именование кейсов | `name` **без пробелов**: `success.one_user`, `error.invalid_user_id`, `empty_fields` |
| Параллельность | `t.Parallel()` на таблице **и** в `t.Run`. Исключение: если codegen зависимостей не потокобезопасен (data race в сгенерированном `New()`/декларации) — без параллельного конструирования |
| Ветки | Минимум: `empty` (если применимо), `success` с проверкой маппинга, `error` от downstream; доменные edge-кейсы — по смыслу |
| Моки | `mockgen` по `contract.go` своего пакета → `Mock*`; **не** тянуть чужой `clients.MockClient` в тест сервиса/entity |
| Проверки | `require.ErrorIs` / `ErrorAs` / `NoError` для ошибок; `assert.Equal` для результата; `wantErr` / `ErrorContains` — только когда нет конкретного sentinel |

## Интеграционные / API-тесты

Живут отдельно в `apitest/` (см. `apitest/README.md`): testify-suite против
настоящего 3x-ui в docker, под build-тегом `apitest`, переиспользуемый `Base` +
суит на область (`UserSuite`, далее `NodeSuite`/`ConfigSuite`), по файлу на
сценарий. Запуск «под ключ»: `make -C apitest test`.
