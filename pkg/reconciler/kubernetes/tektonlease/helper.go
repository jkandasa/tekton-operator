package tektonlease

import "strings"

type LeaseData struct {
	Prefix     string
	Component  string
	Controller string
}

func (ld *LeaseData) Clone() *LeaseData {
	return &LeaseData{
		Prefix:     ld.Prefix,
		Component:  ld.Component,
		Controller: ld.Controller,
	}
}

var (
	leasePrefixes = []LeaseData{
		{Component: "pipeline", Controller: "pipelinerun", Prefix: "tekton-pipelines-controller.github.com.tektoncd.pipeline.pkg.reconciler.pipelinerun.reconciler."},
		{Component: "pipeline", Controller: "taskrun", Prefix: "tekton-pipelines-controller.github.com.tektoncd.pipeline.pkg.reconciler.taskrun.reconciler."},
		{Component: "pipeline", Controller: "resolutionrequest", Prefix: "tekton-pipelines-controller.github.com.tektoncd.pipeline.pkg.reconciler.resolutionrequest.reconciler."},
		{Component: "pipeline", Controller: "customrun", Prefix: "events-controller.github.com.tektoncd.pipeline.pkg.reconciler.notifications.customrun.reconciler."},
	}
)

func getLeaseData(leaseName string) *LeaseData {
	for _, leaseData := range leasePrefixes {
		if strings.HasPrefix(leaseName, leaseData.Prefix) {
			return leaseData.Clone()
		}
	}
	return nil
}

func hasMatchingLeaseData(leaseName string) bool {
	return getLeaseData(leaseName) != nil
}
