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

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type upgradeFunction = func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, cfg *rest.Config) error

// map all the upgrade paths
// version should start with next release version and maintain sequence of "-1", "-2" to support multiple upgrades with in a version.
// also it is possible to upgrade between development version and production version
var upgrades = map[string]upgradeFunction{
	"0.68.0-1": upgrade_0_68_0__1, // v0.68.0 upgrade #1
	"0.68.0-2": upgrade_0_68_0__2, // v0.68.0 upgrade #2
}
