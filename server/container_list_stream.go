package server

import (
	types "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/cri-o/cri-o/internal/log"
)

// ListContainerStream lists all containers by filters, streaming results one by one.
func (s *Server) ListContainerStream(req *types.ListContainerStreamRequest, stream types.RuntimeService_ListContainerStreamServer) error {
	ctx, span := log.StartSpan(stream.Context())
	defer span.End()

	ctrList, err := s.ContainerServer.ListContainers()
	if err != nil {
		return err
	}

	filter := req.GetFilter()
	if filter != nil {
		ctrList = s.filterContainerList(ctx, filter, ctrList)
	}

	for _, ctr := range ctrList {
		// Skip over containers that are still being created
		if !ctr.Created() {
			continue
		}

		c := ctr.CRIContainer()
		// Filter by other criteria such as state and labels.
		if filterContainer(c, req.GetFilter()) {
			if err := stream.Send(&types.ListContainerStreamResponse{
				Container: c,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
