// Package node_save handles POST /admin/api/nodes/save (create or update a node).
package node_save

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

// inboundReq is one inbound row; id==0 marks a new inbound (existing ones send their
// node_inbounds.id back so edits keep it stable).
type inboundReq struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Port int    `json:"port"`
}

// saveReq is the node create/update payload (id==0 → create).
type saveReq struct {
	ID            int64        `json:"id"`
	Name          string       `json:"name"`
	VPNHost       string       `json:"vpnHost"`
	PanelBaseURL  string       `json:"panelBaseURL"`
	PanelBasePath string       `json:"panelBasePath"`
	Token         string       `json:"token"`
	Inbounds      []inboundReq `json:"inbounds"`
}

// Handler creates or updates a node from the node form.
type Handler struct {
	nodes   nodeRepo
	routing routingRepo
	cache   cacheInvalidator
}

// New builds the handler.
func New(nodes nodeRepo, routing routingRepo, cache cacheInvalidator) *Handler {
	return &Handler{nodes: nodes, routing: routing, cache: cache}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req saveReq
	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler node_save: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	n := entity.Node{
		Name:          strings.TrimSpace(req.Name),
		VPNHost:       strings.TrimSpace(req.VPNHost),
		PanelBaseURL:  strings.TrimSpace(req.PanelBaseURL),
		PanelBasePath: strings.TrimSpace(req.PanelBasePath),
		Token:         strings.TrimSpace(req.Token),
	}

	for _, in := range req.Inbounds {
		if in.Port == 0 {
			continue // blank inbound row
		}

		n.Inbounds = append(n.Inbounds, entity.Inbound{
			ID: in.ID, Name: strings.TrimSpace(in.Name), Port: in.Port,
		})
	}

	err := web.ValidateNode(&n)
	id := req.ID

	if err == nil && id > 0 {
		// Block removing an inbound that is still referenced (users / mihomo rules /
		// group members). Inbounds are keyed by node_inbounds.id; the submitted set
		// carries the id for each existing inbound, so "removed" = current ids absent
		// from the submission.
		kept := map[int64]bool{}

		for _, in := range n.Inbounds {
			if in.ID > 0 {
				kept[in.ID] = true
			}
		}

		var removed []int64

		if cur, e := h.nodes.Get(r.Context(), id); e == nil {
			for _, in := range cur.Inbounds {
				if !kept[in.ID] {
					removed = append(removed, in.ID)
				}
			}
		}

		if len(removed) > 0 {
			if msg, e := web.InboundsBlocking(r.Context(), h.nodes, h.routing, id, removed); e != nil {
				err = e
			} else if msg != "" {
				err = fmt.Errorf("%s", msg)
			}
		}
	}

	if err == nil {
		if id > 0 {
			err = h.nodes.Update(r.Context(), id, n, n.Token != "")
		} else {
			_, err = h.nodes.Create(r.Context(), n)
		}
	}

	if err == nil {
		h.cache.Invalidate()
	}

	if err != nil {
		slog.Warn("handler node_save: save failed", "name", n.Name, "err", err)
	}

	web.JSONResult(w, "Узел сохранён: "+n.Name, err)
}
