package service

import (
	"testing"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
)

func TestWalkForNamespace(t *testing.T) {
	tree := model.ElasticQuotaTree{
		Spec: model.ElasticQuotaTreeSpec{Root: &model.ElasticQuotaNode{
			Name: "root",
			Children: []*model.ElasticQuotaNode{
				{Name: "team-a", Min: map[string]string{"cpu": "100"}, Namespaces: []string{"ns-a1", "ns-a2"}},
				{Name: "team-b", Namespaces: []string{"ns-b1"}},
			},
		}},
	}
	if leaf := walkForNamespace(tree.Spec.Root, "ns-a2"); leaf == nil || leaf.Name != "team-a" {
		t.Fatalf("expected team-a leaf, got %+v", leaf)
	}
	if leaf := walkForNamespace(tree.Spec.Root, "ns-b1"); leaf == nil || leaf.Name != "team-b" {
		t.Fatalf("expected team-b leaf, got %+v", leaf)
	}
	if leaf := walkForNamespace(tree.Spec.Root, "missing"); leaf != nil {
		t.Fatalf("expected nil for missing namespace, got %+v", leaf)
	}
	if leaf := walkForNamespace(nil, "x"); leaf != nil {
		t.Fatal("nil node must return nil")
	}
}
