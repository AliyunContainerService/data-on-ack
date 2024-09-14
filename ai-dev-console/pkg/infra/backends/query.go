/*
Copyright 2020 The Alibaba Authors.

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

package backends

import (
	"time"

	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
)

// Query contains a collection of options needed for querying a list of
// jobs persisted in database.
type Query struct {
	UID                 string
	UserName            string
	JobID               string
	Name                string
	Namespace           string
	Type                string
	RegionID            string
	ClusterID           string
	Status              v1.JobConditionType
	StartTime           time.Time
	EndTime             time.Time
	Deleted             *int
	Pagination          *QueryPagination
	AllocatedNamespaces []string
	IsCron              bool
}

// CronQuery contains a collection of options needed for querying a list of
// conrs persisted in database.
type CronQuery struct {
	UID                 string
	UserName            string
	CronID              string
	Name                string
	Namespace           string
	Type                string
	RegionID            string
	ClusterID           string
	Status              string
	StartTime           time.Time
	EndTime             time.Time
	Deleted             *int
	Pagination          *QueryPagination
	AllocatedNamespaces []string
}

type EvaluateJobQuery struct {
	StartTime           time.Time
	EndTime             time.Time
	Pagination          *QueryPagination
	AllocatedNamespaces []string
}

type ModelsQuery struct {
	Pagination   *QueryPagination
	ModelName    string
	ModelVersion string
}

type NotebookQuery struct {
	UID       string
	UserName  string
	Namespace string
	//Pagination          *QueryPagination
}

type QueryPagination struct {
	PageNum  int
	PageSize int
	Count    int
}
