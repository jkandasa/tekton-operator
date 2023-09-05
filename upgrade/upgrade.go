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
	"fmt"
	"sort"
	"time"

	"github.com/Masterminds/semver"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/pkg/logging"
)

const (
	operatorValidationWebhookName = "validation.webhook.operator.tekton.dev"
)

var (
	semanticVersions []*semver.Version
)

func init() {
	// parse semVer
	semvers, err := versions(upgrades)
	if err != nil {
		panic(err)
	}
	semanticVersions = semvers
}

// version return the list of semantic version sorted
func versions(versions map[string]upgradeFunction) ([]*semver.Version, error) {
	versionLists := make([]*semver.Version, len(versions))
	versionIndex := 0
	for v := range versions {
		semv, err := semver.NewVersion(v)
		if err != nil {
			return nil, err
		}
		versionLists[versionIndex] = semv
		versionIndex++
	}

	// to apply the updates in order
	sort.Sort(semver.Collection(versionLists))
	return versionLists, nil
}

// returns the most recent upgrade version
// will be used in on new installation.
// add in the tektonConfig CR as a reference, to avoid upgrades on fresh installation
func GetLatestUpgradeVersion() *semver.Version {
	size := len(semanticVersions)
	return semanticVersions[size-1]
}

// starts the upgrade process
func StartUpgrade(ctx context.Context, cfg *rest.Config) {
	logger := logging.FromContext(ctx).Named("upgrade")

	// get kubernetes client
	k8sClient := kubeclient.Get(ctx)

	// get operator client
	operatorClient := operatorclient.Get(ctx)

	// get the last applied upgrade version
	cr, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("looks like this is fresh installation, upgrade not required.")
			return
		}
		logger.Fatalw("error on getting tektonConfig CR", err)
	}

	// get the last applied version
	lastAppliedUpgrade := cr.Status.GetLastAppliedUpgradeVersion()
	if lastAppliedUpgrade == "" { // there is no upgrade performed yet, keep 0.0.0 as reference version
		lastAppliedUpgrade = "0.0.0"
	}

	lastAppliedUpgradeVersion, err := semver.NewVersion(lastAppliedUpgrade)
	if err != nil {
		logger.Fatal("error on parsing version", zap.String("lastAppliedUpgradeVersion", lastAppliedUpgrade), zap.Error(err))
	}

	// check, if upgrade required
	latestUpgradeVersion := GetLatestUpgradeVersion()
	if latestUpgradeVersion.GreaterThan(lastAppliedUpgradeVersion) {
		waitConditionFunc := operatorWebhookAvailableStatusFunc(ctx, logger, k8sClient)
		interval := time.Second * 10
		timeout := time.Minute * 3
		logger.Infow("waiting for operator webhook endpoint availability",
			"webhookName", operatorValidationWebhookName,
			"pollInterval", interval.String(),
			"pollTimeout", timeout.String(),
		)
		err = wait.PollImmediate(interval, timeout, waitConditionFunc)
		if err != nil {
			logger.Fatalw("error on waiting to operator webhook availability", "webhookName", operatorValidationWebhookName, err)
		}
	} else {
		// upgrade not required
		logger.Infow("lastAppliedUpgradeVersion is greater than or equal to latestUpgradeVersion, skipping upgrade",
			"lastAppliedUpgradeVersion", lastAppliedUpgradeVersion.String(),
			"latestUpgradeVersion", latestUpgradeVersion.String(),
		)
		return
	}

	// execute all the required upgrades
	for _, targetUpgradeVersion := range semanticVersions {
		if targetUpgradeVersion.GreaterThan(lastAppliedUpgradeVersion) {
			logger.Infow("upgrade triggered", "targetUpgradeVersion", targetUpgradeVersion.String())
			err = upgrades[targetUpgradeVersion.String()](ctx, logger, k8sClient, cfg)
			if err != nil {
				logger.Fatalw("error on upgrade", "targetUpgradeVersion", targetUpgradeVersion.String(), err)
			}

			appliedUpgradeVersion := targetUpgradeVersion.String()
			logger.Infow("upgrade successful", "appliedUpgradeVersion", appliedUpgradeVersion)

			// update last applied version into TektonConfig CR, under status
			_cr, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
			if err != nil {
				logger.Fatalw("error on getting tektonConfig CR", "appliedUpgradeVersion", appliedUpgradeVersion, err)
			}
			_cr.Status.SetLastAppliedUpgradeVersion(appliedUpgradeVersion)
			_, err = operatorClient.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, _cr, metav1.UpdateOptions{})
			if err != nil {
				logger.Fatalw("error on updating tektonConfig CR status", "appliedUpgradeVersion", appliedUpgradeVersion, err)
			}
		}
	}
}

func operatorWebhookAvailableStatusFunc(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface) wait.ConditionFunc {
	return func() (bool, error) {
		webhookCfg, err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, operatorValidationWebhookName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				logger.Infow("webhook not available", "webhookName", operatorValidationWebhookName)
				return false, nil
			}
			return false, err
		}

		var serviceReference *admissionregistrationv1.ServiceReference
		for _, webhook := range webhookCfg.Webhooks {
			if webhook.Name == operatorValidationWebhookName {
				serviceReference = webhook.ClientConfig.Service
			}
		}
		if serviceReference == nil {
			return false, fmt.Errorf("service details are not updated in webhook: %s", operatorValidationWebhookName)
		}

		service, err := k8sClient.CoreV1().Services(serviceReference.Namespace).Get(ctx, serviceReference.Name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				logger.Infow("webhook service is not available", "webhookName", webhookCfg.Name)
				return false, nil
			}
			return false, err
		}

		// verify endpoints availability
		endpointSelector := metav1.LabelSelector{
			MatchLabels: service.Spec.Selector,
		}

		labelSelector, err := common.LabelSelector(endpointSelector)
		if err != nil {
			return false, err
		}
		podList, err := k8sClient.CoreV1().Pods(service.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, err
		}
		if len(podList.Items) > 0 {
			for _, pod := range podList.Items {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.ContainersReady && condition.Status == corev1.ConditionTrue {
						logger.Infow("webhook pod is in running state", "podName", pod.Name, "podNamespace", pod.Namespace)
						return true, nil
					}
				}
				logger.Infow("webhook pod is not in running state", "podName", pod.Name, "podNamespace", pod.Namespace)
			}
			logger.Infow("webhook endpoints are available, but non of pods are in running state", "labelSelector", service.Spec.Selector, "numberOfEndpoints", len(podList.Items))
			return false, nil
		}
		logger.Infow("webhook endpoints are not available", "labelSelector", service.Spec.Selector)
		return false, nil
	}
}
