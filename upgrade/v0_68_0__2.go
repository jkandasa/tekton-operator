/*
Copyright 2023 The Tekton Authors

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

package upgrade

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// previous version of tekton operator uses a condition type called "InstallSucceeded" in status
// but in the recent version we do not have that field, hence "InstallSucceeded" condition never updated.
// for some reason, if it was in failed state, tektonConfig CR always in failed state
// even though all the resources are up and running. as the operator sums all the status conditions
// to avoid this, remove all the existing conditions from the status of the CR.
// conditions will be repopulated
func upgrade_0_68_0__2(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, cfg *rest.Config) error {
	// get operator client
	operatorClient := operatorclient.Get(ctx)

	// fetch the current tektonConfig CR
	tcCR, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// remove all the existing conditions
	tcCR.Status.Conditions = duckv1.Conditions{}
	// update the status
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, tcCR, metav1.UpdateOptions{})
	return err
}
