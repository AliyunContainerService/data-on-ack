package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- ElasticQuotaTree CRD (scheduling.sigs.k8s.io/v1beta1) ---

type ElasticQuotaTree struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ElasticQuotaTreeSpec `json:"spec,omitempty"`
	Status            ElasticQuotaTreeStatus `json:"status,omitempty"`
}

type ElasticQuotaTreeSpec struct {
	Root *ElasticQuotaNode `json:"root,omitempty"`
}

type ElasticQuotaTreeStatus struct {
	Root *ElasticQuotaNodeStatus `json:"root,omitempty"`
}

// ElasticQuotaNode is a node in the elastic quota tree.
type ElasticQuotaNode struct {
	Name       string             `json:"name"`
	Min        map[string]string  `json:"min,omitempty"`
	Max        map[string]string  `json:"max,omitempty"`
	Children   []*ElasticQuotaNode `json:"children,omitempty"`
	Namespaces []string           `json:"namespaces,omitempty"`
}

// ElasticQuotaNodeStatus mirrors ElasticQuotaNode but for status.
type ElasticQuotaNodeStatus struct {
	Name       string             `json:"name"`
	Min        map[string]string  `json:"min,omitempty"`
	Max        map[string]string  `json:"max,omitempty"`
	Children   []*ElasticQuotaNodeStatus `json:"children,omitempty"`
	Namespaces []string           `json:"namespaces,omitempty"`
}

type ElasticQuotaTreeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElasticQuotaTree `json:"items"`
}

// --- ElasticQuotaTree helpers ---

// FindNode searches the tree for a node by dotted path (e.g. "root.child1.child2").
func (t *ElasticQuotaTree) FindNode(path string) *ElasticQuotaNode {
	if t.Spec.Root == nil {
		return nil
	}
	return findNodeInTree(t.Spec.Root, path)
}

func findNodeInTree(root *ElasticQuotaNode, path string) *ElasticQuotaNode {
	if root == nil {
		return nil
	}
	if root.Name == path {
		return root
	}
	for _, child := range root.Children {
		if found := findNodeInTree(child, path); found != nil {
			return found
		}
	}
	return nil
}

// CollectLeafNames returns all leaf node names (nodes without children).
func (t *ElasticQuotaTree) CollectLeafNames() []string {
	if t.Spec.Root == nil {
		return nil
	}
	var names []string
	collectLeafNames(t.Spec.Root, &names)
	return names
}

func collectLeafNames(node *ElasticQuotaNode, names *[]string) {
	if len(node.Children) == 0 {
		*names = append(*names, node.Name)
		return
	}
	for _, child := range node.Children {
		collectLeafNames(child, names)
	}
}
