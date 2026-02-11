package server_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	types "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/cri-o/cri-o/internal/oci"
	containerliststreamservermock "github.com/cri-o/cri-o/test/mocks/containerliststream"
)

var _ = t.Describe("ListContainerStream", func() {
	BeforeEach(func() {
		beforeEach()
		setupSUT()
	})

	AfterEach(afterEach)

	t.Describe("ListContainerStream", func() {
		DescribeTable("should succeed", func(
			givenState *oci.ContainerState,
			expectedState types.ContainerState,
			created bool,
		) {
			// Given
			addContainerAndSandbox()
			if created {
				testContainer.SetCreated()
			}
			testContainer.SetState(givenState)

			streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
			streamMock.EXPECT().Context().Return(context.Background())
			if created {
				streamMock.EXPECT().Send(gomock.Any()).DoAndReturn(
					func(resp *types.ListContainerStreamResponse) error {
						Expect(resp.GetContainer().GetState()).To(Equal(expectedState))
						return nil
					},
				)
			}

			// When
			err := sut.ListContainerStream(
				&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{}},
				streamMock,
			)

			// Then
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("Created 1", &oci.ContainerState{
				State: specs.State{Status: oci.ContainerStateCreated},
			}, types.ContainerState_CONTAINER_CREATED, true),
			Entry("Created 2", &oci.ContainerState{
				State: specs.State{Status: oci.ContainerStateCreated},
			}, types.ContainerState_CONTAINER_CREATED, false),
			Entry("Running", &oci.ContainerState{
				State: specs.State{Status: oci.ContainerStateRunning},
			}, types.ContainerState_CONTAINER_RUNNING, true),
			Entry("Stopped", &oci.ContainerState{
				State: specs.State{Status: oci.ContainerStateStopped},
			}, types.ContainerState_CONTAINER_EXITED, true),
		)

		t.Describe("ListContainerStream Filter", func() {
			BeforeEach(func() {
				addContainerAndSandbox()
				testContainer.SetCreated()
			})

			It("should succeed with non matching filter", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						Id: "id",
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})

			It("should succeed with matching filter", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())
				streamMock.EXPECT().Send(gomock.Any()).Return(nil)

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						Id: testContainer.ID(),
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})

			It("should succeed with non matching filter for sandbox ID", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						Id:           testContainer.ID(),
						PodSandboxId: "id",
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})

			It("should succeed with matching filter for sandbox and container ID", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())
				streamMock.EXPECT().Send(gomock.Any()).Return(nil)

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						Id:           testContainer.ID(),
						PodSandboxId: testSandbox.ID(),
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})

			It("should succeed with matching filter for sandbox ID", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())
				streamMock.EXPECT().Send(gomock.Any()).Return(nil)

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						PodSandboxId: testSandbox.ID(),
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})

			It("should succeed with state filter", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						State: &types.ContainerStateValue{
							State: types.ContainerState_CONTAINER_RUNNING,
						},
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})

			It("should succeed with label filter", func() {
				// Given
				streamMock := containerliststreamservermock.NewMockRuntimeService_ListContainerStreamServer[string](mockCtrl)
				streamMock.EXPECT().Context().Return(context.Background())

				// When
				err := sut.ListContainerStream(
					&types.ListContainerStreamRequest{Filter: &types.ContainerFilter{
						LabelSelector: map[string]string{"label": "label"},
					}},
					streamMock,
				)

				// Then
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
