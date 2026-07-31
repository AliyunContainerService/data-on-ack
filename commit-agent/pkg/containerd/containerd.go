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

package containerd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/reference"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	dockerconfig "github.com/containerd/containerd/remotes/docker/config"
	"github.com/containerd/nerdctl/pkg/api/types"
	"github.com/containerd/nerdctl/pkg/errutil"
	"github.com/containerd/nerdctl/pkg/idutil/containerwalker"
	"github.com/containerd/nerdctl/pkg/imgutil/commit"
	"github.com/containerd/nerdctl/pkg/imgutil/dockerconfigresolver"
	"github.com/containerd/nerdctl/pkg/imgutil/push"
	"github.com/containerd/nerdctl/pkg/ipfs"
	"github.com/containerd/nerdctl/pkg/platformutil"
	"github.com/containerd/nerdctl/pkg/referenceutil"
	"github.com/containerd/nerdctl/pkg/signutil"
	log "github.com/sirupsen/logrus"

	_type "github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/type"
)

const defaultContainerdSock = "/host/run/containerd/containerd.sock"

// Options configures the containerd client.
type Options struct {
	// Address is the path to the containerd UDS. Empty = default.
	Address string
	// Namespace is the containerd namespace, e.g. "k8s.io".
	Namespace string
}

// Client wraps a containerd.Client and remembers which namespace to use.
type Client struct {
	cli       *containerd.Client
	namespace string
}

// NewContainerdClient establishes a connection to containerd. The error is
// returned to the caller rather than panicking.
func NewContainerdClient(opts Options) (_type.ContainerClient, error) {
	addr := opts.Address
	if addr == "" {
		addr = defaultContainerdSock
	}
	ns := opts.Namespace
	if ns == "" {
		ns = "k8s.io"
	}

	cli, err := containerd.New(addr)
	if err != nil {
		return nil, fmt.Errorf("dial containerd at %s: %w", addr, err)
	}
	return &Client{cli: cli, namespace: ns}, nil
}

// CommitImageFromSelf snapshots the named container.
func (c *Client) CommitImageFromSelf(ctx context.Context, containerID, image string) error {
	named, err := referenceutil.ParseDockerRef(image)
	if err != nil {
		return fmt.Errorf("parse image %q: %w", image, err)
	}

	opts := &commit.Opts{
		Pause:   false,
		Ref:     named.String(),
		Author:  "",
		Message: "",
		Changes: commit.Changes{},
	}

	walker := &containerwalker.ContainerWalker{
		Client: c.cli,
		OnFound: func(ctx context.Context, found containerwalker.Found) error {
			if found.MatchCount > 1 {
				return fmt.Errorf("ambiguous container ID %q", found.Req)
			}
			_, err := commit.Commit(ctx, c.cli, found.Container, opts)
			return err
		},
	}

	ctx = namespaces.WithNamespace(ctx, c.namespace)
	n, err := walker.Walk(ctx, containerID)
	if err != nil {
		return fmt.Errorf("containerd commit: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no such container %s in namespace %s", containerID, c.namespace)
	}
	return nil
}

// PushImageFromSelf pushes via nerdctl's push helper, with insecure-fallback
// driven by the registry's response.
func (c *Client) PushImageFromSelf(ctx context.Context, rawRef, username, password string) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	return Push(ctx, c.cli, rawRef, username, password, types.ImagePushOptions{
		Stdout:   io.Discard,
		GOptions: types.GlobalCommandOptions{Debug: log.GetLevel() >= log.DebugLevel},
	})
}

// Push uploads `rawRef` to its remote registry. It mirrors nerdctl's push
// implementation but is parameterised by the credentials supplied via gRPC.
func Push(ctx context.Context, client *containerd.Client, rawRef, username, password string, options types.ImagePushOptions) error {
	if scheme, ref, err := referenceutil.ParseIPFSRefWithScheme(rawRef); err == nil {
		if scheme != "ipfs" {
			return fmt.Errorf("ipfs scheme is only supported but got %q", scheme)
		}
		log.Infof("pushing image %q to IPFS", ref)

		var ipfsPath string
		if options.IpfsAddress != "" {
			dir, err := os.MkdirTemp("", "apidirtmp")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			if err := os.WriteFile(filepath.Join(dir, "api"), []byte(options.IpfsAddress), 0600); err != nil {
				return err
			}
			ipfsPath = dir
		}

		var layerConvert converter.ConvertFunc
		c, err := ipfs.Push(ctx, client, ref, layerConvert, options.AllPlatforms, options.Platforms, options.IpfsEnsureImage, ipfsPath)
		if err != nil {
			log.WithError(err).Warnf("ipfs push failed")
			return err
		}
		fmt.Fprintln(options.Stdout, c)
		return nil
	}

	named, err := refdocker.ParseDockerRef(rawRef)
	if err != nil {
		return err
	}
	ref := named.String()
	refDomain := refdocker.Domain(named)

	platMC, err := platformutil.NewMatchComparer(options.AllPlatforms, options.Platforms)
	if err != nil {
		return err
	}
	pushRef := ref
	if !options.AllPlatforms {
		pushRef = ref + "-tmp-reduced-platform"
		platImg, err := converter.Convert(ctx, client, pushRef, ref, converter.WithPlatform(platMC))
		if err != nil {
			if len(options.Platforms) == 0 {
				return fmt.Errorf("failed to create a tmp single-platform image %q: %w", pushRef, err)
			}
			return fmt.Errorf("failed to create a tmp reduced-platform image %q (platform=%v): %w", pushRef, options.Platforms, err)
		}
		defer client.ImageService().Delete(ctx, platImg.Name, images.SynchronousDelete())
		log.Infof("pushing as a reduced-platform image (%s, %s)", platImg.Target.MediaType, platImg.Target.Digest)
	}

	pushTracker := docker.NewInMemoryTracker()

	pushFunc := func(r remotes.Resolver) error {
		return push.Push(ctx, client, r, pushTracker, options.Stdout, pushRef, ref, platMC, options.AllowNondistributableArtifacts, options.Quiet)
	}

	var dOpts []dockerconfigresolver.Opt
	if options.GOptions.InsecureRegistry {
		log.Warnf("skipping verifying HTTPS certs for %q", refDomain)
		dOpts = append(dOpts, dockerconfigresolver.WithSkipVerifyCerts(true))
	}
	dOpts = append(dOpts, dockerconfigresolver.WithHostsDirs(options.GOptions.HostsDir))

	authCreds := func(acArg string) (string, string, error) {
		if acArg == refDomain {
			return username, password, nil
		}
		return "", "", fmt.Errorf("expected acArg to be %q, got %q", refDomain, acArg)
	}

	dOpts = append(dOpts, dockerconfigresolver.WithAuthCreds(authCreds))
	ho, err := dockerconfigresolver.NewHostOptions(ctx, refDomain, dOpts...)
	if err != nil {
		return err
	}

	resolverOpts := docker.ResolverOptions{
		Tracker: pushTracker,
		Hosts:   dockerconfig.ConfigureHosts(ctx, *ho),
	}

	resolver := docker.NewResolver(resolverOpts)
	if err = pushFunc(resolver); err != nil {
		if !errutil.IsErrHTTPResponseToHTTPSClient(err) && !errutil.IsErrConnectionRefused(err) {
			return err
		}
		if options.GOptions.InsecureRegistry {
			log.WithError(err).Warnf("server %q does not seem to support HTTPS, falling back to plain HTTP", refDomain)
			dOpts = append(dOpts, dockerconfigresolver.WithPlainHTTP(true))
			resolver, err = dockerconfigresolver.New(ctx, refDomain, dOpts...)
			if err != nil {
				return err
			}
			return pushFunc(resolver)
		}
		log.WithError(err).Errorf("server %q does not seem to support HTTPS", refDomain)
		log.Info("Hint: you may want to try --insecure-registry to allow plain HTTP (if you are in a trusted network)")
		return err
	}

	img, err := client.ImageService().Get(ctx, pushRef)
	if err != nil {
		return err
	}
	refSpec, err := reference.Parse(pushRef)
	if err != nil {
		return err
	}
	signRef := fmt.Sprintf("%s@%s", refSpec.String(), img.Target.Digest.String())
	if err = signutil.Sign(signRef,
		options.GOptions.Experimental,
		options.SignOptions); err != nil {
		return err
	}
	if options.Quiet {
		fmt.Fprintln(options.Stdout, ref)
	}
	return nil
}
