/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/notebook-controller/api/v1beta1"
	"github.com/AliyunContainerService/data-on-ack/notebook-controller/pkg/culler"
	"github.com/AliyunContainerService/data-on-ack/notebook-controller/pkg/metrics"
	"github.com/go-logr/logr"
	reconcilehelper "github.com/kubeflow/kubeflow/components/common/reconcilehelper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const DefaultContainerPort = 8888
const DefaultServingPort = 80
const AnnotationRewriteURI = "notebooks.kubeflow.org/http-rewrite-uri"
const AnnotationHeadersRequestSet = "notebooks.kubeflow.org/http-headers-request-set"

const PrefixEnvVar = "NB_PREFIX"

const DefaultFSGroup = int64(100)

func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

type NotebookReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	Metrics       *metrics.Metrics
	EventRecorder record.EventRecorder
}

func (r *NotebookReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("notebook", req.NamespacedName)

	event := &corev1.Event{}
	var getEventErr error
	getEventErr = r.Get(ctx, req.NamespacedName, event)
	if getEventErr == nil {
		log.Info("Found event for Notebook. Re-emitting...")

		involvedNotebook := &v1beta1.Notebook{}
		nbName, err := nbNameFromInvolvedObject(r.Client, &event.InvolvedObject)
		if err != nil {
			return ctrl.Result{}, err
		}

		involvedNotebookKey := types.NamespacedName{Name: nbName, Namespace: req.Namespace}
		if err := r.Get(ctx, involvedNotebookKey, involvedNotebook); err != nil {
			log.Error(err, "unable to fetch Notebook by looking at event")
			return ctrl.Result{}, ignoreNotFound(err)
		}

		log.Info("Emitting Notebook Event.", "Event", event)
		r.EventRecorder.Eventf(involvedNotebook, event.Type, event.Reason,
			"Reissued from %s/%s: %s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.Message)
		return ctrl.Result{}, nil
	}

	if getEventErr != nil && !apierrs.IsNotFound(getEventErr) {
		return ctrl.Result{}, getEventErr
	}

	instance := &v1beta1.Notebook{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch Notebook")
		return ctrl.Result{}, ignoreNotFound(err)
	}

	ss := generateStatefulSet(instance)
	if err := ctrl.SetControllerReference(instance, ss, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	foundStateful := &appsv1.StatefulSet{}
	justCreated := false
	err := r.Get(ctx, types.NamespacedName{Name: ss.Name, Namespace: ss.Namespace}, foundStateful)
	if err != nil && apierrs.IsNotFound(err) {
		log.Info("Creating StatefulSet", "namespace", ss.Namespace, "name", ss.Name)
		r.Metrics.NotebookCreation.WithLabelValues(ss.Namespace).Inc()
		err = r.Create(ctx, ss)
		justCreated = true
		if err != nil {
			log.Error(err, "unable to create Statefulset")
			r.Metrics.NotebookFailCreation.WithLabelValues(ss.Namespace).Inc()
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "error getting Statefulset")
		return ctrl.Result{}, err
	}
	if !justCreated && reconcilehelper.CopyStatefulSetFields(ss, foundStateful) {
		log.Info("Updating StatefulSet", "namespace", ss.Namespace, "name", ss.Name)
		err = r.Update(ctx, foundStateful)
		if err != nil {
			log.Error(err, "unable to update Statefulset")
			return ctrl.Result{}, err
		}
	}

	service := generateService(instance)
	if err := ctrl.SetControllerReference(instance, service, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	foundService := &corev1.Service{}
	justCreated = false
	err = r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundService)
	if err != nil && apierrs.IsNotFound(err) {
		log.Info("Creating Service", "namespace", service.Namespace, "name", service.Name)
		err = r.Create(ctx, service)
		justCreated = true
		if err != nil {
			log.Error(err, "unable to create Service")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "error getting Statefulset")
		return ctrl.Result{}, err
	}
	if !justCreated && reconcilehelper.CopyServiceFields(service, foundService) {
		log.Info("Updating Service\n", "namespace", service.Namespace, "name", service.Name)
		err = r.Update(ctx, foundService)
		if err != nil {
			log.Error(err, "unable to update Service")
			return ctrl.Result{}, err
		}
	}

	if os.Getenv("USE_ISTIO") == "true" {
		err = r.reconcileVirtualService(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if foundStateful.Status.ReadyReplicas != instance.Status.ReadyReplicas {
		log.Info("Updating Status", "namespace", instance.Namespace, "name", instance.Name)
		instance.Status.ReadyReplicas = foundStateful.Status.ReadyReplicas
		err = r.Status().Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	pod := &corev1.Pod{}
	podFound := false
	err = r.Get(ctx, types.NamespacedName{Name: ss.Name + "-0", Namespace: ss.Namespace}, pod)
	if err != nil && apierrs.IsNotFound(err) {
		log.Info("Pod not found...")
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		podFound = true

		if len(pod.Status.ContainerStatuses) > 0 {
			notebookContainerFound := false
			for i := range pod.Status.ContainerStatuses {
				if pod.Status.ContainerStatuses[i].Name != instance.Name {
					continue
				}
				if pod.Status.ContainerStatuses[i].State == instance.Status.ContainerState {
					continue
				}

				log.Info("Updating Notebook CR state: ", "namespace", instance.Namespace, "name", instance.Name)
				cs := pod.Status.ContainerStatuses[i].State
				instance.Status.ContainerState = cs
				oldConditions := instance.Status.Conditions
				newCondition := getNextCondition(cs)
				if len(oldConditions) == 0 || oldConditions[0].Type != newCondition.Type ||
					oldConditions[0].Reason != newCondition.Reason ||
					oldConditions[0].Message != newCondition.Message {
					log.Info("Appending to conditions: ", "namespace", instance.Namespace, "name", instance.Name, "type", newCondition.Type, "reason", newCondition.Reason, "message", newCondition.Message)
					instance.Status.Conditions = append([]v1beta1.NotebookCondition{newCondition}, oldConditions...)
				}
				err = r.Status().Update(ctx, instance)
				if err != nil {
					return ctrl.Result{}, err
				}
				notebookContainerFound = true
				break

			}
			if !notebookContainerFound {
				log.Error(nil, "Could not find the Notebook container, will not update the status of the CR. No container has the same name as the CR.", "CR name:", instance.Name)
			}
		}
	}

	if !podFound {
		log.Info("Notebook has not Pod running. Will remove last-activity annotation")
		meta := instance.ObjectMeta
		if meta.GetAnnotations() == nil {
			log.Info("No annotations found")
			return ctrl.Result{}, nil
		}

		if _, ok := meta.GetAnnotations()[culler.LAST_ACTIVITY_ANNOTATION]; !ok {
			log.Info("No last-activity annotations found")
			return ctrl.Result{}, nil
		}

		log.Info("Removing last-activity annotation")
		delete(meta.GetAnnotations(), culler.LAST_ACTIVITY_ANNOTATION)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil

	}
	if culler.UpdateNotebookLastActivityAnnotation(&instance.ObjectMeta) {
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if culler.NotebookNeedsCulling(instance.ObjectMeta) {
		log.Info(fmt.Sprintf(
			"Notebook %s/%s needs culling. Setting annotations",
			instance.Namespace, instance.Name))

		culler.SetStopAnnotation(&instance.ObjectMeta, r.Metrics)
		r.Metrics.NotebookCullingCount.WithLabelValues(instance.Namespace, instance.Name).Inc()
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else if !culler.StopAnnotationIsSet(instance.ObjectMeta) {
		return ctrl.Result{RequeueAfter: culler.GetRequeueTime()}, nil
	}
	return ctrl.Result{RequeueAfter: culler.GetRequeueTime()}, nil
}

func getNextCondition(cs corev1.ContainerState) v1beta1.NotebookCondition {
	var nbtype = ""
	var nbreason = ""
	var nbmsg = ""

	if cs.Running != nil {
		nbtype = "Running"
	} else if cs.Waiting != nil {
		nbtype = "Waiting"
		nbreason = cs.Waiting.Reason
		nbmsg = cs.Waiting.Message
	} else {
		nbtype = "Terminated"
		nbreason = cs.Terminated.Reason
		nbmsg = cs.Terminated.Reason
	}

	newCondition := v1beta1.NotebookCondition{
		Type:          nbtype,
		LastProbeTime: metav1.Now(),
		Reason:        nbreason,
		Message:       nbmsg,
	}
	return newCondition
}

func setPrefixEnvVar(instance *v1beta1.Notebook, container *corev1.Container) {
	prefix := "/notebook/" + instance.Namespace + "/" + instance.Name

	for _, envVar := range container.Env {
		if envVar.Name == PrefixEnvVar {
			envVar.Value = prefix
			return
		}
	}

	container.Env = append(container.Env, corev1.EnvVar{
		Name:  PrefixEnvVar,
		Value: prefix,
	})
}

func generateStatefulSet(instance *v1beta1.Notebook) *appsv1.StatefulSet {
	replicas := int32(1)
	if culler.StopAnnotationIsSet(instance.ObjectMeta) {
		replicas = 0
	}

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Annotations: instance.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"statefulset": instance.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"statefulset":   instance.Name,
					"notebook-name": instance.Name,
				},
					Annotations: instance.Annotations,
				},
				Spec: *instance.Spec.Template.Spec.DeepCopy(),
			},
		},
	}
	l := &ss.Spec.Template.ObjectMeta.Labels
	for k, v := range instance.ObjectMeta.Labels {
		(*l)[k] = v
	}

	podSpec := &ss.Spec.Template.Spec
	container := &podSpec.Containers[0]
	if container.WorkingDir == "" {
		container.WorkingDir = "/home/jovyan"
	}
	if container.Ports == nil {
		container.Ports = []corev1.ContainerPort{
			{
				ContainerPort: DefaultContainerPort,
				Name:          "notebook-port",
				Protocol:      "TCP",
			},
		}
	}

	setPrefixEnvVar(instance, container)
	if value, exists := os.LookupEnv("ADD_FSGROUP"); !exists || value == "true" {
		if podSpec.SecurityContext == nil {
			fsGroup := DefaultFSGroup
			podSpec.SecurityContext = &corev1.PodSecurityContext{
				FSGroup: &fsGroup,
			}
		}
	}
	return ss
}

func generateService(instance *v1beta1.Notebook) *corev1.Service {
	port := DefaultContainerPort
	containerPorts := instance.Spec.Template.Spec.Containers[0].Ports
	if containerPorts != nil {
		port = int(containerPorts[0].ContainerPort)
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     "ClusterIP",
			Selector: map[string]string{"statefulset": instance.Name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http-" + instance.Name,
					Port:       DefaultServingPort,
					TargetPort: intstr.FromInt(port),
					Protocol:   "TCP",
				},
			},
		},
	}
	return svc
}

func virtualServiceName(kfName string, namespace string) string {
	return fmt.Sprintf("notebook-%s-%s", namespace, kfName)
}

func generateVirtualService(instance *v1beta1.Notebook) (*unstructured.Unstructured, error) {
	name := instance.Name
	namespace := instance.Namespace
	clusterDomain := "cluster.local"
	prefix := fmt.Sprintf("/notebook/%s/%s/", namespace, name)

	annotations := make(map[string]string)
	for k, v := range instance.ObjectMeta.Annotations {
		annotations[k] = v
	}

	rewrite := fmt.Sprintf("/notebook/%s/%s/", namespace, name)
	if _, ok := annotations[AnnotationRewriteURI]; ok && len(annotations[AnnotationRewriteURI]) > 0 {
		rewrite = annotations[AnnotationRewriteURI]
	}

	if clusterDomainFromEnv, ok := os.LookupEnv("CLUSTER_DOMAIN"); ok {
		clusterDomain = clusterDomainFromEnv
	}
	service := fmt.Sprintf("%s.%s.svc.%s", name, namespace, clusterDomain)

	vsvc := &unstructured.Unstructured{}
	vsvc.SetAPIVersion("networking.istio.io/v1alpha3")
	vsvc.SetKind("VirtualService")
	vsvc.SetName(virtualServiceName(name, namespace))
	vsvc.SetNamespace(namespace)
	if err := unstructured.SetNestedStringSlice(vsvc.Object, []string{"*"}, "spec", "hosts"); err != nil {
		return nil, fmt.Errorf("Set .spec.hosts error: %v", err)
	}

	istioGateway := os.Getenv("ISTIO_GATEWAY")
	if len(istioGateway) == 0 {
		istioGateway = "kubeflow/kubeflow-gateway"
	}
	if err := unstructured.SetNestedStringSlice(vsvc.Object, []string{istioGateway},
		"spec", "gateways"); err != nil {
		return nil, fmt.Errorf("Set .spec.gateways error: %v", err)
	}

	headersRequestSet := make(map[string]string)
	if _, ok := annotations[AnnotationHeadersRequestSet]; ok && len(annotations[AnnotationHeadersRequestSet]) > 0 {
		requestHeadersBytes := []byte(annotations[AnnotationHeadersRequestSet])
		if err := json.Unmarshal(requestHeadersBytes, &headersRequestSet); err != nil {
			headersRequestSet = make(map[string]string)
		}
	}
	headersRequestSetInterface := make(map[string]interface{})
	for key, element := range headersRequestSet {
		headersRequestSetInterface[key] = element
	}

	http := []interface{}{
		map[string]interface{}{
			"headers": map[string]interface{}{
				"request": map[string]interface{}{
					"set": headersRequestSetInterface,
				},
			},
			"match": []interface{}{
				map[string]interface{}{
					"uri": map[string]interface{}{
						"prefix": prefix,
					},
				},
			},
			"rewrite": map[string]interface{}{
				"uri": rewrite,
			},
			"route": []interface{}{
				map[string]interface{}{
					"destination": map[string]interface{}{
						"host": service,
						"port": map[string]interface{}{
							"number": int64(DefaultServingPort),
						},
					},
				},
			},
		},
	}

	if err := unstructured.SetNestedSlice(vsvc.Object, http, "spec", "http"); err != nil {
		return nil, fmt.Errorf("Set .spec.http error: %v", err)
	}

	return vsvc, nil

}

func (r *NotebookReconciler) reconcileVirtualService(instance *v1beta1.Notebook) error {
	log := r.Log.WithValues("notebook", instance.Namespace)
	virtualService, err := generateVirtualService(instance)
	if err := ctrl.SetControllerReference(instance, virtualService, r.Scheme); err != nil {
		return err
	}
	foundVirtual := &unstructured.Unstructured{}
	justCreated := false
	foundVirtual.SetAPIVersion("networking.istio.io/v1alpha3")
	foundVirtual.SetKind("VirtualService")
	err = r.Get(context.TODO(), types.NamespacedName{Name: virtualServiceName(instance.Name,
		instance.Namespace), Namespace: instance.Namespace}, foundVirtual)
	if err != nil && apierrs.IsNotFound(err) {
		log.Info("Creating virtual service", "namespace", instance.Namespace, "name",
			virtualServiceName(instance.Name, instance.Namespace))
		err = r.Create(context.TODO(), virtualService)
		justCreated = true
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if !justCreated && reconcilehelper.CopyVirtualService(virtualService, foundVirtual) {
		log.Info("Updating virtual service", "namespace", instance.Namespace, "name",
			virtualServiceName(instance.Name, instance.Namespace))
		err = r.Update(context.TODO(), foundVirtual)
		if err != nil {
			return err
		}
	}

	return nil
}

func isStsOrPodEvent(event *corev1.Event) bool {
	return event.InvolvedObject.Kind == "Pod" || event.InvolvedObject.Kind == "StatefulSet"
}

func nbNameFromInvolvedObject(c client.Client, object *corev1.ObjectReference) (string, error) {
	name, namespace := object.Name, object.Namespace

	if object.Kind == "StatefulSet" {
		return name, nil
	}
	if object.Kind == "Pod" {
		pod := &corev1.Pod{}
		err := c.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
			pod,
		)
		if err != nil {
			return "", err
		}
		if nbName, ok := pod.Labels["notebook-name"]; ok {
			return nbName, nil
		}
	}
	return "", fmt.Errorf("object isn't related to a Notebook")
}

func nbNameExists(client client.Client, nbName string, namespace string) bool {
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: nbName}, &v1beta1.Notebook{}); err != nil {
		return !apierrs.IsNotFound(err)
	}
	return true
}

func (r *NotebookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Notebook{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{})
	// watch Istio virtual service
	if os.Getenv("USE_ISTIO") == "true" {
		virtualService := &unstructured.Unstructured{}
		virtualService.SetAPIVersion("networking.istio.io/v1alpha3")
		virtualService.SetKind("VirtualService")
		builder.Owns(virtualService)
	}

	c, err := builder.Build(r)
	if err != nil {
		return err
	}

	mapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []ctrl.Request {
			return []ctrl.Request{
				{NamespacedName: types.NamespacedName{
					Name:      a.Meta.GetLabels()["notebook-name"],
					Namespace: a.Meta.GetNamespace(),
				}},
			}
		})

	checkNBLabel := func(m metav1.Object) bool {
		_, ok := m.GetLabels()["notebook-name"]
		return ok
	}
	// TODO: refactor to use predicate.NewPredicateFuncs when controller-runtime module version is updated
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return checkNBLabel(e.MetaOld) && e.ObjectOld != e.ObjectNew
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return checkNBLabel(e.Meta)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return checkNBLabel(e.Meta)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return checkNBLabel(e.Meta)
		},
	}

	eventToRequest := handler.ToRequestsFunc(
		func(a handler.MapObject) []ctrl.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      a.Meta.GetName(),
					Namespace: a.Meta.GetNamespace(),
				}},
			}
		})

	checkEvent := func(o runtime.Object, m metav1.Object) bool {
		event := o.(*corev1.Event)
		nbName, err := nbNameFromInvolvedObject(r.Client, &event.InvolvedObject)
		if err != nil {
			return false
		}
		return isStsOrPodEvent(event) && nbNameExists(r.Client, nbName, m.GetNamespace())
	}
	eventsPredicates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld != e.ObjectNew && checkEvent(e.ObjectNew, e.MetaNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return checkEvent(e.Object, e.Meta)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return checkEvent(e.Object, e.Meta)
		},
	}

	if err = c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: mapFn,
		},
		p); err != nil {
		return err
	}

	if err = c.Watch(
		&source.Kind{Type: &corev1.Event{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: eventToRequest,
		},
		eventsPredicates); err != nil {
		return err
	}

	return nil
}
