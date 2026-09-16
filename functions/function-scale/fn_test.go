package main

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

const inputAPIVersion = "function-scale.fn.kaonix.com/v1beta1"

// input returns the function input as a structpb.Struct.
func input(scaleTargets string) *structpb.Struct {
	return resource.MustStructJSON(`{
		"apiVersion": "` + inputAPIVersion + `",
		"kind": "Input",
		"spec": {"scaleTargets": ` + scaleTargets + `}
	}`)
}

// deployment returns a desired Deployment resource as a structpb.Struct.
func deployment(name string, replicas int) *structpb.Struct {
	return resource.MustStructJSON(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {"name": "` + name + `"},
		"spec": {"replicas": ` + itoa(replicas) + `}
	}`)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// state returns a desired state with the supplied named resources marked ready.
func state(resources ...*fnv1.Resource) *fnv1.State {
	s := &fnv1.State{Resources: map[string]*fnv1.Resource{}}
	for _, r := range resources {
		s.Resources[r.Resource.Fields["metadata"].GetStructValue().Fields["name"].GetStringValue()] = r
	}
	return s
}

func ready(kind *fnv1.Resource) *fnv1.Resource {
	kind.Ready = fnv1.Ready_READY_TRUE
	return kind
}

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ScaleTargetSetsReplicas": {
			reason: "A scale target that matches a desired resource should set its spec.replicas and report Ready/Synced.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta:    &fnv1.RequestMeta{Tag: "hello"},
					Input:   input(`[{"name":"my-app","replicas":5}]`),
					Desired: state(ready(&fnv1.Resource{Resource: deployment("my-app", 1)})),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:       &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired:    state(ready(&fnv1.Resource{Resource: deployment("my-app", 5)})),
					Conditions: succeededConditions(),
				},
			},
		},
		"UnmatchedTargetSurfacesCondition": {
			reason: "A scale target that matches no desired resource should leave resources unchanged and record a TargetNotFound condition.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta:    &fnv1.RequestMeta{Tag: "hello"},
					Input:   input(`[{"name":"nonexistent","replicas":3}]`),
					Desired: state(ready(&fnv1.Resource{Resource: deployment("other-app", 1)})),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: state(ready(&fnv1.Resource{Resource: deployment("other-app", 1)})),
					Conditions: []*fnv1.Condition{
						syncedFalse(reasonNotFound, "no desired composed resource matched scale target(s): nonexistent"),
						readyTrue(),
					},
				},
			},
		},
		"EmptyInputIsNoOp": {
			reason: "An input with no scale targets should pass the desired state through unchanged with no added conditions.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta:    &fnv1.RequestMeta{Tag: "hello"},
					Input:   input(`[]`),
					Desired: state(ready(&fnv1.Resource{Resource: deployment("my-app", 1)})),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: state(ready(&fnv1.Resource{Resource: deployment("my-app", 1)})),
				},
			},
		},
		"NullInputIsNoOp": {
			reason: "A null input should pass the desired state through unchanged with no errors or conditions.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta:    &fnv1.RequestMeta{Tag: "hello"},
					Desired: state(ready(&fnv1.Resource{Resource: deployment("my-app", 1)})),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: state(ready(&fnv1.Resource{Resource: deployment("my-app", 1)})),
				},
			},
		},
		"MultipleTargetsPatchIndependently": {
			reason: "Multiple scale targets should patch their matching desired resources independently and leave non-matching resources untouched.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta:  &fnv1.RequestMeta{Tag: "hello"},
					Input: input(`[{"name":"app-a","replicas":2},{"name":"app-b","replicas":7}]`),
					Desired: state(
						ready(&fnv1.Resource{Resource: deployment("app-a", 1)}),
						ready(&fnv1.Resource{Resource: deployment("app-b", 1)}),
						ready(&fnv1.Resource{Resource: deployment("sidecar", 1)}),
					),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: state(
						ready(&fnv1.Resource{Resource: deployment("app-a", 2)}),
						ready(&fnv1.Resource{Resource: deployment("app-b", 7)}),
						ready(&fnv1.Resource{Resource: deployment("sidecar", 1)}),
					),
					Conditions: succeededConditions(),
				},
			},
		},
		"RerunIsIdempotent": {
			reason: "Running the function twice with the same input should produce an identical result with no drift.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta:    &fnv1.RequestMeta{Tag: "hello"},
					Input:   input(`[{"name":"my-app","replicas":5}]`),
					Desired: state(ready(&fnv1.Resource{Resource: deployment("my-app", 5)})),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:       &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired:    state(ready(&fnv1.Resource{Resource: deployment("my-app", 5)})),
					Conditions: succeededConditions(),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func succeededConditions() []*fnv1.Condition {
	return []*fnv1.Condition{
		readyTrue(),
		syncedTrue(),
	}
}

func readyTrue() *fnv1.Condition {
	return &fnv1.Condition{
		Type:   conditionReady,
		Status: fnv1.Status_STATUS_CONDITION_TRUE,
		Reason: reasonSucceeded,
		Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
	}
}

func syncedTrue() *fnv1.Condition {
	return &fnv1.Condition{
		Type:   conditionSynced,
		Status: fnv1.Status_STATUS_CONDITION_TRUE,
		Reason: reasonSucceeded,
		Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
	}
}

func syncedFalse(reason, message string) *fnv1.Condition {
	return &fnv1.Condition{
		Type:    conditionSynced,
		Status:  fnv1.Status_STATUS_CONDITION_FALSE,
		Reason:  reason,
		Message: &message,
		Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
	}
}
