//go:build apitest

package config_test

// User-facing messages the config endpoints return, mirrored from the presentation
// layer (internal/handlers/web/messages.go for save validation; the provider_check
// handler for the probe). Black-box tests assert the exact text subgen produces.
const (
	// mihomo-config save validation (web.UserMessage → mihomo sentinels).
	msgMatchNotLast    = "Правило MATCH должно быть последним"
	msgGroupNameTaken  = "Proxy-группа с таким названием уже существует"
	msgGroupCycle      = "Proxy-группы образуют циклическую ссылку"
	msgGroupRefRange   = "Ссылка на несуществующую группу"
	msgProviderNameReq = "Укажите название rule-provider"
	msgProviderNameDup = "Rule-provider с таким именем уже существует"
	msgRuleSetUnknown  = "RULE-SET ссылается на несуществующего rule-provider"
	msgGeneratedKey    = "Уберите из YAML генерируемые разделы"
	msgBaseYAMLInvalid = "YAML невалиден — проверьте синтаксис"
	msgRuleValueReq    = "У правила не указано значение"
	msgGroupNoMembers  = "Пустая proxy-группа"

	// Save success.
	msgSaved = "Конфиг сохранён"
)
