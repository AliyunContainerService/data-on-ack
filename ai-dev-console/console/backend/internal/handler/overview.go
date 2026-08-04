package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
)

func buildOverview(
	notebookSvc *service.NotebookService,
	trainingSvc *service.TrainingService,
	servingSvc *service.ServingService,
	namespaces []string,
	_ string, // userName for future use
) model.DashboardOverview {
	overview := model.DashboardOverview{}

	// Notebooks
	notebooks, err := notebookSvc.List(namespaces)
	if err == nil {
		overview.Notebooks.Total = len(notebooks)
		for _, nb := range notebooks {
			switch nb.Status {
			case "Running":
				overview.Notebooks.Running++
			case "Failed":
				overview.Notebooks.Failed++
			case "Pending":
				overview.Notebooks.Pending++
			}
		}
	}

	// Training jobs
	jobs, err := trainingSvc.List(namespaces, "")
	if err == nil {
		overview.TrainingJobs.Total = len(jobs)
		for _, j := range jobs {
			switch j.Status {
			case "Running":
				overview.TrainingJobs.Running++
			case "Failed":
				overview.TrainingJobs.Failed++
			case "Pending":
				overview.TrainingJobs.Pending++
			}
		}
	}

	// Serving
	serving, err := servingSvc.List(namespaces)
	if err == nil {
		overview.ServingJobs.Total = len(serving)
		for _, s := range serving {
			switch s.Status {
			case "Ready":
				overview.ServingJobs.Running++
			case "Pending", "Progressing":
				overview.ServingJobs.Pending++
			default:
				overview.ServingJobs.Failed++
			}
		}
	}

	return overview
}
