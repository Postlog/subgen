// Package provider_check implements the providerCheck operation
// (POST /admin/api/config/mihomo/provider/check) — a read-only probe of a rule-provider
// URL: is it reachable, is a file actually there, does it match the declared format.
package provider_check

import (
	"context"
	"fmt"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages. Exported so apitest can assert against them without duplicating
// the text. There is no empty-URL case: a blank URL is just one flavour of un-probeable
// URL (like a malformed one) and surfaces as RulesetCheckUnreachable — the handler only
// maps checker outcomes to text, it does not validate the URL.
const (
	MsgUnreachable  = "Не удалось подключиться к URL"
	MsgEmpty        = "Ответ пустой — по URL нет файла"
	MsgUnreachableP = "Не удалось подключиться: " // + technical detail
)

// Handler probes a rule-provider URL via the checker service.
type Handler struct {
	checker rulesetChecker
}

// New builds the handler.
func New(checker rulesetChecker) *Handler { return &Handler{checker: checker} }

// ProviderCheck implements oas.Handler: a reachable, right-format file is a 200 with a
// message; any other outcome is a 400. The handler does not validate the URL — a blank or
// malformed one is just an un-probeable URL the checker reports as unreachable.
func (h *Handler) ProviderCheck(ctx context.Context, req *oas.ProviderCheckReq) (oas.ProviderCheckRes, error) {
	res := h.checker.Check(ctx, req.URL, req.Format)

	ok, msg := describe(res, req.Format)
	if !ok {
		return &oas.ProviderCheckBadRequest{ErrMessage: msg}, nil
	}

	return &oas.MessageResponse{Message: msg}, nil
}

// describe maps the structured check outcome to a user-facing message.
func describe(res entity.RulesetCheckResult, format string) (bool, string) {
	switch res.Outcome {
	case entity.RulesetCheckOK:
		return true, fmt.Sprintf("Доступен:  формат «%s», %s", format, humanSize(res.Size))
	case entity.RulesetCheckHTTPError:
		return false, fmt.Sprintf("Сервер вернул HTTP %d — файла нет или нет доступа", res.Status)
	case entity.RulesetCheckEmpty:
		return false, MsgEmpty
	case entity.RulesetCheckFormatMismatch:
		return false, fmt.Sprintf("Скачалось (%s), но содержимое не похоже на формат «%s»", humanSize(res.Size), format)
	default: // RulesetCheckUnreachable
		if res.Detail != "" {
			return false, MsgUnreachableP + res.Detail
		}

		return false, MsgUnreachable
	}
}

func humanSize(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}
