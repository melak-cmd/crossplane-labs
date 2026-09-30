package function

import (
	"context"
	"errors"
	"strings"
	"testing"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/cnpg"
	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakeCluster is an in-memory ClusterClient. It mimics what the real client
// persists on the PostgreSQLRestore so that phase handling can be tested
// without a cluster.
type fakeCluster struct {
	restore   *unstructured.Unstructured
	calls     []string
	deleteErr error
}

func (f *fakeCluster) count(call string) int {
	n := 0
	for _, c := range f.calls {
		if c == call {
			n++
		}
	}
	return n
}

func (f *fakeCluster) annotations() map[string]string {
	if a := f.restore.GetAnnotations(); a != nil {
		return a
	}
	return map[string]string{}
}

func (f *fakeCluster) setAnnotations(add map[string]string) {
	a := f.annotations()
	for k, v := range add {
		a[k] = v
	}
	f.restore.SetAnnotations(a)
}

func (f *fakeCluster) phase() model.Phase {
	return model.Phase(f.annotations()[model.PhaseAnnotation])
}

func (f *fakeCluster) GetCluster(context.Context, string, string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform", "uid": "u"},
		"spec": map[string]interface{}{
			"instances": int64(1),
			"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}},
		},
	}}, nil
}

func (f *fakeCluster) GetPostgreSQLRestore(context.Context, string, string) (*unstructured.Unstructured, error) {
	return f.restore.DeepCopy(), nil
}

func (f *fakeCluster) SetRestorePhase(_ context.Context, _ model.RestoreRef, phase model.Phase) error {
	f.calls = append(f.calls, "phase:"+string(phase))
	f.setAnnotations(map[string]string{model.PhaseAnnotation: string(phase)})
	return nil
}

func (f *fakeCluster) GetPostgreSQL(context.Context, string, string) (*unstructured.Unstructured, error) {
	return postgresqlXR(), nil
}

func (f *fakeCluster) ResumePostgreSQL(context.Context, string, string) error {
	f.calls = append(f.calls, "resume")
	return nil
}

func (f *fakeCluster) CreateRestoredCluster(_ context.Context, cluster *unstructured.Unstructured) error {
	f.calls = append(f.calls, "restore")
	bootstrap, _, _ := unstructured.NestedMap(cluster.Object, "spec", "bootstrap")
	if _, ok := bootstrap["recovery"]; !ok {
		return errors.New("restored Cluster has no recovery bootstrap")
	}
	return nil
}

func (f *fakeCluster) DeleteAndWait(context.Context, string, string) error {
	f.calls = append(f.calls, "delete")
	return f.deleteErr
}

func (f *fakeCluster) RemoveRecovery(context.Context, string, string) error {
	f.calls = append(f.calls, "cleanup")
	return nil
}

func (f *fakeCluster) PrepareAndDelete(ctx context.Context, restore model.RestoreRef, pgNamespace, pgName, clusterNamespace, clusterName, plan string) error {
	if err := f.PrepareRecovery(ctx, restore, pgNamespace, pgName, plan); err != nil {
		return err
	}
	return f.DeleteAndWait(ctx, clusterNamespace, clusterName)
}

func (f *fakeCluster) PrepareRecovery(_ context.Context, _ model.RestoreRef, _, _ string, plan string) error {
	f.calls = append(f.calls, "prepare")
	f.setAnnotations(map[string]string{
		model.PlanAnnotation:  plan,
		model.PhaseAnnotation: string(model.PhasePrepared),
	})
	return nil
}

func postgresqlXR() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQL",
		"metadata":   map[string]interface{}{"name": "orders-database", "namespace": "platform"},
		"spec": map[string]interface{}{"crossplane": map[string]interface{}{"resourceRefs": []interface{}{
			map[string]interface{}{"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster", "name": "orders-primary"},
		}}},
	}}
}

func restoreObject(annotations map[string]interface{}) *unstructured.Unstructured {
	metadata := map[string]interface{}{"name": "orders-restore", "namespace": "platform"}
	if annotations != nil {
		metadata["annotations"] = annotations
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQLRestore",
		"metadata":   metadata,
		"spec":       map[string]interface{}{"name": "orders-database", "backupName": "orders-backup"},
	}}
}

func newFake(annotations map[string]interface{}) *fakeCluster {
	return &fakeCluster{restore: restoreObject(annotations)}
}

func watchedRequest(mode string, snapshot *unstructured.Unstructured) *fnv1.RunFunctionRequest {
	return &fnv1.RunFunctionRequest{
		Input: resource.MustStructJSON(`{"apiVersion":"function-pg-recovery.fn.database.nuagik.sncf.fr/v1beta1","kind":"Input","spec":{"mode":"` + mode + `","target":{},"watchedRequest":true}}`),
		RequiredResources: map[string]*fnv1.Resources{
			"ops.crossplane.io/watched-resource": {Items: []*fnv1.Resource{{Resource: resource.MustStructObject(snapshot)}}},
		},
	}
}

func manualRequest(mode string, withRestore bool) *fnv1.RunFunctionRequest {
	input := `{"spec":{"mode":"` + mode + `","target":{"namespace":"platform"}`
	if mode == "restore" {
		input += `,"backup":{"name":"orders-backup","namespace":"platform"}`
	}
	input += `}}`
	req := &fnv1.RunFunctionRequest{
		Input: resource.MustStructJSON(input),
		RequiredResources: map[string]*fnv1.Resources{
			"postgresql": {Items: []*fnv1.Resource{{Resource: resource.MustStructObject(postgresqlXR())}}},
		},
	}
	if withRestore {
		req.RequiredResources["postgresqlrestore"] = &fnv1.Resources{Items: []*fnv1.Resource{{Resource: resource.MustStructObject(restoreObject(nil))}}}
	}
	return req
}

func run(t *testing.T, f *fakeCluster, req *fnv1.RunFunctionRequest) *fnv1.RunFunctionResponse {
	t.Helper()
	rsp, err := New(nil, f).RunFunction(context.Background(), req)
	if err != nil {
		t.Fatalf("RunFunction returned an error: %v", err)
	}
	return rsp
}

func message(rsp *fnv1.RunFunctionResponse) string {
	message, _ := rsp.GetOutput().AsMap()["message"].(string)
	return message
}

var orderedModes = []string{"prepare", "delete", "restore", "cleanup", "resume"}

func TestStepsAdvanceThePhaseInOrderEvenWithAStaleWatchedSnapshot(t *testing.T) {
	f := newFake(nil)
	snapshot := restoreObject(nil) // never updated: later steps must not rely on it
	want := []model.Phase{model.PhasePrepared, model.PhaseDeleted, model.PhaseRestored, model.PhaseCleaned, model.PhaseResumed}
	for i, mode := range orderedModes {
		rsp := run(t, f, watchedRequest(mode, snapshot))
		if Failed(rsp) {
			t.Fatalf("%s failed: %s", mode, message(rsp))
		}
		if f.phase() != want[i] {
			t.Fatalf("after %s phase = %q, want %q", mode, f.phase(), want[i])
		}
	}
	for _, call := range []string{"prepare", "delete", "restore", "cleanup", "resume"} {
		if f.count(call) != 1 {
			t.Fatalf("%s ran %d times, want once (calls: %v)", call, f.count(call), f.calls)
		}
	}
	if f.annotations()[model.PlanAnnotation] == "" {
		t.Fatal("expected the plan annotation to stay on the PostgreSQLRestore")
	}
}

func TestEveryStepRunsOnlyInItsPredecessorPhase(t *testing.T) {
	phases := []model.Phase{model.PhaseNone, model.PhasePrepared, model.PhaseDeleted, model.PhaseRestored, model.PhaseCleaned, model.PhaseResumed}
	for _, mode := range orderedModes {
		required, _, _ := model.PhaseGate(model.Operation(mode))
		for _, phase := range phases {
			t.Run(mode+"/"+string(phase), func(t *testing.T) {
				annotations := map[string]interface{}{}
				if phase != model.PhaseNone {
					annotations[model.PhaseAnnotation] = string(phase)
				}
				f := newFake(annotations)
				// A valid plan is present so that a wrongly running restore would succeed.
				plan, _ := (&fakeCluster{}).GetCluster(context.Background(), "", "")
				f.setAnnotations(map[string]string{model.PlanAnnotation: clusterJSON(t, plan)})

				rsp := run(t, f, watchedRequest(mode, restoreObject(nil)))
				if Failed(rsp) {
					t.Fatalf("unexpected failure: %s", message(rsp))
				}
				ran := f.count(mode) == 1
				if ran != (phase == required) {
					t.Fatalf("ran=%v in phase %q, required phase is %q (calls: %v)", ran, phase, required, f.calls)
				}
				if !ran {
					if !strings.Contains(message(rsp), "skipped") {
						t.Fatalf("expected a skipped message, got %q", message(rsp))
					}
					if f.phase() != phase {
						t.Fatalf("a skipped step changed the phase from %q to %q", phase, f.phase())
					}
				}
			})
		}
	}
}

func TestDuplicateOperationRepeatsNoCompletedWork(t *testing.T) {
	f := newFake(nil)
	for _, mode := range orderedModes {
		run(t, f, watchedRequest(mode, restoreObject(nil)))
	}
	callsAfterFirstRun := len(f.calls)
	for _, mode := range orderedModes {
		rsp := run(t, f, watchedRequest(mode, restoreObject(nil)))
		if Failed(rsp) {
			t.Fatalf("duplicate %s must succeed as a no-op: %s", mode, message(rsp))
		}
	}
	if len(f.calls) != callsAfterFirstRun {
		t.Fatalf("duplicate Operation changed resources: %v", f.calls[callsAfterFirstRun:])
	}
	if f.phase() != model.PhaseResumed {
		t.Fatalf("phase = %q, want %q", f.phase(), model.PhaseResumed)
	}
}

func TestDeleteDoesNotRunAfterTheClusterWasRestored(t *testing.T) {
	f := newFake(map[string]interface{}{model.PhaseAnnotation: string(model.PhaseRestored)})
	rsp := run(t, f, watchedRequest("delete", restoreObject(nil)))
	if Failed(rsp) || f.count("delete") != 0 {
		t.Fatalf("delete must not run in phase restored (failed=%v, calls=%v)", Failed(rsp), f.calls)
	}
}

func TestFailedStepDoesNotAdvanceThePhase(t *testing.T) {
	f := newFake(map[string]interface{}{model.PhaseAnnotation: string(model.PhasePrepared)})
	f.deleteErr = errors.New("delete timed out")
	rsp := run(t, f, watchedRequest("delete", restoreObject(nil)))
	if !Failed(rsp) {
		t.Fatal("expected the delete step to fail")
	}
	if f.phase() != model.PhasePrepared {
		t.Fatalf("phase = %q after a failed step, want %q", f.phase(), model.PhasePrepared)
	}
	if f.count("phase:"+string(model.PhaseDeleted)) != 0 {
		t.Fatal("the phase must not be written when the step fails")
	}
}

func TestRestoreReadsThePlanFromTheLiveRestoreNotTheSnapshot(t *testing.T) {
	f := newFake(map[string]interface{}{model.PhaseAnnotation: string(model.PhaseDeleted)})
	plan, _ := (&fakeCluster{}).GetCluster(context.Background(), "", "")
	f.setAnnotations(map[string]string{model.PlanAnnotation: clusterJSON(t, plan)})
	rsp := run(t, f, watchedRequest("restore", restoreObject(nil))) // snapshot has no plan
	if Failed(rsp) {
		t.Fatalf("restore failed: %s", message(rsp))
	}
	if f.count("restore") != 1 {
		t.Fatalf("expected the cluster to be restored from the live plan, calls: %v", f.calls)
	}
}

func TestRestoreFailsClearlyWhenThePlanAnnotationIsMissing(t *testing.T) {
	f := newFake(map[string]interface{}{model.PhaseAnnotation: string(model.PhaseDeleted)})
	rsp := run(t, f, watchedRequest("restore", restoreObject(nil)))
	if !Failed(rsp) || !strings.Contains(message(rsp), "no recovery plan found on PostgreSQLRestore platform/orders-restore") {
		t.Fatalf("expected a missing plan error, got failed=%v message=%q", Failed(rsp), message(rsp))
	}
	if f.count("restore") != 0 || f.phase() != model.PhaseDeleted {
		t.Fatalf("a failed restore must change nothing (calls=%v phase=%q)", f.calls, f.phase())
	}
}

func TestManualOperationStoresPlanAndPhaseOnTheSuppliedRestore(t *testing.T) {
	f := newFake(nil)
	for _, mode := range []string{"prepare", "delete", "restore"} {
		rsp := run(t, f, manualRequest(mode, true))
		if Failed(rsp) {
			t.Fatalf("manual %s failed: %s", mode, message(rsp))
		}
	}
	if f.annotations()[model.PlanAnnotation] == "" || f.phase() != model.PhaseRestored {
		t.Fatalf("expected plan and phase restored on the PostgreSQLRestore, got %v", f.annotations())
	}
}

func TestManualOperationRequiresAPostgreSQLRestore(t *testing.T) {
	for _, mode := range []string{"prepare", "restore"} {
		f := newFake(nil)
		rsp := run(t, f, manualRequest(mode, false))
		if !Failed(rsp) || !strings.Contains(message(rsp), "PostgreSQLRestore") {
			t.Fatalf("%s without a PostgreSQLRestore: failed=%v message=%q", mode, Failed(rsp), message(rsp))
		}
		if len(f.calls) != 0 {
			t.Fatalf("%s changed resources without a PostgreSQLRestore: %v", mode, f.calls)
		}
	}
}

func TestLegacyPlanNameInputIsRejected(t *testing.T) {
	f := newFake(nil)
	req := watchedRequest("prepare", restoreObject(nil))
	req.Input = resource.MustStructJSON(`{"spec":{"mode":"prepare","target":{},"watchedRequest":true,"planName":"orders-recovery-plan"}}`)
	rsp := run(t, f, req)
	if !Failed(rsp) || !strings.Contains(message(rsp), "planName is no longer supported") {
		t.Fatalf("expected planName to be rejected, got failed=%v message=%q", Failed(rsp), message(rsp))
	}
	if len(f.calls) != 0 {
		t.Fatalf("rejected input must change nothing: %v", f.calls)
	}
}

func clusterJSON(t *testing.T, cluster *unstructured.Unstructured) string {
	t.Helper()
	in := &model.Input{Spec: model.InputSpec{Target: model.ClusterReference{Name: "orders-primary", Namespace: "platform"}}}
	plan, err := cnpg.RecoveryPlan(in, cluster)
	if err != nil {
		t.Fatalf("cannot build plan: %v", err)
	}
	return plan
}
