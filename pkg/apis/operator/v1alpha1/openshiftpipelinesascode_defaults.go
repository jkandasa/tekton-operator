/*
Copyright 2022 The Tekton Authors

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

package v1alpha1

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	pacSettings "github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

func (pac *OpenShiftPipelinesAsCode) SetDefaults(ctx context.Context) {
	if pac.Spec.PACSettings.Settings == nil {
		pac.Spec.PACSettings.Settings = map[string]string{}
	}
	if pac.Spec.PACSettings.AdditionalPACControllers == nil {
		pac.Spec.PACSettings.AdditionalPACControllers = map[string]AdditionalPACControllerConfig{}
	}
	logger := logging.FromContext(ctx)
	pac.Spec.PACSettings.setPACDefaults(logger)
}

func (set *PACSettings) setPACDefaults(logger *zap.SugaredLogger) {
	if set.Settings == nil {
		set.Settings = map[string]string{}
	}
	defaultPacSettings := pacSettings.DefaultSettings()
	err := pacSettings.SyncConfig(logger, &defaultPacSettings, set.Settings)
	if err != nil {
		logger.Error("error on applying default PAC settings", err)
	}
	set.Settings = ConvertPacStructToConfigMap(&defaultPacSettings)
	setAdditionalPACControllerDefault(set.AdditionalPACControllers)
}

// Set the default values for additional PAc controller resources
func setAdditionalPACControllerDefault(additionalPACController map[string]AdditionalPACControllerConfig) {
	for name, additionalPACInfo := range additionalPACController {
		if additionalPACInfo.Enable == nil {
			additionalPACInfo.Enable = ptr.Bool(true)
		}
		if additionalPACInfo.ConfigMapName == "" {
			additionalPACInfo.ConfigMapName = fmt.Sprintf("%s-pipelines-as-code-configmap", name)
		}
		if additionalPACInfo.SecretName == "" {
			additionalPACInfo.SecretName = fmt.Sprintf("%s-pipelines-as-code-secret", name)
		}
		additionalPACController[name] = additionalPACInfo
	}
}

func ConvertPacStructToConfigMap(settings *pacSettings.Settings) map[string]string {
	structValue := reflect.ValueOf(settings).Elem()
	structType := reflect.TypeOf(settings).Elem()
	config := map[string]string{}

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		key := strings.ToLower(jsonTag)
		element := structValue.FieldByName(fieldName)
		if !element.IsValid() {
			continue
		}

		//nolint
		switch field.Type.Kind() {
		case reflect.String:
			config[key] = element.String()
		case reflect.Bool:
			config[key] = strconv.FormatBool(element.Bool())
		case reflect.Int:
			if element.Int() == 0 {
				config[key] = ""
				continue
			}
			config[key] = strconv.FormatInt(element.Int(), 10)

		case reflect.Ptr:
			if key == "" {
				catalogData, ok := element.Interface().(*sync.Map)
				if !ok {
					continue
				}

				// collect the keys
				catalogIDs := []any{}
				catalogData.Range(func(catalogID, value any) bool {
					_, ok := value.(pacSettings.HubCatalog)
					if !ok {
						return true
					}
					catalogIDs = append(catalogIDs, catalogID)
					return true
				})
				// sort the keys
				sort.Slice(catalogIDs, func(i, j int) bool {
					return fmt.Sprintf("%s", catalogIDs[i]) < fmt.Sprintf("%s", catalogIDs[j])
				})

				index := uint64(1)
				for _, catalogID := range catalogIDs {
					value, ok := catalogData.Load(catalogID)
					if !ok {
						continue
					}

					catalogData := value.(pacSettings.HubCatalog)
					if catalogID == "default" {
						config[pacSettings.HubURLKey] = catalogData.URL
						config[pacSettings.HubCatalogNameKey] = catalogData.Name
						continue
					}
					config[fmt.Sprintf("catalog-%d-id", index)] = catalogData.ID
					config[fmt.Sprintf("catalog-%d-name", index)] = catalogData.Name
					config[fmt.Sprintf("catalog-%d-url", index)] = catalogData.URL
					index++
				}
			}

		default:
			// Skip unsupported field types
			continue
		}
	}

	return config
}
