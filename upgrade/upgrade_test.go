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
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
)

func noop(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, cfg *rest.Config) error {
	return nil
}

func TestVersions(t *testing.T) {
	upgradeVersions := map[string]upgradeFunction{
		"0.70.0":    noop,
		"0.68.4-2":  noop,
		"0.68.0-1":  noop,
		"0.89.1-10": noop,
		"0.89.1-9":  noop,
		"0.70.2":    noop,
	}
	sortedSemVersions := []*semver.Version{
		semver.MustParse("0.68.0-1"),
		semver.MustParse("0.68.4-2"),
		semver.MustParse("0.70.0"),
		semver.MustParse("0.70.2"),
		semver.MustParse("0.89.1-9"),
		semver.MustParse("0.89.1-10"),
	}

	semVersions, err := versions(upgradeVersions)
	assert.NoError(t, err)
	assert.Equal(t, semVersions, sortedSemVersions)
}

func TestVersionsError(t *testing.T) {
	upgradeVersions := map[string]upgradeFunction{
		"0.70.0":    noop,
		"0,68.4-2":  noop, // bad format, comma instead of dot
		"0.68.0-1":  noop,
		"0.89.1-10": noop,
		"0.89.1-9":  noop,
		"0.70.2":    noop,
	}

	_, err := versions(upgradeVersions)
	assert.Error(t, err)
}

func TestGetLatestUpgradeVersion(t *testing.T) {
	semanticVersions = []*semver.Version{
		semver.MustParse("0.68.0-1"),
		semver.MustParse("0.68.4-2"),
		semver.MustParse("0.70.0"),
		semver.MustParse("0.70.2"),
		semver.MustParse("0.89.1-9"),
		semver.MustParse("0.89.1-10"),
	}
	latestVersion := GetLatestUpgradeVersion()
	assert.Equal(t, "0.89.1-10", latestVersion.String())
}

func TestOperatorWebhookAvailableStatusFunc(t *testing.T) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx) // returns fallback logger
	k8sClient := fake.NewSimpleClientset()

	statusFunc := operatorWebhookAvailableStatusFunc(ctx, logger, k8sClient)

	// TEST
	// webhook "validation.webhook.operator.tekton.dev" not available
	// should return false and no error
	status, err := statusFunc()
	assert.NoError(t, err)
	assert.False(t, status)

	// add webhook, without service details
	validationWebhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorValidationWebhookName,
		},
	}
	_, err = k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(ctx, validationWebhookConfig, metav1.CreateOptions{})
	assert.NoError(t, err)

	// TEST
	// test without service details, returns false and error
	status, err = statusFunc()
	assert.Error(t, err)
	assert.False(t, status)

	// update service details
	validationWebhookConfig.Webhooks = []admissionregistrationv1.ValidatingWebhook{
		{
			Name: operatorValidationWebhookName,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "webhook-endpoints",
					Namespace: "tekton-custom-operator",
				},
			},
		},
	}
	_, err = k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, validationWebhookConfig, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// TEST
	// service details available, but actual service is not available
	// test returns false and no error
	status, err = statusFunc()
	assert.NoError(t, err)
	assert.False(t, status)

	// create webhookService
	webhookService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-endpoints",
			Namespace: "tekton-custom-operator",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "webhook-app",
				"foo": "bar",
			},
		},
	}
	_, err = k8sClient.CoreV1().Services(webhookService.GetNamespace()).Create(ctx, webhookService, metav1.CreateOptions{})
	assert.NoError(t, err)

	// TEST
	// webhook service available, but pods are not available
	// test returns false and no error
	status, err = statusFunc()
	assert.NoError(t, err)
	assert.False(t, status)

	// create pods
	endpointPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-custom-webhook-pod-abc123",
			Namespace: webhookService.GetNamespace(),
			Labels:    webhookService.Spec.Selector,
		},
	}
	_, err = k8sClient.CoreV1().Pods(endpointPod.GetNamespace()).Create(ctx, endpointPod, metav1.CreateOptions{})
	assert.NoError(t, err)

	// TEST
	// webhook endpoint pods are available, but not in running state
	// test returns false and no error
	status, err = statusFunc()
	assert.NoError(t, err)
	assert.False(t, status)

	// update the webhook endpoint status to running
	endpointPod.Status.Phase = corev1.PodRunning
	endpointPod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.ContainersReady,
			Status: corev1.ConditionTrue,
		},
	}
	_, err = k8sClient.CoreV1().Pods(endpointPod.GetNamespace()).Update(ctx, endpointPod, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// TEST
	// webhook endpoint pods are available and in running state
	// test returns true and no error
	status, err = statusFunc()
	assert.NoError(t, err)
	assert.True(t, status)
}
