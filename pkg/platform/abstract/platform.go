/*
Copyright 2017 The Nuclio Authors.

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

package abstract

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"reflect"

	"github.com/nuclio/logger"
)

//
// Base for all platforms
//

type Platform struct {
	Logger              logger.Logger
	platform            platform.Platform
	invoker             *invoker
	ExternalIPAddresses []string
	DeployLogStreams    map[string]*LogStream
}

func NewPlatform(parentLogger logger.Logger, platform platform.Platform) (*Platform, error) {
	var err error

	newPlatform := &Platform{
		Logger:           parentLogger.GetChild("platform"),
		platform:         platform,
		DeployLogStreams: map[string]*LogStream{},
	}

	// create invoker
	newPlatform.invoker, err = newInvoker(newPlatform.Logger, platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create invoker")
	}

	return newPlatform, nil
}

func (ap *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {

	// execute a build
	builder, err := build.NewBuilder(createFunctionBuildOptions.Logger, ap.platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	return builder.Build(createFunctionBuildOptions)
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *Platform) HandleDeployFunction(existingFunctionConfig *functionconfig.ConfigWithStatus,
	createFunctionOptions *platform.CreateFunctionOptions,
	onAfterConfigUpdated func(*functionconfig.Config) error,
	onAfterBuild func(*platform.CreateFunctionBuildResult, error) (*platform.CreateFunctionResult, error)) (*platform.CreateFunctionResult, error) {

	createFunctionOptions.Logger.InfoWith("Deploying function", "name", createFunctionOptions.FunctionConfig.Meta.Name)

	var buildResult *platform.CreateFunctionBuildResult
	var buildErr error

	// when the config is updated, save to deploy options and call underlying hook
	onAfterConfigUpdatedWrapper := func(updatedFunctionConfig *functionconfig.Config) error {
		createFunctionOptions.FunctionConfig = *updatedFunctionConfig

		return onAfterConfigUpdated(updatedFunctionConfig)
	}

	functionBuildRequired, err := ap.functionBuildRequired(existingFunctionConfig, createFunctionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed determining whether function should build")
	}

	createFunctionOptions.Logger.InfoWith("Build req", "req", functionBuildRequired)

	// check if we need to build the image
	if functionBuildRequired {
		buildResult, buildErr = ap.platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
			Logger:              createFunctionOptions.Logger,
			FunctionConfig:      createFunctionOptions.FunctionConfig,
			PlatformName:        ap.platform.GetName(),
			OnAfterConfigUpdate: onAfterConfigUpdatedWrapper,
		})

		if buildErr == nil {

			// use the function configuration augmented by the builder
			createFunctionOptions.FunctionConfig.Spec.Image = buildResult.Image

			// if run registry isn't set, set it to that of the build
			if createFunctionOptions.FunctionConfig.Spec.RunRegistry == "" {
				createFunctionOptions.FunctionConfig.Spec.RunRegistry = createFunctionOptions.FunctionConfig.Spec.Build.Registry
			}
		}
	} else {

		// no function build required and no image passed, means to use latest known image
		if existingFunctionConfig != nil && createFunctionOptions.FunctionConfig.Spec.Image == "" {
			createFunctionOptions.FunctionConfig.Spec.Image = existingFunctionConfig.Spec.Image
		}

		// verify user passed runtime
		if createFunctionOptions.FunctionConfig.Spec.Runtime == "" {
			return nil, errors.New("If image is passed, runtime must be specified")
		}

		// trigger the on after config update ourselves
		if err = onAfterConfigUpdatedWrapper(&createFunctionOptions.FunctionConfig); err != nil {
			return nil, errors.Wrap(err, "Failed to trigger on after config update")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction
	deployResult, err := onAfterBuild(buildResult, buildErr)
	if buildErr != nil || err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// sanity
	if deployResult == nil {
		return nil, errors.New("Deployer returned no error, but nil deploy result")
	}

	// if we got a deploy result and build result, set them
	if buildResult != nil {
		deployResult.CreateFunctionBuildResult = *buildResult
	}

	// indicate that we're done
	createFunctionOptions.Logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, nil
}

// CreateFunctionInvocation will invoke a previously deployed function
func (ap *Platform) CreateFunctionInvocation(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	return ap.invoker.invoke(createFunctionInvocationOptions)
}

// GetHealthCheckMode returns the healthcheck mode the platform requires
func (ap *Platform) GetHealthCheckMode() platform.HealthCheckMode {

	// by default return that some external entity does health checks for us
	return platform.HealthCheckModeExternal
}

// CreateProject will probably create a new project
func (ap *Platform) CreateProject(createProjectOptions *platform.CreateProjectOptions) error {
	return errors.New("Unsupported")
}

// UpdateProject will update a previously existing project
func (ap *Platform) UpdateProject(updateProjectOptions *platform.UpdateProjectOptions) error {
	return errors.New("Unsupported")
}

// DeleteProject will delete a previously existing project
func (ap *Platform) DeleteProject(deleteProjectOptions *platform.DeleteProjectOptions) error {
	return errors.New("Unsupported")
}

// GetProjects will list existing projects
func (ap *Platform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	return nil, errors.New("Unsupported")
}

// CreateFunctionEvent will create a new function event that can later be used as a template from
// which to invoke functions
func (ap *Platform) CreateFunctionEvent(createFunctionEventOptions *platform.CreateFunctionEventOptions) error {
	return errors.New("Unsupported")
}

// UpdateFunctionEvent will update a previously existing function event
func (ap *Platform) UpdateFunctionEvent(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) error {
	return errors.New("Unsupported")
}

// DeleteFunctionEvent will delete a previously existing function event
func (ap *Platform) DeleteFunctionEvent(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) error {
	return errors.New("Unsupported")
}

// GetFunctionEvents will list existing function events
func (ap *Platform) GetFunctionEvents(getFunctionEventsOptions *platform.GetFunctionEventsOptions) ([]platform.FunctionEvent, error) {
	return nil, errors.New("Unsupported")
}

// SetExternalIPAddresses configures the IP addresses invocations will use, if "via" is set to "external-ip".
// If this is not invoked, each platform will try to discover these addresses automatically
func (ap *Platform) SetExternalIPAddresses(externalIPAddresses []string) error {
	ap.ExternalIPAddresses = externalIPAddresses

	return nil
}

// GetExternalIPAddresses returns the external IP addresses invocations will use, if "via" is set to "external-ip".
// These addresses are either set through SetExternalIPAddresses or automatically discovered
func (ap *Platform) GetExternalIPAddresses() ([]string, error) {
	return ap.ExternalIPAddresses, nil
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func (ap *Platform) ResolveDefaultNamespace(defaultNamespace string) string {
	return ""
}

func (ap *Platform) functionBuildRequired(existingFunctionConfig *functionconfig.ConfigWithStatus,
	createFunctionOptions *platform.CreateFunctionOptions) (bool, error) {

	// check if we have something to compare to, if so, check if anything changed
	if existingFunctionConfig == nil ||
		existingFunctionConfig.Status.State == functionconfig.FunctionStateError ||
		!ap.equalFunctionConfigs(&existingFunctionConfig.Config, &createFunctionOptions.FunctionConfig) {

		// if the function contains source code, an image name or a path somewhere - we need to rebuild. the shell
		// runtime supports a case where user just tells image name and we build around the handler without a need
		// for a path
		if createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode != "" ||
			createFunctionOptions.FunctionConfig.Spec.Build.Path != "" ||
			createFunctionOptions.FunctionConfig.Spec.Build.Image != "" {
			return true, nil
		}

		// if user didn't give any of the above but _did_ specify an image to run from, just dont build
		if createFunctionOptions.FunctionConfig.Spec.Image != "" {
			return false, nil
		}

		// should not get here - we should either be able to build an image or have one specified for us
		return false, errors.New("Function must have either spec.build.path," +
			"spec.build.functionSourceCode, spec.build.image or spec.image set in order to create")
	}

	return false, nil
}

func (ap *Platform) equalFunctionConfigs(existingFunctionConfig *functionconfig.Config,
	createFunctionConfig *functionconfig.Config) bool {

		// TODO use more reflection here
		existingBuild := existingFunctionConfig.Spec.Build
		createBuild := createFunctionConfig.Spec.Build

		if existingBuild.NoBaseImagesPull != createBuild.NoBaseImagesPull {
			return false
		}
		if existingBuild.Offline != createBuild.Offline {
			return false
		}
		if existingBuild.CodeEntryType != createBuild.CodeEntryType {
			return false
		}
		if existingBuild.BaseImage != createBuild.BaseImage {
			return false
		}
		if existingBuild.Path != createBuild.Path {
			return false
		}
		if existingBuild.FunctionSourceCode != createBuild.FunctionSourceCode {
			return false
		}
		if existingBuild.FunctionConfigPath != createBuild.FunctionConfigPath {
			return false
		}
		if existingBuild.TempDir != createBuild.TempDir {
			return false
		}
		if existingBuild.Registry != createBuild.Registry {
			return false
		}
		if existingBuild.Image != createBuild.Image {
			return false
		}
		if existingBuild.NoCache != createBuild.NoCache {
			return false
		}
		if existingBuild.NoCleanup != createBuild.NoCleanup {
			return false
		}
		if existingBuild.OnbuildImage != createBuild.OnbuildImage {
			return false
		}
		if !ap.equalStringSlices(existingBuild.Commands, createBuild.Commands) {
			return false
		}
		if !ap.equalStringSlices(existingBuild.Dependencies, createBuild.Dependencies) {
			return false
		}
		if !ap.equalStringInterfaceMaps(existingBuild.RuntimeAttributes, createBuild.RuntimeAttributes) {
			return false
		}
		if !ap.equalStringInterfaceMaps(existingBuild.CodeEntryAttributes, createBuild.CodeEntryAttributes) {
			return false
		}
		if !ap.equalStringDirectivesMaps(existingBuild.Directives, createBuild.Directives) {
			return false
		}

		return true

}

func (ap *Platform) equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func (ap *Platform) equalStringInterfaceMaps(a map[string]interface{}, b map[string]interface{}) bool {
	for k, v := range a {
		if reflect.TypeOf(b[k]) != reflect.TypeOf(v) {
			return false
		}

		if b[k] != v {
			return false
		}
	}
	return true
}

func (ap *Platform) equalStringDirectivesMaps(a map[string][]functionconfig.Directive,
	b map[string][]functionconfig.Directive) bool {
	for k, v := range a {
		if len(v) != len(b[k]) {
			return false
		}

		for i, directive := range v {
			if directive != b[k][i] {
				return false
			}
		}
	}
	return true
}

