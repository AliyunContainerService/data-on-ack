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

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

// ClientVersion is the build-stamped CLI version, set via -ldflags from the
// Makefile. Matches the value the agent reports for its `Version` RPC.
var ClientVersion = "dev"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print client and remote agent version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("client: %s\n", ClientVersion)

		conn, err := dial(serverSocket)
		if err != nil {
			return fmt.Errorf("dial commit-agent at %s: %w", serverSocket, err)
		}
		defer conn.Close()

		c := v1beta1.NewImageServiceClient(conn)
		ctx, cancel := context.WithTimeout(cmd.Context(), rpcTimeout)
		defer cancel()

		ver, err := client.GetVersion(ctx, c, &v1beta1.VersionRequest{})
		if err != nil {
			return err
		}
		fmt.Printf("agent:  %s\n", ver)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
