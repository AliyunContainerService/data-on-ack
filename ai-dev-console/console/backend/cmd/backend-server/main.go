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
    
package main

import (
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/routers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/spf13/pflag"
	"k8s.io/klog"
)

func main() {
	defer func() {
		klog.Flush()
		if p := recover(); p != nil {
			debug.PrintStack()
		}
	}()
	c := make(chan os.Signal)
	f := func() chan os.Signal { return c }
	signal.Notify(f(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		klog.Infof("receive exit signal to delete webapp")
		if constants.IsCreateWebApp {
			auth.DeleteAppDefer()
		}
		time.Sleep(time.Second * 3)
		os.Exit(1)
	}()

	pflag.Parse()
	clientmgr.Init()
	client.Init()
	registry.RegisterStorageBackends()
	r := routers.InitRouter()
	if constants.IsCreateWebApp {
		klog.Infof("defer to delete webapp")
		defer auth.DeleteAppDefer()
		time.Sleep(time.Second * 3)
	}

	clientmgr.Start()

	_ = r.Run(":9090")
}
