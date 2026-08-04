package service

import (
	"context"
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	datasetGVR = schema.GroupVersionResource{
		Group: "data.fluid.io", Version: "v1alpha1", Resource: "datasets",
	}
	alluxioRuntimeGVR = schema.GroupVersionResource{
		Group: "data.fluid.io", Version: "v1alpha1", Resource: "alluxioruntimes",
	}
	jindoRuntimeGVR = schema.GroupVersionResource{
		Group: "data.fluid.io", Version: "v1alpha1", Resource: "jindoruntimes",
	}
)

type DatasetService struct {
	kubeClient *k8s.Client
	crdClient  *k8s.CRDClient
}

func NewDatasetService(kubeClient *k8s.Client) *DatasetService {
	return &DatasetService{
		kubeClient: kubeClient,
		crdClient:  k8s.NewCRDClient(kubeClient),
	}
}

func (s *DatasetService) ListDatasets(namespace string) (*unstructured.UnstructuredList, error) {
	if namespace == "" {
		namespace = ""
	}
	return s.crdClient.ListRaw(datasetGVR, namespace)
}

func (s *DatasetService) CreateDataset(name, namespace, datasetConfYAML, runtimeConfYAML string) error {
	// Apply Dataset CR
	datasetConf, err := yamlToUnstructured(datasetConfYAML)
	if err != nil {
		return fmt.Errorf("parse dataset conf: %w", err)
	}
	datasetConf.SetName(name)
	datasetConf.SetNamespace(namespace)

	_, err = s.kubeClient.Dynamic().Resource(datasetGVR).Namespace(namespace).Create(
		context.TODO(), datasetConf, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create dataset cr: %w", err)
	}

	// Apply Runtime CR (AlluxioRuntime or JindoRuntime)
	if runtimeConfYAML != "" {
		runtimeConf, err := yamlToUnstructured(runtimeConfYAML)
		if err != nil {
			return fmt.Errorf("parse runtime conf: %w", err)
		}
		runtimeConf.SetName(name)
		runtimeConf.SetNamespace(namespace)

		gvr := alluxioRuntimeGVR
		if runtimeConf.GetKind() == "JindoRuntime" {
			gvr = jindoRuntimeGVR
		}
		_, err = s.kubeClient.Dynamic().Resource(gvr).Namespace(namespace).Create(
			context.TODO(), runtimeConf, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create runtime cr: %w", err)
		}
	}
	return nil
}

func (s *DatasetService) DeleteDataset(name, namespace string) error {
	if err := s.kubeClient.Dynamic().Resource(alluxioRuntimeGVR).Namespace(namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		// JindoRuntime might be used instead, try that
		_ = s.kubeClient.Dynamic().Resource(jindoRuntimeGVR).Namespace(namespace).Delete(
			context.TODO(), name, metav1.DeleteOptions{})
	}
	return s.kubeClient.Dynamic().Resource(datasetGVR).Namespace(namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{})
}

func yamlToUnstructured(yamlStr string) (*unstructured.Unstructured, error) {
	data, err := yaml.YAMLToJSON([]byte(yamlStr))
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return u, nil
}
