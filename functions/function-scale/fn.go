package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/melak-cmd/crossplane-labs/functions/function-scale/input/v1beta1"
)

const (
	conditionReady  = "Ready"
	conditionSynced = "Synced"
	reasonSucceeded = "Succeeded"
	reasonNotFound  = "TargetNotFound"
)

// Function is a Crossplane composition Function that scales desired composed
// Deployments by app name.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction sets spec.replicas on every desired composed resource whose
// metadata.name matches an entry in the Function's input scaleTargets. All
// other desired resources pass through unchanged.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	// The response carries the desired state, context, and tag over from the
	// request, so untouched resources pass through unchanged.
	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if req.GetInput() != nil {
		if err := request.GetInput(req, in); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
			return rsp, nil
		}
	}

	if len(in.Spec.ScaleTargets) == 0 {
		// No scale targets is a no-op: pass the desired state through
		// unchanged, without adding any conditions.
		f.log.Info("No scale targets specified; passing desired state through unchanged")
		return rsp, nil
	}

	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired resources from %T", req))
		return rsp, nil
	}

	// targets maps a desired resource name to the replica count to apply.
	targets := make(map[string]int32, len(in.Spec.ScaleTargets))
	for _, t := range in.Spec.ScaleTargets {
		targets[t.Name] = t.Replicas
	}

	matched := make(map[string]bool, len(targets))
	for name, dcd := range desired {
		resourceName := dcd.Resource.GetName()
		replicas, ok := targets[resourceName]
		if !ok {
			continue
		}
		if err := unstructured.SetNestedField(dcd.Resource.Object, int64(replicas), "spec", "replicas"); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot set spec.replicas on %q", name))
			return rsp, nil
		}
		matched[resourceName] = true
		f.log.Info("Scaled desired resource", "name", name, "replicas", replicas)
	}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources in %T", rsp))
		return rsp, nil
	}

	// Deterministically report targets that matched no desired resource. This
	// does not fail the run; the resources are passed through as-is.
	unmatched := make([]string, 0, len(targets))
	for name := range targets {
		if !matched[name] {
			unmatched = append(unmatched, name)
		}
	}
	sort.Strings(unmatched)
	if len(unmatched) > 0 {
		f.log.Info("Scale targets matched no desired resource", "names", unmatched)
		response.ConditionFalse(rsp, conditionSynced, reasonNotFound).
			WithMessage(fmt.Sprintf("no desired composed resource matched scale target(s): %s", strings.Join(unmatched, ", "))).
			TargetCompositeAndClaim()
	}

	response.ConditionTrue(rsp, conditionReady, reasonSucceeded).TargetCompositeAndClaim()
	if len(unmatched) == 0 {
		response.ConditionTrue(rsp, conditionSynced, reasonSucceeded).TargetCompositeAndClaim()
	}

	return rsp, nil
}
