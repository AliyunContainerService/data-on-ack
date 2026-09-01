/*
Copyright 2021 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package code_sync

import (
	"encoding/json"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
)

const (
	defaultGitSyncImage = "kubedl/git-sync:v1"
)

// Environment variable names understood by the git-sync toolkit. They are
// centralized here because they form the wire contract with the git-sync
// container and are referenced from multiple submission paths (init-container
// injection below and arena submission builders).
//
// SECURITY: GitSyncUsernameEnv/GitSyncPasswordEnv carry user git credentials
// as PLAINTEXT Pod environment variables; anyone able to read Pod specs in
// the tenant namespace (e.g. `kubectl get pod -o yaml`) can obtain them.
// The env-injection protocol is kept for backward compatibility. Mitigation
// plan: inject credentials via Kubernetes Secrets instead (secretKeyRef env
// sources or a mounted GIT_SYNC_CREDENTIAL file) so that no literal secret
// value ever appears in the Pod spec, then remove the plaintext path.
const (
	GitSyncRepoEnv        = "GIT_SYNC_REPO"
	GitSyncOneTimeEnv     = "GIT_SYNC_ONE_TIME"
	GitSyncMaxFailuresEnv = "GIT_SYNC_MAX_SYNC_FAILURES"
	GitSyncBranchEnv      = "GIT_SYNC_BRANCH"
	GitSyncRevisionEnv    = "GIT_SYNC_REV"
	GitSyncDepthEnv       = "GIT_SYNC_DEPTH"
	GitSyncRootEnv        = "GIT_SYNC_ROOT"
	GitSyncDestEnv        = "GIT_SYNC_DEST"
	GitSyncSSHEnv         = "GIT_SYNC_SSH"
	GitSSHKeyFileEnv      = "GIT_SSH_KEY_FILE"
	GitSyncUsernameEnv    = "GIT_SYNC_USERNAME"
	GitSyncPasswordEnv    = "GIT_SYNC_PASSWORD"
)

var _ CodeSyncHandler = &gitSyncHandler{}

type GitSyncOptions struct {
	SyncOptions `json:",inline"`

	// All fields down below are optional.

	// Git repository settings for user to specify.
	Branch   string `json:"branch,omitempty"`
	Revision string `json:"revision,omitempty"`
	Depth    string `json:"depth,omitempty"`
	// Max consecutive failures allowed.
	MaxFailures int `json:"maxFailures,omitempty"`
	// SSH settings for users to use git in ssh pattern.
	SSH     bool   `json:"ssh,omitempty"`
	SSHFile string `json:"sshFile,omitempty"`
	// User-customized account settings.
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type gitSyncHandler struct{}

func (h *gitSyncHandler) InitContainer(optsConfig []byte, mountVolume *v1.Volume) (*v1.Container, string, string, error) {
	opts := GitSyncOptions{}
	if err := json.Unmarshal(optsConfig, &opts); err != nil {
		return nil, "", "", err
	}
	setDefaultSyncOpts(&opts)
	setSyncOptsEnvs(&opts)

	container := v1.Container{
		Name:            "git-sync-code",
		Image:           opts.Image,
		Env:             opts.Envs,
		ImagePullPolicy: v1.PullIfNotPresent,
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      mountVolume.Name,
				ReadOnly:  false,
				MountPath: opts.RootPath,
			},
		},
	}

	relativeCodePath := opts.DestPath
	if opts.RelativeCodePath != "" {
		relativeCodePath = opts.RelativeCodePath
	}

	return &container, relativeCodePath, opts.DestPath, nil
}

func setDefaultSyncOpts(opts *GitSyncOptions) {
	if opts.RootPath == "" {
		opts.RootPath = DefaultCodeRootPath
	}
	// Default as project name parsed from git path.
	if opts.DestPath == "" {
		parts := strings.Split(strings.Trim(opts.Source, "/"), "/")
		opts.DestPath = parts[len(parts)-1]
		if strings.HasSuffix(opts.DestPath, ".git") {
			opts.DestPath = opts.DestPath[:len(opts.DestPath)-4]
		}
	}
	if opts.Image == "" {
		opts.Image = defaultGitSyncImage
	}
	if opts.MaxFailures == 0 {
		opts.MaxFailures = 3
	}
}

func setSyncOptsEnvs(opts *GitSyncOptions) {
	opts.Envs = append(opts.Envs, v1.EnvVar{
		Name:  GitSyncRepoEnv,
		Value: opts.Source,
	})
	// Critical: if it's false the init container will never exit.
	opts.Envs = append(opts.Envs, v1.EnvVar{
		Name:  GitSyncOneTimeEnv,
		Value: "true",
	})
	if opts.MaxFailures >= 0 {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncMaxFailuresEnv,
			Value: strconv.Itoa(opts.MaxFailures),
		})
	}
	if opts.Branch != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncBranchEnv,
			Value: opts.Branch,
		})
	}
	if opts.Revision != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncRevisionEnv,
			Value: opts.Revision,
		})
	}
	if opts.Depth != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncDepthEnv,
			Value: opts.Depth,
		})
	}
	if opts.RootPath != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncRootEnv,
			Value: opts.RootPath,
		})
	}
	if opts.DestPath != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncDestEnv,
			Value: opts.DestPath,
		})
	}
	if opts.SSH {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncSSHEnv,
			Value: "true",
		})
	}
	if opts.SSH && opts.SSHFile != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSSHKeyFileEnv,
			Value: opts.SSHFile,
		})
	}
	if opts.User != "" {
		// See SECURITY note on GitSyncUsernameEnv/GitSyncPasswordEnv above.
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncUsernameEnv,
			Value: opts.User,
		})
	}
	if opts.Password != "" {
		opts.Envs = append(opts.Envs, v1.EnvVar{
			Name:  GitSyncPasswordEnv,
			Value: opts.Password,
		})
	}
}
