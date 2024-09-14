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
    
package api

import (
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/gin-contrib/sessions"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

func NewCodeSourceAPIsController(codeSourceHandler *handlers.CodeSourceHandler) *CodeSourceAPIsController {
	return &CodeSourceAPIsController{
		codeSourceHandler: codeSourceHandler,
	}
}

type CodeSourceAPIsController struct {
	codeSourceHandler *handlers.CodeSourceHandler
}

func (dc *CodeSourceAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	overview := routes.Group("/codesource")
	overview.POST("", dc.postCodeSource)
	overview.DELETE("/:name", dc.deleteCodeSource)
	overview.PUT("", dc.putCodeSource)
	overview.GET("", dc.getCodeSource)
	overview.GET("/:name", dc.getCodeSource)
}

func (dc *CodeSourceAPIsController) postCodeSource(c *gin.Context) {
	userId := c.PostForm("userid")
	username := c.PostForm("username")
	name := c.PostForm("name")
	_type := c.PostForm("type")
	codePath := c.PostForm("code_path")
	defaultBranch := c.PostForm("default_branch")
	localPath := c.PostForm("local_path")
	description := c.PostForm("description")
	gitUsername := c.PostForm("gitUsername")
	gitPassword := c.PostForm("gitPassword")
	createTime := time.Now().Format("2006-01-02 15:04:05")
	updateTime := time.Now().Format("2006-01-02 15:04:05")

	codesource := model.CodeSource{
		UserId:        userId,
		Username:      username,
		Name:          name,
		Type:          _type,
		CodePath:      codePath,
		DefaultBranch: defaultBranch,
		LocalPath:     localPath,
		Description:   description,
		CreateTime:    createTime,
		UpdateTime:    updateTime,
	}

	if gitUsername != "" && gitPassword != "" {
		codesource.GitUsername = gitUsername
		codesource.GitPassword = gitPassword
	}

	err := dc.codeSourceHandler.PostCodeSourceToConfigMap(username, codesource)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to create code, error: %v", err))
		return
	}
	utils.Succeed(c, "success to create code")
}

func (dc *CodeSourceAPIsController) deleteCodeSource(c *gin.Context) {
	name := c.Param("name")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	err := dc.codeSourceHandler.DeleteCodeSourceFromConfigMap(loginUserName, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to delete code, error: %v", err))
		return
	}
	utils.Succeed(c, "success to delete code.")
}

func (dc *CodeSourceAPIsController) putCodeSource(c *gin.Context) {
	userId := c.PostForm("userid")
	username := c.PostForm("username")
	name := c.PostForm("name")
	_type := c.PostForm("type")
	codePath := c.PostForm("code_path")
	defaultBranch := c.PostForm("default_branch")
	localPath := c.PostForm("local_path")
	description := c.PostForm("description")
	updateTime := time.Now().Format("2006-01-02 15:04:05")

	codesource := model.CodeSource{
		UserId:        userId,
		Username:      username,
		Name:          name,
		Type:          _type,
		CodePath:      codePath,
		DefaultBranch: defaultBranch,
		LocalPath:     localPath,
		Description:   description,
		UpdateTime:    updateTime,
	}

	err := dc.codeSourceHandler.PutCodeSourceToConfigMap(username, codesource)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to update code, error: %v", err))
		return
	}
	utils.Succeed(c, "success to update code")
}

func (dc *CodeSourceAPIsController) getCodeSource(c *gin.Context) {
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	name := c.Param("name")
	if name == "" {
		codeSources, err := dc.codeSourceHandler.ListCodeSourceFromConfigMap(loginUserName)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to get code, error: %v", err))
			return
		}

		for k, v := range codeSources {
			if v.Username != loginUserName {
				delete(codeSources, k)
			}
		}

		utils.Succeed(c, codeSources)
	} else {
		codeSources, err := dc.codeSourceHandler.GetCodeSourceFromConfigMap(loginUserName, name)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to get code, error: %v", err))
			return
		}
		utils.Succeed(c, codeSources)
	}

}
