package web

import (
	"errors"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// User-facing messages (Russian) live here at the presentation layer — lower
// layers return technical/sentinel errors only.
const (
	msgInvalidUserName   = "Имя клиента: разрешены символы a-z, 0-9, _ и -. От 1 до 32 символов"
	msgNameTaken         = "Имя занято"
	msgNodeNameTaken     = "Узел с таким именем уже существует"
	msgInboundDuplicate  = "Имя или порт инбаунда уже заняты на этом узле"
	msgProviderNameTaken = "Rule-provider с таким именем уже существует"
	msgNoConnection      = "Выберите хотя бы одно подключение"
	msgNodeNotFound      = "Узел не найден"
	msgInboundNotFound   = "Указанный инбаунд не найден"
	msgUserConfigExists  = "У пользователя уже есть кастомный конфиг"
	msgUserConfigMissing = "У пользователя нет кастомного конфига"

	// mihomo-config validation (decode/validate live in internal/mihomo).
	msgGroupNameEmpty   = "Укажите название proxy-группы"
	msgGroupNameTaken   = "Proxy-группа с таким названием уже существует"
	msgGroupUnknownType = "Неизвестный тип proxy-группы"
	msgGroupNoMembers   = "Пустая proxy-группа"
	msgGroupCycle       = "Proxy-группы образуют циклическую ссылку"
	msgBadRef           = "Некорректная цель правила/элемента группы"
	msgGroupRefRange    = "Ссылка на несуществующую группу"
	msgUnknownRuleType  = "Неизвестный тип правила"
	msgMatchNotLast     = "Правило MATCH должно быть последним"
	msgRuleValueReq     = "У правила не указано значение"
	msgBaseYAMLInvalid  = "YAML невалиден — проверьте синтаксис"
	msgGeneratedKey     = "Уберите из YAML генерируемые разделы"

	// mihomo rule-providers.
	msgProviderNameEmpty   = "Укажите название rule-provider"
	msgProviderBadBehavior = "Неизвестный behavior у rule-provider"
	msgProviderBadFormat   = "Неизвестный format у rule-provider"
	msgProviderURLEmpty    = "Укажите URL у rule-provider"
	msgRuleSetUnknownProv  = "RULE-SET ссылается на несуществующего rule-provider"
)

// UserMessage maps a known domain sentinel to friendly Russian text. For any other
// error it returns the technical message as-is (admin-facing operational detail,
// e.g. per-panel provisioning failures).
func UserMessage(err error) string {
	var pce entity.PanelClientExistsError
	if errors.As(err, &pce) {
		return "на панели «" + pce.Node + "» уже есть клиент с таким именем — удалите его там вручную или выберите другое имя"
	}

	switch {
	case errors.Is(err, entity.ErrInvalidUserName):
		return msgInvalidUserName
	case errors.Is(err, entity.ErrNameTaken):
		return msgNameTaken
	case errors.Is(err, entity.ErrNodeNameTaken):
		return msgNodeNameTaken
	case errors.Is(err, entity.ErrInboundDuplicate):
		return msgInboundDuplicate
	case errors.Is(err, entity.ErrRuleProviderNameTaken):
		return msgProviderNameTaken
	case errors.Is(err, entity.ErrNoConnectionSelected):
		return msgNoConnection
	case errors.Is(err, entity.ErrNodeNotFound):
		return msgNodeNotFound
	case errors.Is(err, entity.ErrInboundNotFound):
		return msgInboundNotFound
	case errors.Is(err, entity.ErrUserConfigExists):
		return msgUserConfigExists
	case errors.Is(err, entity.ErrUserConfigNotFound):
		return msgUserConfigMissing
	case errors.Is(err, mihomo.ErrGroupNameEmpty):
		return msgGroupNameEmpty
	case errors.Is(err, mihomo.ErrGroupNameTaken):
		return msgGroupNameTaken
	case errors.Is(err, mihomo.ErrGroupUnknownType):
		return msgGroupUnknownType
	case errors.Is(err, mihomo.ErrGroupNoMembers):
		return msgGroupNoMembers
	case errors.Is(err, mihomo.ErrGroupCycle):
		return msgGroupCycle
	case errors.Is(err, mihomo.ErrBadRef):
		return msgBadRef
	case errors.Is(err, mihomo.ErrGroupRefRange):
		return msgGroupRefRange
	case errors.Is(err, mihomo.ErrUnknownRuleType):
		return msgUnknownRuleType
	case errors.Is(err, mihomo.ErrMatchNotLast):
		return msgMatchNotLast
	case errors.Is(err, mihomo.ErrRuleValueRequired):
		return msgRuleValueReq
	case errors.Is(err, mihomo.ErrBaseYAMLInvalid):
		return msgBaseYAMLInvalid
	case errors.Is(err, mihomo.ErrGeneratedKeyPresent):
		return msgGeneratedKey
	case errors.Is(err, mihomo.ErrProviderNameEmpty):
		return msgProviderNameEmpty
	case errors.Is(err, mihomo.ErrProviderBadBehavior):
		return msgProviderBadBehavior
	case errors.Is(err, mihomo.ErrProviderBadFormat):
		return msgProviderBadFormat
	case errors.Is(err, mihomo.ErrProviderURLEmpty):
		return msgProviderURLEmpty
	case errors.Is(err, mihomo.ErrRuleSetUnknownProvider):
		return msgRuleSetUnknownProv
	default:
		return err.Error()
	}
}
