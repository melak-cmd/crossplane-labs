package main

import (
	"context"
	"testing"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"google.golang.org/protobuf/types/known/structpb"
)

const inputAPIVersion = "function-pause.fn.kaonix.com/v1beta1"

func pauseInput(paused bool) *structpb.Struct {
	return resource.MustStructJSON(`{
		"apiVersion": "` + inputAPIVersion + `",
		"kind": "Input",
		"spec": {
			"target": {
				"apiVersion": "kaonix.com/v1alpha1",
				"kind": "Database",
				"name": "orders",
				"namespace": "platform"
			},
			"paused": ` + boolString(paused) + `
		}
	}`)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func targetResource(annotations string) *structpb.Struct {
	return resource.MustStructJSON(`{
		"apiVersion": "kaonix.com/v1alpha1",
		"kind": "Database",
		"metadata": {
			"name": "orders",
			"namespace": "platform"` + annotations + `
		},
		"spec": {"id": "orders"}
	}`)
}

func requestFor(paused bool, target *structpb.Struct) *fnv1.RunFunctionRequest {
	return &fnv1.RunFunctionRequest{
		Meta:  &fnv1.RequestMeta{Tag: "test"},
		Input: pauseInput(paused),
		RequiredResources: map[string]*fnv1.Resources{
			"target": {Items: []*fnv1.Resource{{Resource: target}}},
		},
	}
}

func TestRunFunction(t *testing.T) {
	tests := map[string]struct {
		paused         bool
		annotations    string
		wantAnnotation string
	}{
		"Pause": {
			paused:         true,
			wantAnnotation: "true",
		},
		"Resume": {
			paused:         false,
			annotations:    `, "annotations": {"crossplane.io/paused": "true", "owner": "test"}`,
			wantAnnotation: "false",
		},
		"IdempotentPause": {
			paused:         true,
			annotations:    `, "annotations": {"crossplane.io/paused": "true"}`,
			wantAnnotation: "true",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			function := &Function{log: logging.NewNopLogger()}
			response, err := function.RunFunction(context.Background(), requestFor(test.paused, targetResource(test.annotations)))
			if err != nil {
				t.Fatalf("RunFunction returned an error: %v", err)
			}
			if len(response.GetDesired().GetResources()) != 1 {
				t.Fatalf("expected one desired resource, got %d", len(response.GetDesired().GetResources()))
			}
			updated := response.GetDesired().GetResources()["orders"].GetResource().AsMap()
			metadata := updated["metadata"].(map[string]any)
			annotations, _ := metadata["annotations"].(map[string]any)
			got, _ := annotations[pausedAnnotation].(string)
			if got != test.wantAnnotation {
				t.Fatalf("pause annotation = %q, want %q", got, test.wantAnnotation)
			}
		})
	}
}

func TestRunFunctionMissingTarget(t *testing.T) {
	function := &Function{log: logging.NewNopLogger()}
	response, err := function.RunFunction(context.Background(), &fnv1.RunFunctionRequest{
		Input: pauseInput(true),
	})
	if err != nil {
		t.Fatalf("RunFunction returned an error: %v", err)
	}
	if len(response.GetConditions()) != 1 || response.GetConditions()[0].GetReason() != reasonNotFound {
		t.Fatalf("expected %q condition, got %#v", reasonNotFound, response.GetConditions())
	}
}
