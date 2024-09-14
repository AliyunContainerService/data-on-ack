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

package evaluate

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &enqueueForEvaluate{}

type enqueueForEvaluate struct{}

func (e *enqueueForEvaluate) Create(evt event.CreateEvent, queue workqueue.RateLimitingInterface) {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.Meta.GetNamespace(),
		Name:      util.IDName(evt.Meta),
	}})
}

func (e *enqueueForEvaluate) Update(evt event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.MetaOld.GetNamespace(),
		Name:      util.IDName(evt.MetaOld),
	}})

	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.MetaNew.GetNamespace(),
		Name:      util.IDName(evt.MetaNew),
	}})
}

func (e *enqueueForEvaluate) Delete(evt event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: evt.Meta.GetNamespace(),
		Name:      util.IDName(evt.Meta),
	}})
}

func (e *enqueueForEvaluate) Generic(evt event.GenericEvent, queue workqueue.RateLimitingInterface) {
	e.Create(event.CreateEvent{Meta: evt.Meta, Object: evt.Object}, queue)
}
