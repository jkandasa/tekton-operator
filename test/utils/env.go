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

package utils

import (
	"os"
	"strings"
)

const (
	// log level of the logger used in e2e tests
	ENV_LOG_LEVEL = "LOG_LEVEL"
)

var (
	isOpenShift = false
)

func init() {
	isOpenShift = strings.ToLower(os.Getenv("TARGET")) == "openshift"
}

func IsOpenShift() bool {
	return isOpenShift
}

func GetEnvironment(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
