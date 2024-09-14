/*
Copyright 2021 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"fmt"
	"reflect"
	"sync"
)

type SyncMapCache struct {
	cache     *sync.Map
	valueType reflect.Type
}

func NewSyncMapCache(valueType reflect.Type) *SyncMapCache {
	return &SyncMapCache{
		cache:     new(sync.Map),
		valueType: valueType,
	}
}

func (s *SyncMapCache) validateValueType(value interface{}) bool {
	return reflect.TypeOf(value).AssignableTo(s.valueType)
}

func (s *SyncMapCache) Get(key string) (interface{}, bool) {
	return s.cache.Load(key)
}

func (s *SyncMapCache) Store(key string, value interface{}) error {
	if !s.validateValueType(value) {
		return fmt.Errorf("validation error, value type should be %T", s.valueType)
	}
	s.cache.Store(key, value)
	return nil
}

func (s *SyncMapCache) Delete(key string) {
	s.cache.Delete(key)
}
