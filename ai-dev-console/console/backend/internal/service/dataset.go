package service

import (
	"context"
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatasetService manages PVC-based datasets and Fluid cache detection.
type DatasetService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
}

func NewDatasetService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *DatasetService {
	return &DatasetService{
		adminClient: adminClient,
		tenants:     tenants,
	}
}

// List returns PVCs across user namespaces, with Fluid detection.
func (s *DatasetService) List(namespaces []string) ([]model.DatasetInfo, error) {
	var results []model.DatasetInfo

	for _, ns := range namespaces {
		pvcs, err := s.adminClient.Typed().CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, pvc := range pvcs.Items {
			info := model.DatasetInfo{
				Name:         pvc.Name,
				Namespace:    pvc.Namespace,
				Status:       string(pvc.Status.Phase),
				StorageClass: getStorageClass(pvc),
				Capacity:     getCapacity(pvc),
				AccessModes:  getAccessModes(pvc),
				FluidCached:  isFluidCached(pvc),
			}
			if !pvc.CreationTimestamp.IsZero() {
				info.Age = formatDuration(pvc.CreationTimestamp.Time.Sub(pvc.CreationTimestamp.Time))
			}
			results = append(results, info)
		}
	}

	return results, nil
}

// Create creates a new PVC.
func (s *DatasetService) Create(spec *model.DatasetCreateSpec, userName string) error {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return fmt.Errorf("get tenant client: %w", err)
	}

	accessMode := corev1.ReadWriteOnce
	switch spec.AccessMode {
	case "ReadWriteMany":
		accessMode = corev1.ReadWriteMany
	case "ReadOnlyMany":
		accessMode = corev1.ReadOnlyMany
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ai-dev-console",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(spec.Capacity),
				},
			},
		},
	}

	if spec.StorageClass != "" {
		pvc.Spec.StorageClassName = &spec.StorageClass
	}

	_, err = tc.Typed.CoreV1().PersistentVolumeClaims(spec.Namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	return err
}

// Delete removes a PVC.
func (s *DatasetService) Delete(name, namespace, userName string) error {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return fmt.Errorf("get tenant client: %w", err)
	}
	return tc.Typed.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// --- Helpers ---

func getStorageClass(pvc corev1.PersistentVolumeClaim) string {
	if pvc.Spec.StorageClassName != nil {
		return *pvc.Spec.StorageClassName
	}
	return ""
}

func getCapacity(pvc corev1.PersistentVolumeClaim) string {
	if cap, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		return cap.String()
	}
	// Fallback to spec requests
	if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		return req.String()
	}
	return ""
}

func getAccessModes(pvc corev1.PersistentVolumeClaim) []string {
	var modes []string
	for _, m := range pvc.Spec.AccessModes {
		modes = append(modes, string(m))
	}
	return modes
}

// isFluidCached detects if a PVC is backed by Fluid (JindoFS/Alluxio).
func isFluidCached(pvc corev1.PersistentVolumeClaim) bool {
	// Fluid creates PVCs with specific labels/annotations
	labels := pvc.GetLabels()
	if labels != nil {
		if _, ok := labels["fluid.io/dataset"]; ok {
			return true
		}
		if _, ok := labels["fluid.io/managed-by"]; ok {
			return true
		}
	}
	// Also check StorageClass name patterns
	sc := getStorageClass(pvc)
	return sc == "fluid" || sc == "jindo" || sc == "alluxio"
}
