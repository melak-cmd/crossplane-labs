package main

import (
	"context"
	"encoding/json"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/input/v1beta1"
)

const (
	conditionSuccess = "FunctionSuccess"
	reasonSuccess    = "Success"
	reasonInvalid    = "InvalidRecoveryInput"
	modePrepare      = "prepare"
	modeRestore      = "restore"
	planDataKey      = "manifest.json"
)

type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	log logging.Logger
}

func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	rsp := response.To(req, response.DefaultTTL)
	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get recovery input"))
		return rsp, nil
	}
	if err := validate(in); err != nil {
		response.ConditionFalse(rsp, conditionSuccess, reasonInvalid).WithMessage(err.Error())
		return rsp, nil
	}
	requiredName := "cluster-object"
	if in.Spec.Mode == modeRestore {
		requiredName = "recovery-plan"
	}
	required, resolved, err := request.GetRequiredResource(req, requiredName)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get recovery resource"))
		return rsp, nil
	}
	if !resolved || len(required) != 1 {
		response.ConditionFalse(rsp, conditionSuccess, reasonInvalid).WithMessage("required recovery resource is not resolved")
		return rsp, nil
	}

	var desired map[resource.Name]*unstructured.Unstructured
	if in.Spec.Mode == modePrepare {
		plan, err := renderPlan(in, required[0].Resource)
		if err != nil {
			response.ConditionFalse(rsp, conditionSuccess, reasonInvalid).WithMessage(err.Error())
			return rsp, nil
		}
		desired = map[resource.Name]*unstructured.Unstructured{resource.Name(in.Spec.PlanName): plan}
	} else {
		object, err := renderRestoredObject(in, required[0].Resource)
		if err != nil {
			response.ConditionFalse(rsp, conditionSuccess, reasonInvalid).WithMessage(err.Error())
			return rsp, nil
		}
		desired = map[resource.Name]*unstructured.Unstructured{resource.Name(in.Spec.Target.Name): object}
	}
	if err := response.SetDesiredResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot set recovery Cluster"))
		return rsp, nil
	}
	response.ConditionTrue(rsp, conditionSuccess, reasonSuccess)
	return rsp, nil
}

func validate(in *v1beta1.Input) error {
	if in.Spec.Target.Name == "" || in.Spec.Target.Namespace == "" {
		return errors.New("target name and namespace are required")
	}
	if in.Spec.Mode != modePrepare && in.Spec.Mode != modeRestore {
		return errors.New("mode must be prepare or restore")
	}
	if in.Spec.PlanName == "" {
		return errors.New("planName is required")
	}
	if in.Spec.Mode == modePrepare {
		return nil
	}
	backup := in.Spec.Backup != nil
	snapshots := in.Spec.VolumeSnapshots != nil
	if backup == snapshots {
		return errors.New("exactly one of backup or volumeSnapshots is required")
	}
	if backup && (in.Spec.Backup.Name == "" || in.Spec.Backup.Namespace == "") {
		return errors.New("backup name and namespace are required")
	}
	if backup && in.Spec.Backup.Namespace != in.Spec.Target.Namespace {
		return errors.New("backup and target must use the same namespace")
	}
	if snapshots && (in.Spec.VolumeSnapshots.Data == "" || in.Spec.VolumeSnapshots.Wal == "" || in.Spec.VolumeSnapshots.StorageClass == "") {
		return errors.New("volumeSnapshots data, wal, and storageClass are required")
	}
	return nil
}

func renderPlan(in *v1beta1.Input, object *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	manifest, found, err := unstructured.NestedMap(object.Object, "spec", "forProvider", "manifest")
	if err != nil || !found {
		return nil, errors.New("cluster Object does not contain spec.forProvider.manifest")
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, errors.Wrap(err, "cannot encode cluster manifest")
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": in.Spec.PlanName, "namespace": in.Spec.Target.Namespace},
		"data":     map[string]interface{}{planDataKey: string(encoded)},
	}}, nil
}

func renderRestoredObject(in *v1beta1.Input, plan *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	data, found, err := unstructured.NestedStringMap(plan.Object, "data")
	if err != nil || !found || data[planDataKey] == "" {
		return nil, errors.New("recovery plan does not contain manifest.json")
	}
	manifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(data[planDataKey]), &manifest); err != nil {
		return nil, errors.Wrap(err, "cannot decode recovery plan")
	}
	if manifest["kind"] != "Cluster" || manifest["apiVersion"] != "postgresql.cnpg.io/v1" {
		return nil, errors.New("recovery plan does not contain a CNPG Cluster manifest")
	}
	unstructured.RemoveNestedField(manifest, "spec", "bootstrap", "initdb")
	bootstrap, _, _ := unstructured.NestedMap(manifest, "spec", "bootstrap")
	if bootstrap == nil {
		bootstrap = map[string]interface{}{}
	}
	if in.Spec.Backup != nil {
		bootstrap["recovery"] = map[string]interface{}{"backup": map[string]interface{}{"name": in.Spec.Backup.Name}}
	} else {
		bootstrap["recovery"] = map[string]interface{}{"volumeSnapshots": map[string]interface{}{
			"storage":    map[string]interface{}{"storageClass": in.Spec.VolumeSnapshots.StorageClass, "volumeSnapshot": map[string]interface{}{"name": in.Spec.VolumeSnapshots.Data}},
			"walStorage": map[string]interface{}{"volumeSnapshot": map[string]interface{}{"name": in.Spec.VolumeSnapshots.Wal}},
		}}
	}
	if err := unstructured.SetNestedMap(manifest, bootstrap, "spec", "bootstrap"); err != nil {
		return nil, errors.Wrap(err, "cannot set recovery bootstrap")
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kubernetes.m.crossplane.io/v1alpha1", "kind": "Object",
		"metadata": map[string]interface{}{
			"name":      in.Spec.Target.Name,
			"namespace": in.Spec.Target.Namespace,
			"annotations": map[string]interface{}{
				"crossplane.io/composition-resource-name": "cluster",
			},
		},
		"spec": map[string]interface{}{
			"forProvider":       map[string]interface{}{"manifest": manifest},
			"providerConfigRef": map[string]interface{}{"kind": "ProviderConfig", "name": "default"},
		},
	}}, nil
}
