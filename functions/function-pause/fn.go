package main

import (
	"context"
	"fmt"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/melak-cmd/crossplane-labs/functions/function-pause/input/v1beta1"
)

const (
	conditionSuccess = "FunctionSuccess"
	reasonSuccess    = "Success"
	reasonNotFound   = "TargetNotFound"
	pausedAnnotation = "crossplane.io/paused"
)

// Function pauses or resumes a resource supplied to an Operation as a required resource.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction applies the requested pause state to the Operation target.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	if in.Spec.Target.APIVersion == "" || in.Spec.Target.Kind == "" || in.Spec.Target.Name == "" {
		response.Fatal(rsp, errors.New("target apiVersion, kind, and name are required"))
		return rsp, nil
	}

	resources, resolved, err := request.GetRequiredResource(req, "target")
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get required target resource"))
		return rsp, nil
	}
	if !resolved || len(resources) != 1 {
		message := fmt.Sprintf("target %s/%s %q was not found", in.Spec.Target.Kind, in.Spec.Target.APIVersion, in.Spec.Target.Name)
		response.ConditionFalse(rsp, conditionSuccess, reasonNotFound).WithMessage(message)
		return rsp, nil
	}

	observed := resources[0].Resource
	if observed.GetAPIVersion() != in.Spec.Target.APIVersion || observed.GetKind() != in.Spec.Target.Kind || observed.GetName() != in.Spec.Target.Name || observed.GetNamespace() != in.Spec.Target.Namespace {
		response.ConditionFalse(rsp, conditionSuccess, reasonNotFound).WithMessage("required resource does not match input target")
		return rsp, nil
	}

	// Only the fields we have an opinion about: server-populated fields like
	// managedFields and resourceVersion must not be sent back on apply.
	patch := &unstructured.Unstructured{}
	patch.SetAPIVersion(observed.GetAPIVersion())
	patch.SetKind(observed.GetKind())
	patch.SetName(observed.GetName())
	patch.SetNamespace(observed.GetNamespace())
	patch.SetAnnotations(observed.GetAnnotations())
	setPaused(patch, in.Spec.Paused)

	if err := response.SetDesiredResources(rsp, map[resource.Name]*unstructured.Unstructured{resource.Name(in.Spec.Target.Name): patch}); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot set desired target resource"))
		return rsp, nil
	}

	response.ConditionTrue(rsp, conditionSuccess, reasonSuccess)
	f.log.Info("Updated target pause state", "name", in.Spec.Target.Name, "paused", in.Spec.Paused)

	return rsp, nil
}

func setPaused(resource *unstructured.Unstructured, paused bool) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	// Overwrite rather than delete: each Operation run uses a distinct field
	// manager, so a different manager can't remove a key it doesn't own via
	// omission under server-side apply, but it can force-overwrite its value.
	if paused {
		annotations[pausedAnnotation] = "true"
	} else {
		annotations[pausedAnnotation] = "false"
	}
	resource.SetAnnotations(annotations)
}
