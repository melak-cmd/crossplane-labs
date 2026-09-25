package function

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestReadClusterName(t *testing.T) {
	tests := []struct {
		name    string
		refs    []interface{}
		want    string
		wantErr bool
	}{
		{
			name: "finds the CNPG Cluster ref",
			refs: []interface{}{
				map[string]interface{}{"apiVersion": "postgresql.cnpg.io/v1", "kind": "ScheduledBackup", "name": "orders-backup"},
				map[string]interface{}{"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster", "name": "orders-primary"},
			},
			want: "orders-primary",
		},
		{
			name:    "requires a Cluster ref",
			refs:    []interface{}{},
			wantErr: true,
		},
		{
			name: "rejects multiple Cluster refs",
			refs: []interface{}{
				map[string]interface{}{"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster", "name": "orders-primary"},
				map[string]interface{}{"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster", "name": "orders-replica"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "database.kaonix.inc.fr/v1alpha1",
				"kind":       "PostgreSQL",
				"spec": map[string]interface{}{
					"crossplane": map[string]interface{}{"resourceRefs": tt.refs},
				},
			}}
			got, err := ReadClusterName(database)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadClusterName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ReadClusterName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadClusterNameRejectsUnexpectedResourceKind(t *testing.T) {
	database := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.kaonix.inc.fr/v1alpha1",
		"kind":       "Database",
		"spec":       map[string]interface{}{"crossplane": map[string]interface{}{"resourceRefs": []interface{}{}}},
	}}
	if _, err := ReadClusterName(database); err == nil {
		t.Fatal("expected non-PostgreSQL resource to be rejected")
	}
}
