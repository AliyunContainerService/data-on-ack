package service

import (
	"fmt"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var eqTreeGVR = schema.GroupVersionResource{
	Group: "scheduling.sigs.k8s.io", Version: "v1beta1", Resource: "elasticquotatrees",
}

const eqTreeNamespace = "kube-system"

type QuotaService struct {
	kubeClient *k8s.Client
	crdClient  *k8s.CRDClient
}

func NewQuotaService(kubeClient *k8s.Client) *QuotaService {
	return &QuotaService{
		kubeClient: kubeClient,
		crdClient:  k8s.NewCRDClient(kubeClient),
	}
}

// ListQuotaTrees returns all ElasticQuotaTree CRs.
func (s *QuotaService) ListQuotaTrees() ([]model.ElasticQuotaTree, error) {
	var list model.ElasticQuotaTreeList
	if err := s.crdClient.List(eqTreeGVR, eqTreeNamespace, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetQuotaTree returns a specific ElasticQuotaTree by name.
func (s *QuotaService) GetQuotaTree(name string) (*model.ElasticQuotaTree, error) {
	var tree model.ElasticQuotaTree
	if err := s.crdClient.Get(eqTreeGVR, eqTreeNamespace, name, &tree); err != nil {
		return nil, err
	}
	return &tree, nil
}

// CreateQuotaTree creates a new ElasticQuotaTree CR.
func (s *QuotaService) CreateQuotaTree(tree *model.ElasticQuotaTree) error {
	tree.Namespace = eqTreeNamespace
	return s.crdClient.CreateOrReplace(eqTreeGVR, eqTreeNamespace, tree)
}

// AddNode adds a child node under the specified parent path in the tree.
func (s *QuotaService) AddNode(treeName, parentPath string, node *model.ElasticQuotaNode) error {
	tree, err := s.GetQuotaTree(treeName)
	if err != nil {
		return err
	}
	if tree.Spec.Root == nil {
		return fmt.Errorf("tree has no root")
	}

	parent := tree.FindNode(parentPath)
	if parent == nil {
		return fmt.Errorf("parent node %s not found", parentPath)
	}

	fullName := buildNodeName(parentPath, node.Name)
	node.Name = fullName
	parent.Children = append(parent.Children, node)

	return s.crdClient.CreateOrReplace(eqTreeGVR, eqTreeNamespace, tree)
}

// DeleteNode removes a node from the tree by path.
func (s *QuotaService) DeleteNode(treeName, nodePath string) error {
	tree, err := s.GetQuotaTree(treeName)
	if err != nil {
		return err
	}
	if tree.Spec.Root == nil {
		return fmt.Errorf("tree has no root")
	}

	if tree.Spec.Root.Name == nodePath {
		return fmt.Errorf("cannot delete root node")
	}

	if !removeNodeFromTree(tree.Spec.Root, nodePath) {
		return fmt.Errorf("node %s not found", nodePath)
	}
	return s.crdClient.CreateOrReplace(eqTreeGVR, eqTreeNamespace, tree)
}

// UpdateNode updates a node's min/max/namespaces in the tree.
func (s *QuotaService) UpdateNode(treeName, nodePath string, node *model.ElasticQuotaNode) error {
	tree, err := s.GetQuotaTree(treeName)
	if err != nil {
		return err
	}
	existing := tree.FindNode(nodePath)
	if existing == nil {
		return fmt.Errorf("node %s not found", nodePath)
	}

	if node.Min != nil {
		existing.Min = node.Min
	}
	if node.Max != nil {
		existing.Max = node.Max
	}
	if node.Namespaces != nil {
		existing.Namespaces = node.Namespaces
	}
	if node.Name != "" && node.Name != nodePath {
		existing.Name = node.Name
	}

	return s.crdClient.CreateOrReplace(eqTreeGVR, eqTreeNamespace, tree)
}

// GetQuotaNamespaces returns all namespaces bound to the given quota node names.
func (s *QuotaService) GetQuotaNamespaces(quotaNames []string) ([]string, error) {
	trees, err := s.ListQuotaTrees()
	if err != nil {
		return nil, err
	}
	var namespaces []string
	for _, tree := range trees {
		if tree.Spec.Root == nil {
			continue
		}
		for _, qn := range quotaNames {
			if node := tree.FindNode(qn); node != nil && len(node.Namespaces) > 0 {
				namespaces = append(namespaces, node.Namespaces...)
			}
		}
	}
	return uniqueStrings(namespaces), nil
}

// buildNodeName creates a dotted-path node name: "parent.child".
func buildNodeName(parentPath, childName string) string {
	if parentPath == "" {
		return childName
	}
	return parentPath + "." + childName
}

// removeNodeFromTree recursively removes a node by path from the children.
func removeNodeFromTree(parent *model.ElasticQuotaNode, targetPath string) bool {
	for i, child := range parent.Children {
		if child.Name == targetPath {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return true
		}
		if removeNodeFromTree(child, targetPath) {
			return true
		}
	}
	return false
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// Prefix-aware node name serialization (for API compatibility with original Java).
// Node names in the CRD are stored as "root.parent.child" dotted paths.
func SerializeNodeName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func DeserializeNodeName(fullName string) (prefix, name string) {
	idx := strings.LastIndex(fullName, ".")
	if idx == -1 {
		return "", fullName
	}
	return fullName[:idx], fullName[idx+1:]
}
