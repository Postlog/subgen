//go:build apitest

package nodes_test

// User-facing messages the node endpoints return, mirrored from the presentation layer.
// Node/inbound validation now lives in the nodes service (entity.ErrValidation* sentinels,
// ADR-0003), mapped to per-rule message constants in the node_save handler; tests assert a
// stable substring of those.
const (
	// Duplicate node name (entity.ErrNodeNameTaken → handler const), matched exactly.
	msgNodeNameTaken = "Узел с таким именем уже существует"

	// Field-validation substrings (handler constants per entity.ErrValidation* sentinel).
	fragVPNHost      = "невалиден"            // "Адрес VPN-хоста невалиден — ..."
	fragNoInbound    = "хотя бы один инбаунд" // "Укажите хотя бы один инбаунд"
	fragInboundName  = "Имя инбаунда"         // "Имя инбаунда: разрешены a-z, 0-9 и -"
	fragNodeNameChar = "Имя узла"             // "Имя узла: разрешены a-z, 0-9, -, пробел и флаги..."

	// In-payload duplicate inbound (validateNode catches these before the DB; the per-node
	// UNIQUE(node_id,name|port) sentinel msgInboundDuplicate is unreachable via a single
	// save because validation rejects the payload first).
	fragInboundDupName = "Повторяющееся имя инбаунда"
	fragInboundDupPort = "Повторяющийся порт инбаунда"

	// FK-block pre-check (nodes service → web.InboundsBlockedMessage), shared by delete +
	// inbound-removal save.
	fragBlocked = "сначала отвяжите от инбаундов"
)
