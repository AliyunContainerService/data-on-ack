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
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"net"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "commit NAME[:TAG]",
	Short: "Create a new image from the notebook",
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		var opts []grpc.DialOption
		var dialer = func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", addr)
		}
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		opts = append(opts, grpc.WithContextDialer(dialer))

		conn, err := grpc.Dial(serverSocket, opts...)
		if err != nil {
			log.Errorf("did not connect: %v", err)
			return err
		}
		defer conn.Close()

		c := v1beta1.NewImageServiceClient(conn)

		cgroupMessage, err := utils.ReadSystemdLine(pkg.CgroupPath)
		if err != nil {
			log.Errorf("get container information failed: %v", err)
			return err
		}

		containerID := utils.GetContainerID(cgroupMessage)

		log.Infof(fmt.Sprintf("container id: %s", containerID))

		// get version
		client.CommitImage(c, &v1beta1.CommitRequest{
			Image:       args[0],
			ContainerID: containerID,
		})

		return nil
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
