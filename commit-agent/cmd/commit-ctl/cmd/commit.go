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

package cmd

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "commit NAME[:TAG]",
	Short: "Create a new image from the notebook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		image := strings.TrimSpace(args[0])
		if image == "" {
			return fmt.Errorf("image name must not be empty")
		}

		conn, err := dial(serverSocket)
		if err != nil {
			return fmt.Errorf("dial commit-agent at %s: %w", serverSocket, err)
		}
		defer conn.Close()

		c := v1beta1.NewImageServiceClient(conn)

		cgroupLine, err := utils.ReadCgroupLine(pkg.CgroupPath)
		if err != nil {
			return fmt.Errorf("read cgroup info: %w", err)
		}
		containerID := utils.GetContainerID(cgroupLine)
		if containerID == "" {
			return fmt.Errorf("could not extract container ID from %q", cgroupLine)
		}
		log.Infof("container id: %s", containerID)

		ctx, cancel := context.WithTimeout(cmd.Context(), rpcTimeout)
		defer cancel()

		return client.CommitImage(ctx, c, &v1beta1.CommitRequest{
			Image:       image,
			ContainerID: containerID,
		})
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
