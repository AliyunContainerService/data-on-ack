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
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	serverSocket string
	rpcTimeout   time.Duration
	dialTimeout  time.Duration
	logLevel     string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ack-commit-ctl",
	Short: "The command line client of ack-commit-agent.",
	Long:  `The command line client of ack-commit-agent.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if lvl, err := log.ParseLevel(logLevel); err == nil {
			log.SetLevel(lvl)
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra has already printed the error.
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverSocket, "server-socket", "/mnt/commit-agent/commit-agent.sock", "the commit-agent server socket address")
	rootCmd.PersistentFlags().DurationVar(&rpcTimeout, "timeout", 30*time.Minute, "deadline for the entire RPC call (image push can be lengthy)")
	rootCmd.PersistentFlags().DurationVar(&dialTimeout, "dial-timeout", 5*time.Second, "deadline for establishing the gRPC connection")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: panic, fatal, error, warn, info, debug, trace")
}

// dial returns a grpc.ClientConn against the agent's UDS, using a bounded
// dial timeout so a misconfigured socket fails quickly.
func dial(socket string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", addr)
	}

	//nolint:staticcheck // grpc.DialContext is still the canonical API for blocking dials.
	return grpc.DialContext(ctx, socket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
}
