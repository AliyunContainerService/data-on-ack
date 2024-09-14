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

package mysql

import (
	"fmt"
	"strconv"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
)

// Constants down below defines configurations to initialize a mysql backend
// storage service, user should set sls environment variables in Dockerfile or
// deployment manifests files, for security user should better init environment
// variables by referencing Secret key as down below:
// spec:
//
//	containers:
//	- name: xxx-container
//	  image: xxx
//	  env:
//	    - name: MYSQL_HOST
//	      valueFrom:
//	        secretKeyRef:
//	          name: my-mysql-secret
//	          key: host
const (
	EnvDBHost     = "MYSQL_HOST"
	EnvDBPort     = "MYSQL_PORT"
	EnvDBDatabase = "MYSQL_DB_NAME"
	EnvDBUser     = "MYSQL_USER"
	EnvDBPassword = "MYSQL_PASSWORD"
	EnvLogMode    = "MYSQL_LOGMODE"
)

func GetMysqlDBSource() (dbSource, logMode string, err error) {
	host := utils.GetEnvOrDefault(EnvDBHost, "ack-mysql.kube-ai.svc.cluster.local")
	port, err := strconv.Atoi(utils.GetEnvOrDefault(EnvDBPort, "3306"))
	if err != nil {
		return "", "", err
	}
	db := utils.GetEnvOrDefault(EnvDBDatabase, "kubeai")
	user := utils.GetEnvOrDefault(EnvDBUser, "kubeai")
	password := utils.GetEnvOrDefault(EnvDBPassword, "kubeai@ACK")

	dbSource = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=true", user, password, host, port, db)
	//klog.Infof("mysql datasource: %s", dbSource)
	logMode = utils.GetEnvOrDefault(EnvLogMode, "error")
	return dbSource, logMode, nil
}
