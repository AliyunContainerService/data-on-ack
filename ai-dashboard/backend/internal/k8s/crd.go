package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CRDClient provides typed CRUD operations on a CRD via the dynamic client.
type CRDClient struct {
	client *Client
}

func NewCRDClient(c *Client) *CRDClient {
	return &CRDClient{client: c}
}

// Create creates a CRD resource from a Go struct.
func (d *CRDClient) Create(gvr schema.GroupVersionResource, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(data); err != nil {
		return nil, fmt.Errorf("unmarshal to unstructured: %w", err)
	}
	return d.client.dynamic.Resource(gvr).Namespace(namespace).Create(context.TODO(), u, metav1.CreateOptions{})
}

// Get retrieves a CRD resource and unmarshals into the target.
func (d *CRDClient) Get(gvr schema.GroupVersionResource, namespace, name string, target interface{}) error {
	u, err := d.client.dynamic.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	data, err := u.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// List lists CRD resources and unmarshals into the target list.
func (d *CRDClient) List(gvr schema.GroupVersionResource, namespace string, target interface{}, opts ...metav1.ListOptions) error {
	listOpts := metav1.ListOptions{}
	if len(opts) > 0 {
		listOpts = opts[0]
	}
	u, err := d.client.dynamic.Resource(gvr).Namespace(namespace).List(context.TODO(), listOpts)
	if err != nil {
		return err
	}
	data, err := u.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Update creates or replaces a CRD resource (server-side apply semantics).
func (d *CRDClient) CreateOrReplace(gvr schema.GroupVersionResource, namespace string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	_, err = d.client.dynamic.Resource(gvr).Namespace(namespace).Update(context.TODO(), u, metav1.UpdateOptions{})
	if err == nil {
		return nil
	}
	if errors.IsNotFound(err) {
		_, err = d.client.dynamic.Resource(gvr).Namespace(namespace).Create(context.TODO(), u, metav1.CreateOptions{})
	}
	return err
}

// Delete removes a CRD resource.
func (d *CRDClient) Delete(gvr schema.GroupVersionResource, namespace, name string) error {
	return d.client.dynamic.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// GetRaw returns the unstructured object directly.
func (d *CRDClient) GetRaw(gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return d.client.dynamic.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// ListRaw returns the unstructured list directly.
func (d *CRDClient) ListRaw(gvr schema.GroupVersionResource, namespace string, opts ...metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	listOpts := metav1.ListOptions{}
	if len(opts) > 0 {
		listOpts = opts[0]
	}
	return d.client.dynamic.Resource(gvr).Namespace(namespace).List(context.TODO(), listOpts)
}
