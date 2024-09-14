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
    
package utils

import (
	"github.com/gin-gonic/gin"
	"k8s.io/klog"
	"net/http"
)

func RedirectTo(c *gin.Context, redirectTarget string) {
	klog.Infof("index redirect to %v uri:[%v]", redirectTarget, c.Request.URL)
	c.Redirect(http.StatusFound, redirectTarget)
}

func Redirect403(c *gin.Context) {
	c.Redirect(http.StatusFound, "/403")
}

func Redirect404(c *gin.Context) {
	c.Redirect(http.StatusFound, "/404")
}

func Redirect500(c *gin.Context) {
	c.Redirect(http.StatusFound, "/500")
}

func Redirect1000(c *gin.Context) {
	c.Redirect(http.StatusFound, "/1000")
}
