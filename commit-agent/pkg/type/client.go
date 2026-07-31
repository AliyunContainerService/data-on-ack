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

package _type

import "context"

// ContainerClient abstracts the operations the agent performs against a
// container runtime daemon (Docker or containerd). Implementations must be
// safe for concurrent use across goroutines.
type ContainerClient interface {
	CommitImageFromSelf(ctx context.Context, containerID, image string) error
	PushImageFromSelf(ctx context.Context, image, username, password string) error
}
