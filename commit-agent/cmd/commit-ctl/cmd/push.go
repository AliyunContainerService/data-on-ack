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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

const (
	envUsername = "ACR_USERNAME"
	envPassword = "ACR_PASSWORD"
)

var (
	username      string
	password      string
	passwordStdin bool
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push NAME[:TAG]",
	Short: "Push an image or a repository to a registry.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		image := strings.TrimSpace(args[0])
		if image == "" {
			return fmt.Errorf("image name must not be empty")
		}

		if passwordStdin {
			if password != "" {
				return fmt.Errorf("--password and --password-stdin are mutually exclusive")
			}
			pw, err := readPasswordStdin(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read password from stdin: %w", err)
			}
			password = pw
		}

		// Allow credentials via env to avoid putting passwords in shell history.
		if username == "" {
			username = os.Getenv(envUsername)
		}
		if password == "" {
			password = os.Getenv(envPassword)
		}

		conn, err := dial(serverSocket)
		if err != nil {
			return fmt.Errorf("dial commit-agent at %s: %w", serverSocket, err)
		}
		defer conn.Close()

		c := v1beta1.NewImageServiceClient(conn)

		ctx, cancel := context.WithTimeout(cmd.Context(), rpcTimeout)
		defer cancel()

		return client.PushImage(ctx, c, &v1beta1.PushRequest{
			Image:    image,
			Username: username,
			Password: password,
		})
	},
}

// readPasswordStdin slurps the password from the reader, stripping the
// trailing newline if present. Empty passwords are rejected so the caller
// can't silently authenticate as anonymous.
func readPasswordStdin(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	pw, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	pw = strings.TrimRight(pw, "\r\n")
	if pw == "" {
		return "", fmt.Errorf("password from stdin is empty")
	}
	return pw, nil
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringVar(&username, "username", "", "registry username (defaults to $ACR_USERNAME)")
	pushCmd.Flags().StringVar(&password, "password", "", "registry password (defaults to $ACR_PASSWORD); avoid in argv, prefer --password-stdin")
	pushCmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read registry password from stdin")
}
