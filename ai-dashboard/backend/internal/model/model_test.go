package model

import (
	"encoding/json"
	"testing"
)

func TestElasticQuotaTree_FindNode(t *testing.T) {
	tree := &ElasticQuotaTree{
		Spec: ElasticQuotaTreeSpec{
			Root: &ElasticQuotaNode{
				Name: "root",
				Children: []*ElasticQuotaNode{
					{
						Name: "root.default",
						Children: []*ElasticQuotaNode{
							{Name: "root.default.groupA"},
							{Name: "root.default.groupB"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		path string
		want string // expected name, empty if nil
	}{
		{"root", "root"},
		{"root.default", "root.default"},
		{"root.default.groupA", "root.default.groupA"},
		{"root.default.groupB", "root.default.groupB"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			node := tree.FindNode(tt.path)
			if tt.want == "" {
				if node != nil {
					t.Errorf("FindNode(%q) should return nil", tt.path)
				}
			} else {
				if node == nil {
					t.Fatalf("FindNode(%q) returned nil", tt.path)
				}
				if node.Name != tt.want {
					t.Errorf("Name = %q, want %q", node.Name, tt.want)
				}
			}
		})
	}
}

func TestElasticQuotaTree_CollectLeafNames(t *testing.T) {
	tree := &ElasticQuotaTree{
		Spec: ElasticQuotaTreeSpec{
			Root: &ElasticQuotaNode{
				Name: "root",
				Children: []*ElasticQuotaNode{
					{
						Name: "root.child1",
						Children: []*ElasticQuotaNode{
							{Name: "root.child1.leaf1"},
							{Name: "root.child1.leaf2"},
						},
					},
					{Name: "root.child2"}, // leaf
				},
			},
		},
	}

	leaves := tree.CollectLeafNames()
	if len(leaves) != 3 {
		t.Fatalf("expected 3 leaves, got %d: %v", len(leaves), leaves)
	}
	expected := map[string]bool{"root.child1.leaf1": true, "root.child1.leaf2": true, "root.child2": true}
	for _, l := range leaves {
		if !expected[l] {
			t.Errorf("unexpected leaf: %s", l)
		}
	}
}

func TestElasticQuotaTree_JSONRoundTrip(t *testing.T) {
	tree := ElasticQuotaTree{
		Spec: ElasticQuotaTreeSpec{
			Root: &ElasticQuotaNode{
				Name: "root",
				Min:  map[string]string{"cpu": "10", "memory": "20Gi"},
				Max:  map[string]string{"cpu": "20", "memory": "40Gi"},
				Children: []*ElasticQuotaNode{
					{
						Name:       "root.default",
						Min:        map[string]string{"cpu": "5"},
						Max:        map[string]string{"cpu": "10"},
						Namespaces: []string{"default-group"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ElasticQuotaTree
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Spec.Root == nil {
		t.Fatal("decoded root is nil")
	}
	if decoded.Spec.Root.Name != "root" {
		t.Errorf("root name = %q", decoded.Spec.Root.Name)
	}
	if decoded.Spec.Root.Min["cpu"] != "10" {
		t.Errorf("root min cpu = %q", decoded.Spec.Root.Min["cpu"])
	}
	if len(decoded.Spec.Root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(decoded.Spec.Root.Children))
	}
	if decoded.Spec.Root.Children[0].Namespaces[0] != "default-group" {
		t.Errorf("child namespace = %v", decoded.Spec.Root.Children[0].Namespaces)
	}
}

func TestUser_JSONRoundTrip(t *testing.T) {
	deletable := true
	user := User{
		Spec: UserSpec{
			UserName:  "test@example.com",
			UserId:    "test-example.com",
			Aliuid:    "123456",
			ApiRoles:  []string{"admin"},
			Groups:    []string{"default-user-group"},
			Deletable:  &deletable,
			K8sServiceAccount: &K8sServiceAccount{
				Name:      "test-example.com",
				Namespace: "kube-ai",
				RoleBindings: []RoleBinding{
					{RoleName: "kubeai-researcher-role", Namespace: "default-group"},
				},
				ClusterRoleBindings: []RoleBinding{
					{RoleName: "kubeai-admin-clusterrole"},
				},
			},
		},
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Spec.UserName != "test@example.com" {
		t.Errorf("userName = %q", decoded.Spec.UserName)
	}
	if decoded.Spec.Deletable == nil || !*decoded.Spec.Deletable {
		t.Error("deletable should be true")
	}
	if len(decoded.Spec.K8sServiceAccount.RoleBindings) != 1 {
		t.Errorf("roleBindings len = %d", len(decoded.Spec.K8sServiceAccount.RoleBindings))
	}
}
