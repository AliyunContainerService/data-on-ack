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

package arena

import (
	"fmt"
	trainingv1alpha1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	clientregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/tenant"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"github.com/kubeflow/arena/pkg/apis/cron"
	"github.com/kubeflow/arena/pkg/apis/evaluate"
	"github.com/kubeflow/arena/pkg/apis/training"
	"k8s.io/klog"
)

func NewArenaClientBackend() backends.ObjectClientBackend {
	return &arenaBackend{
		arena: clientmgr.GetArenaClient(),
	}
}

var _ backends.ObjectClientBackend = &arenaBackend{}

type arenaBackend struct {
	arena    *arenaclient.ArenaClient
	userName string
}

func (a *arenaBackend) Initialize() error {
	return nil
}
func (a *arenaBackend) Close() error {
	return nil
}

func (a *arenaBackend) Name() string {
	return "arena"
}

func (a *arenaBackend) UserName(userName string) backends.ObjectClientBackend {
	copyArenaBackend := &arenaBackend{
		arena:    a.arena,
		userName: userName,
	}
	return copyArenaBackend
}

func (a *arenaBackend) getArenaClient() *arenaclient.ArenaClient {
	var arena *arenaclient.ArenaClient
	var err error
	if a.userName == "" {
		arena = a.arena
	} else {
		arena, err = clientregistry.GetArenaClient(a.userName)
		if err != nil {
			klog.Errorf("get arena client of user %s failed, err:%v", a.userName, err)
		}
	}
	return arena
}

func (a *arenaBackend) SubmitEvaluateJob(evaluateJob *dmo.SubmitEvaluateJobInfo) error {
	submitEvaluateJob, err := genArenaSubmitEvaluateJob(evaluateJob)
	if err != nil {
		klog.Errorf("failed to build evaluateJob, reason: %v\n", err)
		return err
	}
	ns := evaluateJob.Namespace
	if ns == "" {
		ns = "default"
	}

	if err := a.getArenaClient().Evaluate().Namespace(ns).SubmitEvaluateJob(submitEvaluateJob); err != nil {
		klog.Errorf("failed to submit evaluateJob, reason: %v\n", err)
		return err
	}
	return nil
}

func (a *arenaBackend) SubmitJob(job *dmo.SubmitJobInfo) error {
	if job.EnableCron {
		return a.submitCron(job)
	}
	submitJob, err := genArenaSubmitJob(job)
	if err != nil {
		klog.Errorf("failed to build training job, reason: %v\n", err)
		return err
	}
	ns := job.Namespace
	if ns == "" {
		ns = "default"
	}

	if err := a.getArenaClient().Training().Namespace(ns).Submit(submitJob); err != nil {
		klog.Errorf("failed to submit job, reason: %v\n", err)
		return err
	}
	return nil
}

func (a *arenaBackend) submitCron(job *dmo.SubmitJobInfo) error {
	cronJob, err := genArenaSubmitCron(job)
	if err != nil {
		klog.Errorf("failed to build cron, reason: %v\n", err)
		return err
	}

	ns := job.Namespace
	if ns == "" {
		ns = "default"
	}
	if err = a.getArenaClient().Cron().Namespace(ns).SubmitCronTrainingJob(cronJob); err != nil {
		klog.Errorf("failed to submit cron, reason: %v\n", err)
		return err
	}
	return nil
}

func (a *arenaBackend) SuspendCron(ns, name, cronID string) error {
	if ns == "" {
		ns = "default"
	}
	err := a.getArenaClient().Cron().Namespace(ns).Suspend(name)
	if err != nil {
		klog.Errorf("suspend cron %v/%v error: %v", ns, name, err)
		return err
	}
	return nil
}

func (a *arenaBackend) ResumeCron(ns, name, cronID string) error {
	if ns == "" {
		ns = "default"
	}
	err := a.getArenaClient().Cron().Namespace(ns).Resume(name)
	if err != nil {
		klog.Errorf("resume cron %v/%v error: %v", ns, name, err)
		return err
	}
	return nil
}

func (a *arenaBackend) StopCron(ns, name, cronID string) error {
	if ns == "" {
		ns = "default"
	}
	err := a.getArenaClient().Cron().Namespace(ns).Delete(name)
	if err != nil {
		klog.Errorf("delete cron %v/%v error: %v", ns, name, err)
	}
	return err
}

func (a *arenaBackend) StopJob(ns, name, jobID, kind string) error {
	err := a.getArenaClient().Training().Namespace(ns).Delete(utils.GetArenaJobTypeFromKind(kind), name)
	if err != nil {
		klog.Errorf("delete job %v/%v error: %v", ns, name, err)
		return err
	}
	return nil
}

func (a *arenaBackend) DeleteEvaluateJob(ns, name string) error {
	err := a.getArenaClient().Evaluate().Namespace(ns).Delete(name)
	if err != nil {
		klog.Errorf("delete evaluateJob %v/%v error: %v", ns, name, err)
		return err
	}
	return nil
}

func genArenaSubmitEvaluateJob(evaluateJob *dmo.SubmitEvaluateJobInfo) (*evaluate.EvaluateJob, error) {
	envs := evaluateJob.Envs
	if evaluateJob.CodeBranch != "" {
		envs["GIT_SYNC_BRANCH"] = evaluateJob.CodeBranch
	}
	if evaluateJob.CodeUser != "" && evaluateJob.CodePassword != "" {
		envs["GIT_SYNC_USERNAME"] = evaluateJob.CodeUser
		envs["GIT_SYNC_PASSWORD"] = evaluateJob.CodePassword
	}
	builder := evaluate.NewEvaluateJobBuilder().
		Name(evaluateJob.Name).
		Namespace(evaluateJob.Namespace).
		WorkingDir(evaluateJob.WorkingDir).
		Command(evaluateJob.Command).
		Image(evaluateJob.Image).
		Annotations(evaluateJob.Annotations).
		ImagePullSecrets(evaluateJob.ImagePullSecrets).
		SyncMode(evaluateJob.CodeType).
		SyncSource(evaluateJob.CodeSource).
		ModelName(evaluateJob.ModelName).
		ModelPath(evaluateJob.ModelPath).
		ModelVersion(evaluateJob.ModelVersion).
		DatasetPath(evaluateJob.DatasetPath).
		MetricsPath(evaluateJob.MetricsPath).
		Cpu(evaluateJob.CPU).
		Gpu(evaluateJob.GPU).
		Memory(evaluateJob.Memory).
		Datas(evaluateJob.DataSources)
	if len(envs) > 0 {
		builder.Envs(envs)
	}
	return builder.Build()
}

func genArenaSubmitJob(job *dmo.SubmitJobInfo) (*training.Job, error) {
	switch job.Kind {
	case trainingv1alpha1.TFJobKind:
		envs := map[string]string{}
		builder := training.NewTFJobBuilder(nil).
			Name(job.Name).
			Shell(job.Shell).
			Command(job.Command).
			WorkingDir(job.WorkingDir).
			Datas(job.Volumes).
			SyncMode(job.CodeType).
			SyncSource(job.CodeSource).
			GPUCount(job.WorkerGPU).
			WorkerCount(int(job.WorkerCount)).
			WorkerCPU(job.WorkerCPU).
			WorkerMemory(job.WorkerMemory).
			WorkerImage(job.WorkerImage).
			PsCount(int(job.PsCount)).
			PsGPU(job.PsGPU).
			PsCPU(job.PsCPU).
			PsMemory(job.PsMemory).
			PsImage(job.PsImage).
			ImagePullSecrets(job.ImagePullSecrets).
			CleanPodPolicy("None").
			TTLSecondsAfterFinished(job.TTLSecondsAfterFinished).
			NodeSelectors(job.NodeSelectors).
			Annotations(job.Annotations).
			Labels(job.Labels).
			Devices(job.Devices)

		if job.Toleration != nil {
			tolerations := make([]string, 0)
			for key, tolerationValue := range job.Toleration {
				tolerations = append(tolerations, fmt.Sprintf("%s=%s:%s,%s", key, tolerationValue.Value,
					tolerationValue.Effect, tolerationValue.Operator))
			}
			builder.Tolerations(tolerations)
		}

		if job.LogDir != "" {
			builder.LogDir(job.LogDir)
		}

		if job.EnableTensorboard {
			builder.EnableTensorboard()
		}

		// arena should specify --chief-gpu --chief-memory for worker resources in standalone training job
		if job.WorkerCount == 1 {
			builder.ChiefCPU(job.WorkerCPU)
			builder.ChiefMemory(job.WorkerMemory)
		}

		if job.ChiefCPU != "" || job.ChiefMemory != "" {
			builder.ChiefCPU(job.ChiefCPU)
			builder.ChiefMemory(job.ChiefMemory)
			builder.EnableChief()
		}

		if job.EvaluatorCPU != "" || job.EvaluatorMemory != "" {
			builder.EvaluatorCPU(job.EvaluatorCPU)
			builder.EvaluatorMemory(job.EvaluatorMemory)
			builder.EnableEvaluator()
		}

		if job.CodeBranch != "" {
			envs["GIT_SYNC_BRANCH"] = job.CodeBranch
		}
		if job.CodeUser != "" && job.CodePassword != "" {
			envs["GIT_SYNC_USERNAME"] = job.CodeUser
			envs["GIT_SYNC_PASSWORD"] = job.CodePassword
		}
		if len(envs) > 0 {
			builder.Envs(envs)
		}
		return builder.Build()

	case trainingv1alpha1.PyTorchJobKind:
		envs := map[string]string{}
		builder := training.NewPytorchJobBuilder().
			Name(job.Name).
			Shell(job.Shell).
			Command(job.Command).
			WorkingDir(job.WorkingDir).
			Datas(job.Volumes).
			SyncMode(job.CodeType).
			SyncSource(job.CodeSource).
			GPUCount(job.WorkerGPU).
			WorkerCount(int(job.WorkerCount)).
			Image(job.WorkerImage).
			ImagePullSecrets(job.ImagePullSecrets).
			CleanPodPolicy("None").
			TTLSecondsAfterFinished(job.TTLSecondsAfterFinished).
			NodeSelectors(job.NodeSelectors).
			Annotations(job.Annotations).
			Labels(job.Labels).
			Devices(job.Devices)

		if job.LogDir != "" {
			builder.LogDir(job.LogDir)
		}

		if job.Toleration != nil {
			tolerations := make([]string, 0)
			for key, tolerationValue := range job.Toleration {
				tolerations = append(tolerations, fmt.Sprintf("%s=%s:%s,%s", key, tolerationValue.Value,
					tolerationValue.Effect, tolerationValue.Operator))
			}
			builder.Tolerations(tolerations)
		}

		if job.EnableTensorboard {
			builder.EnableTensorboard()
		}

		if job.CodeBranch != "" {
			envs["GIT_SYNC_BRANCH"] = job.CodeBranch
		}
		if job.CodeUser != "" && job.CodePassword != "" {
			envs["GIT_SYNC_USERNAME"] = job.CodeUser
			envs["GIT_SYNC_PASSWORD"] = job.CodePassword
		}
		if len(envs) > 0 {
			builder.Envs(envs)
		}
		return builder.Build()
	}
	return nil, nil
}

func genArenaSubmitCron(job *dmo.SubmitJobInfo) (*cron.Job, error) {
	switch job.Kind {
	case trainingv1alpha1.TFJobKind:
		envs := map[string]string{}
		builder := cron.NewCronTFJobBuilder().
			Name(job.Name).
			Schedule(job.Schedule).
			ConcurrencyPolicy(job.ConcurrencyPolicy).
			HistoryLimit(job.HistoryLimit).
			Deadline(job.Deadline).
			Shell(job.Shell).
			Command(job.Command).
			WorkingDir(job.WorkingDir).
			Datas(job.Volumes).
			SyncMode(job.CodeType).
			SyncSource(job.CodeSource).
			GPUCount(job.WorkerGPU).
			WorkerCount(int(job.WorkerCount)).
			WorkerCPU(job.WorkerCPU).
			WorkerMemory(job.WorkerMemory).
			WorkerImage(job.WorkerImage).
			PsCount(int(job.PsCount)).
			PsGPU(job.PsGPU).
			PsCPU(job.PsCPU).
			PsMemory(job.PsMemory).
			PsImage(job.PsImage).
			ImagePullSecrets(job.ImagePullSecrets).
			CleanPodPolicy("None").
			TTLSecondsAfterFinished(job.TTLSecondsAfterFinished).
			NodeSelectors(job.NodeSelectors).
			Annotations(job.Annotations).
			Labels(job.Labels)

		if job.LogDir != "" {
			builder.LogDir(job.LogDir)
		}

		if job.EnableTensorboard {
			builder.EnableTensorboard()
		}

		// arena should specify --chief-gpu --chief-memory for worker resources in standalone training job
		if job.WorkerCount == 1 {
			builder.ChiefCPU(job.WorkerCPU)
			builder.ChiefMemory(job.WorkerMemory)
		}

		if job.ChiefCPU != "" || job.ChiefMemory != "" {
			builder.ChiefCPU(job.ChiefCPU)
			builder.ChiefMemory(job.ChiefMemory)
			builder.EnableChief()
		}

		if job.EvaluatorCPU != "" || job.EvaluatorMemory != "" {
			builder.EvaluatorCPU(job.EvaluatorCPU)
			builder.EvaluatorMemory(job.EvaluatorMemory)
			builder.EnableEvaluator()
		}

		if job.CodeBranch != "" {
			envs["GIT_SYNC_BRANCH"] = job.CodeBranch
		}
		if job.CodeUser != "" && job.CodePassword != "" {
			envs["GIT_SYNC_USERNAME"] = job.CodeUser
			envs["GIT_SYNC_PASSWORD"] = job.CodePassword
		}
		if len(envs) > 0 {
			builder.Envs(envs)
		}

		return builder.Build()
	}
	return nil, nil
}
