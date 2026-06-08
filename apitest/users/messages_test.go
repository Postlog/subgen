//go:build apitest

package users_test

// User-facing messages the API returns, mirrored from the presentation layer
// (internal/handlers/web/messages.go + each user handler's success constant). Black-box
// tests assert the exact text subgen produces; re-stating it here is the seam between
// the (unexported) production strings and these tests.
const (
	// Validation (web.UserMessage → entity sentinels). Note: the "no connection" and
	// "empty name" handler messages are now unreachable over the API — the request schema
	// (minItems:1 / minLength:1) rejects those before the handler runs — so the relevant
	// tests assert the generic api.MsgBadRequest for them instead.
	msgInvalidUserName = "Имя клиента: разрешены символы a-z, 0-9, _ и -. От 1 до 32 символов"
	msgNameTaken       = "Имя занято"
	msgInboundNotFound = "Указанный инбаунд не найден"

	// Success messages (per-handler constants).
	msgCreated   = "Создан пользователь"
	msgUpdated   = "Подключения обновлены"
	msgDeleted   = "Пользователь удалён"
	msgRecreated = "Клиенты пересозданы"
)
