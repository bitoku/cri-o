package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/goccy/go-json"
	"k8s.io/apimachinery/pkg/fields"
	types "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/cri-o/cri-o/internal/lib/sandbox"
	"github.com/cri-o/cri-o/internal/log"
)

var secret = []byte("cri-o/foobar")

type pageToken struct {
	ID        string `json:"i"`
	CreatedAt int64  `json:"c"`
}

// Encode creates an opaque, signed token
func (t *pageToken) encode() string {
	if t == nil {
		return ""
	}
	payload, _ := json.Marshal(t)
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	sig := mac.Sum(nil)[:16] // truncate to 16 bytes
	return base64.RawURLEncoding.EncodeToString(append(sig, payload...))
}

func decodeToken(tok string) (*pageToken, error) {
	data, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(data) < 16 {
		return nil, errors.New("invalid token")
	}
	sig, payload := data[:16], data[16:]
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)[:16]) {
		return nil, errors.New("invalid token signature")
	}
	var t pageToken
	if err := json.Unmarshal(payload, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListPodSandbox returns a list of SandBoxes.
func (s *Server) ListPodSandbox(ctx context.Context, req *types.ListPodSandboxRequest) (*types.ListPodSandboxResponse, error) {
	ctx, span := log.StartSpan(ctx)
	defer span.End()

	podList := s.filterSandboxList(ctx, req.GetFilter(), s.ListSandboxes())
	respList := make([]*types.PodSandbox, 0, len(podList))

	var token, nextPageToken *pageToken
	var err error

	if req.GetPageToken() == "" {
		token = &pageToken{}
	} else {
		token, err = decodeToken(req.GetPageToken())
		if err != nil {
			return nil, fmt.Errorf("failed to decode token: %w", err)
		}
	}

	for _, sb := range podList {
		// Skip sandboxes that aren't created yet
		if !sb.Created() {
			continue
		}

		pod := sb.CRISandbox()

		if pod.CreatedAt < token.CreatedAt || pod.CreatedAt == token.CreatedAt && pod.Id < token.ID {
			continue
		}
		// Filter by other criteria such as state and labels.
		if filterSandbox(pod, req.GetFilter()) {
			if len(respList) == int(req.GetPageSize()) {
				lastPod := respList[len(respList)-1]
				nextPageToken = &pageToken{
					ID:        lastPod.Id,
					CreatedAt: lastPod.CreatedAt,
				}
				break
			}
			respList = append(respList, pod)
		}
	}

	return &types.ListPodSandboxResponse{
		Items:         respList,
		NextPageToken: nextPageToken.encode(),
	}, nil
}

// filterSandboxList applies a protobuf-defined filter to retrieve only intended pod sandboxes. Not matching
// the filter is not considered an error but will return an empty response.
//
// podList must be sorted by id.
func (s *Server) filterSandboxList(ctx context.Context, filter *types.PodSandboxFilter, podList []*sandbox.Sandbox) []*sandbox.Sandbox {
	ctx, span := log.StartSpan(ctx)
	defer span.End()

	// Filter by pod id first.
	if filter == nil {
		return podList
	}

	if filter.GetId() != "" {
		id, err := s.ContainerServer.PodIDIndex().Get(filter.GetId())
		if err != nil {
			// Not finding an ID in a filtered list should not be considered
			// and error; it might have been deleted when stop was done.
			// Log and return an empty struct.
			log.Warnf(ctx, "Unable to find pod %s with filter", filter.GetId())

			return []*sandbox.Sandbox{}
		}

		sb := s.getSandbox(ctx, id)
		if sb == nil {
			podList = []*sandbox.Sandbox{}
		} else {
			podList = []*sandbox.Sandbox{sb}
		}
	}

	finalList := make([]*sandbox.Sandbox, 0, len(podList))

	for _, pod := range podList {
		// Skip sandboxes that aren't created yet
		if !pod.Created() {
			continue
		}

		if filter.GetState() != nil {
			if pod.State() != filter.GetState().GetState() {
				continue
			}
		}

		if filter.LabelSelector != nil {
			sel := fields.SelectorFromSet(filter.GetLabelSelector())
			if !sel.Matches(pod.Labels()) {
				continue
			}
		}

		finalList = append(finalList, pod)
	}

	return finalList
}

// filterSandbox returns whether passed container matches filtering criteria.
func filterSandbox(p *types.PodSandbox, filter *types.PodSandboxFilter) bool {
	if filter != nil {
		if filter.GetState() != nil {
			if p.GetState() != filter.GetState().GetState() {
				return false
			}
		}

		if filter.LabelSelector != nil {
			sel := fields.SelectorFromSet(filter.GetLabelSelector())
			if !sel.Matches(fields.Set(p.GetLabels())) {
				return false
			}
		}
	}

	return true
}
