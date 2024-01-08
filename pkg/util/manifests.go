/*

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

package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

type manifest struct {
	Raw []byte
}

// UnmarshalJSON unmarshals bytes of single kubernetes object to manifest.
func (m *manifest) UnmarshalJSON(in []byte) error {
	if m == nil {
		return errors.New("manifest: UnmarshalJSON on nil pointer")
	}

	// This happens when marshaling
	// <yaml>
	// ---	(this between two `---`)
	// ---
	// <yaml>
	if bytes.Equal(in, []byte("null")) {
		m.Raw = nil
		return nil
	}

	m.Raw = append(m.Raw[0:0], in...)
	return nil
}

// ParseManifests parses a YAML or JSON document that may contain one or more
// kubernetes resources.
func ParseManifests(filename string, r io.Reader) ([]manifest, error) {
	d := yamlutil.NewYAMLOrJSONDecoder(r, 1024)
	var manifests []manifest
	for {
		m := manifest{}
		if err := d.Decode(&m); err != nil {
			if err == io.EOF {
				return manifests, nil
			}
			return manifests, fmt.Errorf("error parsing %q: %w", filename, err)
		}
		m.Raw = bytes.TrimSpace(m.Raw)
		if len(m.Raw) == 0 || bytes.Equal(m.Raw, []byte("null")) {
			continue
		}
		manifests = append(manifests, m)
	}
}

func ListFiles(dirPaths string) ([]string, error) {
	dirs := strings.Split(dirPaths, ",")
	return ListFilesFromMultiplePaths(dirs)
}

func ListFilesFromMultiplePaths(dirPaths []string) ([]string, error) {
	results := []string{}
	for _, dir := range dirPaths {
		err := filepath.WalkDir(dir,
			func(path string, info os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				results = append(results, path)
				return nil
			})
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// When no MCPs are present, it is desirable to still generate the relevant files based off of the standard
// MCP labels and node selectors. Here we create the default `master` and `worker` MCP with their respective base
// Labels and NodeSelector Labels, this allows any resource such as PAO's to utalize the deault during bootstrap
// rendoring.
func CreateLabeledDefaultMCPManifests() []*mcfgv1.MachineConfigPool {
	const (
		master             = "master"
		worker             = "worker"
		labelPrefix        = "pools.operator.machineconfiguration.openshift.io/"
		masterLabels       = labelPrefix + master
		workerLabels       = labelPrefix + worker
		masterNodeSelector = components.NodeRoleLabelPrefix + master
		workerNodeSelector = components.NodeRoleLabelPrefix + worker
	)
	return []*mcfgv1.MachineConfigPool{
		{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					masterLabels: "",
				},
				Name: master,
			},
			Spec: mcfgv1.MachineConfigPoolSpec{
				NodeSelector: v1.AddLabelToSelector(&v1.LabelSelector{}, masterNodeSelector, ""),
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					workerLabels: "",
				},
				Name: worker,
			},
			Spec: mcfgv1.MachineConfigPoolSpec{
				NodeSelector: v1.AddLabelToSelector(&v1.LabelSelector{}, workerNodeSelector, ""),
			},
		},
	}
}

func AddGeneratedByAnnotation(annotations map[string]string, profileName, profileNamespace string) map[string]string {
	const generatedByAnnotationKey = "performanceprofile.openshift.io/generatedby"
	if annotations == nil {
		annotations = make(map[string]string)
	}
	value := profileName
	if profileNamespace != "" {
		value = fmt.Sprintf("%s/%s", value, profileNamespace)
	}
	annotations[generatedByAnnotationKey] = value
	return annotations
}
