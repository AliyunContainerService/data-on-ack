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
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func Succeed(c *gin.Context, obj interface{}) {
	c.JSONP(http.StatusOK, gin.H{
		"code": "200",
		"data": obj,
	})
}

func Failed(c *gin.Context, msg string) {
	c.JSONP(http.StatusOK, gin.H{
		"code": "300",
		"data": msg,
	})
	c.Set("failed", true)
}

// Param gets space deduplicated param from URL.
func Param(c *gin.Context, param string) string {
	return strings.TrimSpace(c.Param(param))
}

// Query gets space deduplicated query from URL.
func Query(c *gin.Context, param string) string {
	return strings.TrimSpace(c.Query(param))
}

// TimeTransform transforms from-time and to-time from string to a Time instance
// formatted in RFC3339. Considering time differences between log-timestamp and
// job-timestamp, we shift 1h earlier for from-time and postpone 1h for to-time.
func TimeTransform(from, to string) (fromTime, toTime time.Time, err error) {
	if strings.TrimSpace(from) != "" {
		tmpTime, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return fromTime, toTime, err
		}
		duration, _ := time.ParseDuration("-1h")
		fromTime = tmpTime.Add(duration)
	}

	if strings.TrimSpace(to) != "" {
		tmpTime, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return fromTime, toTime, err
		}
		duration, _ := time.ParseDuration("1h")
		toTime = tmpTime.Add(duration)
	}
	return
}
