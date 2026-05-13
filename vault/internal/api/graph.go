package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 9.3 — Knowledge Graph endpoint.
//
//   GET /admin/graph?project=korva&limit=200
//
// Returns observations as nodes and their relations as edges. Designed to
// drive an interactive force-directed visualization in the dashboard.
//
// Limits are enforced server-side so a curious operator can't crash their
// browser on a project with 50 000 observations:
//   - default limit 150, max 500
//   - project filter is required (graphs across all projects are noise)

// GraphNode is one observation rendered as a node.
type GraphNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Project  string `json:"project"`
	TopicKey string `json:"topic_key,omitempty"`
}

// GraphEdge is one relation between observations.
type GraphEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence"`
}

// GraphResponse is the wire shape of /admin/graph.
type GraphResponse struct {
	Project   string      `json:"project"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	Truncated bool        `json:"truncated"`
}

func adminGraph(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		if project == "" {
			writeError(w, http.StatusBadRequest, "project query param is required")
			return
		}
		limit := 150
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				if n > 500 {
					n = 500
				}
				limit = n
			}
		}

		// Pull observations.
		obs, err := s.Search("", store.SearchFilters{Project: project, Limit: limit + 1})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		truncated := len(obs) > limit
		if truncated {
			obs = obs[:limit]
		}

		// Build node list + id-set for edge filtering.
		nodes := make([]GraphNode, 0, len(obs))
		ids := make(map[string]struct{}, len(obs))
		for _, o := range obs {
			nodes = append(nodes, GraphNode{
				ID:       o.ID,
				Label:    o.Title,
				Type:     string(o.Type),
				Project:  o.Project,
				TopicKey: o.TopicKey,
			})
			ids[o.ID] = struct{}{}
		}

		// Pull project relations and keep only those whose endpoints are in
		// the node set (relations to obs outside the cap would render as
		// dangling edges).
		rels, err := s.ListRelationsByProject(project, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		edges := make([]GraphEdge, 0, len(rels))
		for _, r := range rels {
			if _, ok := ids[r.SourceID]; !ok {
				continue
			}
			if _, ok := ids[r.TargetID]; !ok {
				continue
			}
			edges = append(edges, GraphEdge{
				Source:     r.SourceID,
				Target:     r.TargetID,
				Relation:   string(r.Relation),
				Confidence: r.Confidence,
			})
		}

		writeJSON(w, http.StatusOK, GraphResponse{
			Project:   project,
			Nodes:     nodes,
			Edges:     edges,
			Truncated: truncated,
		})
	}
}
