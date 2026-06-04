// Package provider_check handles POST /admin/api/config/mihomo/provider/check — a
// read-only probe of a rule-provider URL: is it reachable, is a file actually there,
// and does the content match the declared format (mrs / yaml / text)? It saves nothing.
package provider_check

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

// User-facing message fragments (Russian) — presentation layer.
const (
	msgNoURL        = "Укажите URL провайдера"
	msgUnreachable  = "Не удалось подключиться к URL"
	msgEmpty        = "Ответ пустой — по URL нет файла"
	msgUnreachableP = "Не удалось подключиться: " // + technical detail
)

// Handler probes a rule-provider URL via the checker service.
type Handler struct {
	checker providerChecker
}

// New builds the handler.
func New(checker providerChecker) *Handler { return &Handler{checker: checker} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Format string `json:"format"`
	}

	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler provider_check: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	if req.URL == "" {
		web.WriteJSON(w, false, msgNoURL)
		return
	}

	res := h.checker.Check(r.Context(), req.URL, req.Format)
	ok, msg := describe(res, req.Format)
	web.WriteJSON(w, ok, msg)
}

// describe maps the structured check outcome to a user-facing message.
func describe(res entity.RulesetCheckResult, format string) (bool, string) {
	switch res.Outcome {
	case entity.RulesetCheckOK:
		return true, fmt.Sprintf("Доступен:  формат «%s», %s", format, humanSize(res.Size))
	case entity.RulesetCheckHTTPError:
		return false, fmt.Sprintf("Сервер вернул HTTP %d — файла нет или нет доступа", res.Status)
	case entity.RulesetCheckEmpty:
		return false, msgEmpty
	case entity.RulesetCheckFormatMismatch:
		return false, fmt.Sprintf("Скачалось (%s), но содержимое не похоже на формат «%s»", humanSize(res.Size), format)
	default: // CheckUnreachable
		if res.Detail != "" {
			return false, msgUnreachableP + res.Detail
		}

		return false, msgUnreachable
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
