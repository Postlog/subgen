//go:build apitest

package nodes_test

// User-facing messages the node endpoints return, mirrored from the presentation layer.
// The field validators (web.ValidateNode) build interpolated Russian text, so tests
// assert a stable substring of those; the duplicate cases come from entity sentinels
// mapped in web.UserMessage and are matched exactly.
const (
	// Duplicate node name (entity sentinel → web.UserMessage), matched exactly.
	msgNodeNameTaken = "Узел с таким именем уже существует"

	// Field-validation substrings (web.ValidateNode interpolates the offending value).
	fragVPNHost      = "невалиден"            // "адрес %q невалиден — ожидается хост или IP..."
	fragNoInbound    = "хотя бы один инбаунд" // "укажите хотя бы один инбаунд"
	fragInboundName  = "имя инбаунда"         // "имя инбаунда %q: разрешены a-z, 0-9 и -"
	fragNodeNameChar = "имя узла"             // "имя узла: разрешены a-z, 0-9, -, пробел и флаги..."

	// In-payload duplicate inbound (web.ValidateNode catches these before the DB; the
	// per-node UNIQUE(node_id,name|port) sentinel msgInboundDuplicate is unreachable via
	// a single save because validation rejects the payload first).
	fragInboundDupName = "повторяющееся имя инбаунда"
	fragInboundDupPort = "повторяющийся порт инбаунда"

	// FK-block pre-check (web.InboundsBlocking), shared by delete + inbound-removal save.
	fragBlocked = "сначала отвяжите от инбаундов"
)
