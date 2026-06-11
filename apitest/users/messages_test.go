//go:build apitest

package users_test

// User-facing messages the API returns, mirrored from the presentation layer
// (internal/handlers/web/messages.go + each user handler's success constant). Black-box
// tests assert the exact text subgen produces; re-stating it here is the seam between
// the (unexported) production strings and these tests.
const (
	// Validation (web.UserMessage → entity sentinels). Since ADR-0003 (validation in code,
	// no schema value-constraints), an empty name reaches the handler → validateName →
	// msgInvalidUserName. An absent inbound-id list is still rejected as a generic 400 by
	// the kept `required` (a null array), so the no-connection cases assert api.MsgBadRequest.
	msgInvalidUserName = "Имя клиента: разрешены символы a-z, 0-9, _ и -. От 1 до 32 символов"
	msgNameTaken       = "Имя занято"
	msgInboundNotFound = "Указанный инбаунд не найден"

	// Success messages (per-handler constants).
	msgCreated   = "Создан пользователь"
	msgUpdated   = "Подключения обновлены"
	msgDeleted   = "Пользователь удалён"
	msgRecreated = "Клиенты пересозданы"
)
