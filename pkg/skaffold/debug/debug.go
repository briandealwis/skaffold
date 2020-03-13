/*
Copyright 2019 The Skaffold Authors

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

package debug

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/deploy/kubectl"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/runner/runcontext"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/sync"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

var (
	decodeFromYaml = scheme.Codecs.UniversalDeserializer().Decode
	encodeAsYaml   = func(o runtime.Object) ([]byte, error) {
		s := serializer.NewYAMLSerializer(serializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		if err := s.Encode(o, w); err != nil {
			return nil, err
		}
		w.Flush()
		return b.Bytes(), nil
	}
)

// ApplyDebuggingTransforms applies language-platform-specific transforms to a list of manifests.
func ApplyDebuggingTransforms(l kubectl.ManifestList, builds []build.Artifact, insecureRegistries map[string]bool) (kubectl.ManifestList, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retriever := func(image string) (imageConfiguration, error) {
		if artifact := findArtifact(image, builds); artifact != nil {
			return retrieveImageConfiguration(ctx, artifact, insecureRegistries)
		}
		return imageConfiguration{}, errors.Errorf("no build artifact for %q", image)
	}
	return applyDebuggingTransforms(l, retriever)
}

func applyDebuggingTransforms(l kubectl.ManifestList, retriever configurationRetriever) (kubectl.ManifestList, error) {
	var updated kubectl.ManifestList
	for _, manifest := range l {
		obj, _, err := decodeFromYaml(manifest, nil, nil)
		if err != nil {
			logrus.Debugf("Unable to interpret manifest for debugging: %v\n", err)
		} else if transformManifest(obj, retriever) {
			manifest, err = encodeAsYaml(obj)
			if err != nil {
				return nil, errors.Wrap(err, "marshalling yaml")
			}
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.Debugln("Applied debugging transform:\n", string(manifest))
			}
		}
		updated = append(updated, manifest)
	}

	return updated, nil
}

// findArtifact finds the corresponding artifact for the given image
func findArtifact(image string, builds []build.Artifact) *build.Artifact {
	for _, artifact := range builds {
		if image == artifact.ImageName || image == artifact.Tag {
			logrus.Debugf("Found artifact for image %q", image)
			return &artifact
		}
	}
	return nil
}

// retrieveImageConfiguration retrieves the image container configuration for
// the given build artifact
func retrieveImageConfiguration(ctx context.Context, artifact *build.Artifact, insecureRegistries map[string]bool) (imageConfiguration, error) {
	// TODO: use the proper RunContext
	apiClient, err := docker.NewAPIClient(&runcontext.RunContext{
		InsecureRegistries: insecureRegistries,
	})
	if err != nil {
		return imageConfiguration{}, errors.Wrap(err, "could not connect to local docker daemon")
	}

	// the apiClient will go to the remote registry if local docker daemon is not available
	manifest, err := apiClient.ConfigFile(ctx, artifact.Tag)
	if err != nil {
		logrus.Debugf("Error retrieving image manifest for %v: %v", artifact.Tag, err)
		return imageConfiguration{}, errors.Wrapf(err, "retrieving image config for %q", artifact.Tag)
	}

	appRoots := determineAppRoots(artifact, insecureRegistries)

	config := manifest.Config
	logrus.Debugf("Retrieved local image configuration for %v: %v", artifact.Tag, config)
	return imageConfiguration{
		artifact:   artifact.ImageName,
		appRoots:   appRoots,
		env:        envAsMap(config.Env),
		entrypoint: config.Entrypoint,
		arguments:  config.Cmd,
		labels:     config.Labels,
		workingDir: config.WorkingDir,
	}, nil
}

func determineAppRoots(artifact *build.Artifact, insecureRegistries map[string]bool) []string {
	// TODO: hook in artifact-type specific means of determining the application root
	// For example, jib typically uses `/app`, and buildpacks uses $CNB_APP_DIR (default `/workspace`) 
	 
	// syncMap is a map of local source locations to remote destinations (possibly multiple)
	syncMap, err := sync.SyncMap(&artifact.Config, insecureRegistries)
	if err != nil {
		logrus.Warnf("unable to obtain sync map for %s: %v", artifact.ImageName, err)
		return nil
	}
	// TODO: we could try to process the sync-map to build a list of local -> remote roots
	// but it's significantly more complex'
	remoteFiles := []string{}
	for _, r := range syncMap {
		for _, rf := range r {
			remoteFiles = append(remoteFiles, rf)
		}
	}
	// TODO: we assume containers are linux-based
	remoteRoots := util.CommonRoots(remoteFiles, 1, "linux")
	return remoteRoots
}

// envAsMap turns an array of environment "NAME=value" strings into a map
func envAsMap(env []string) map[string]string {
	result := make(map[string]string)
	for _, pair := range env {
		s := strings.SplitN(pair, "=", 2)
		result[s[0]] = s[1]
	}
	return result
}
