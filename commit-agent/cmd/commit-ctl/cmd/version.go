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
	"google.golang.org/grpc"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		var opts []grpc.DialOption
		var dialer = func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", addr)
		}
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		opts = append(opts, grpc.WithContextDialer(dialer))

		conn, err := grpc.Dial(serverSocket, opts...)
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		c := v1beta1.NewImageServiceClient(conn)

		// get version
		client.GetVersion(c, &v1beta1.VersionRequest{})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
