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
limitations under the License.
*/

package agent

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// identity is extracted from the console session. NOTE: these routes
// deliberately do NOT honor the DISABLE_AUTH dev bypass that exists in other
// middleware (design §5.2): an agent endpoint that skips auth is a
// cluster-scoped liability.
type identity struct {
	UserName   string
	Namespaces []string
	Locale     string
}

func extractIdentity(c *gin.Context) (*identity, bool) {
	session := sessions.Default(c)
	name, _ := session.Get(auth.SessionKeyName).(string)
	if name == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "message": "login required"})
		return nil, false
	}
	namespaces := []string{"default"}
	if nsJSON, ok := session.Get(auth.SessionKeyUserNS).(string); ok && nsJSON != "" {
		var ns []string
		if err := json.Unmarshal([]byte(nsJSON), &ns); err == nil && len(ns) > 0 {
			namespaces = ns
		}
	}
	locale := c.GetHeader("Accept-Language")
	return &identity{UserName: name, Namespaces: namespaces, Locale: locale}, true
}

func failed(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": 40000, "message": msg})
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 10000, "data": data})
}

// RegisterRoutes mounts /api/v1/agent/* on the console engine. The session
// middleware is already engine-wide.
func RegisterRoutes(r *gin.Engine, m *Manager) {
	g := r.Group("/api/v1/agent")

	g.GET("/status", func(c *gin.Context) {
		if _, okID := extractIdentity(c); !okID {
			return
		}
		ok(c, m.Status())
	})

	g.POST("/sessions", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		var req struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			AgentType string `json:"agentType"`
			Title     string `json:"title"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, "invalid request: "+err.Error())
			return
		}
		if req.Namespace == "" && len(id.Namespaces) > 0 {
			req.Namespace = id.Namespaces[0]
		}
		// Sessions are always addressed inside the caller's own scope.
		if !containsStr(id.Namespaces, req.Namespace) {
			failed(c, "namespace is not in your allowed scope")
			return
		}
		s, err := m.CreateSession(id.UserName, id.Namespaces, req.Namespace, req.Name,
			AgentType(req.AgentType), req.Title, id.Locale)
		if err != nil {
			failed(c, err.Error())
			return
		}
		ok(c, gin.H{"namespace": s.Namespace, "name": s.Name})
	})

	g.GET("/sessions", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		ok(c, m.ListSessions(id.UserName))
	})

	g.POST("/sessions/:namespace/:name/messages", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		s, err := m.FindSession(id.UserName, c.Param("namespace"), c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 40400, "message": "session not found"})
			return
		}
		var req struct {
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, "content is required")
			return
		}
		run, err := m.StartMessage(s, req.Content)
		if err != nil {
			failed(c, err.Error())
			return
		}
		ok(c, gin.H{"runID": run.ID})
	})

	g.DELETE("/sessions/:namespace/:name", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		if err := m.DeleteSession(id.UserName, c.Param("namespace"), c.Param("name")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 40400, "message": "session not found"})
			return
		}
		ok(c, nil)
	})

	// Reconnectable event stream: clients may resume with ?after=<seq>.
	g.GET("/runs/:runID/events", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		run, err := m.GetRun(c.Param("runID"), id.UserName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 40400, "message": "run not found"})
			return
		}
		var after int64
		if q := c.Query("after"); q != "" {
			if v, err := strconv.ParseInt(q, 10, 64); err == nil && v >= 0 {
				after = v
			}
		}

		subID, ch, backlog := run.subscribe(after)
		defer run.unsubscribe(subID)

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")

		flusher, _ := c.Writer.(http.Flusher)
		writeEvent := func(ev Event) bool {
			b, err := json.Marshal(ev)
			if err != nil {
				return true
			}
			if _, err := c.Writer.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
				return false
			}
			if flusher != nil {
				flusher.Flush()
			}
			return true
		}

		for _, ev := range backlog {
			if !writeEvent(ev) {
				return // client left; the RUN continues server-side
			}
			if isTerminal(ev) {
				return
			}
		}
		for {
			select {
			case <-c.Request.Context().Done():
				return // client left; the RUN continues server-side
			case ev := <-ch:
				if !writeEvent(ev) {
					return
				}
				if isTerminal(ev) {
					return
				}
			}
		}
	})

	// HITL: clients submit ONLY {approved}; the executed arguments come from
	// the server-side snapshot (design §3.3).
	g.POST("/runs/:runID/confirmations/:cfmID", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		run, err := m.GetRun(c.Param("runID"), id.UserName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 40400, "message": "run not found"})
			return
		}
		var req struct {
			Approved bool `json:"approved"`
		}
		_ = c.ShouldBindJSON(&req)
		if err := run.ResolveConfirmation(c.Param("cfmID"), req.Approved); err != nil {
			failed(c, err.Error())
			return
		}
		ok(c, nil)
	})

	g.POST("/runs/:runID/stop", func(c *gin.Context) {
		id, okID := extractIdentity(c)
		if !okID {
			return
		}
		run, err := m.GetRun(c.Param("runID"), id.UserName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 40400, "message": "run not found"})
			return
		}
		run.mu.Lock()
		if run.cancel != nil {
			run.cancel()
		}
		run.mu.Unlock()
		ok(c, nil)
	})
}

func isTerminal(ev Event) bool {
	return ev.Type == EventDone
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
