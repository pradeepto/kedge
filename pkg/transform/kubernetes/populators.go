/*
Copyright 2017 The Kedge Authors All rights reserved.

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

package kubernetes

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kedgeproject/kedge/pkg/spec"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	api_v1 "k8s.io/client-go/pkg/api/v1"
)

func populateProbes(c spec.Container) (spec.Container, error) {
	// check if health and liveness given together
	if c.Health != nil && (c.ReadinessProbe != nil || c.LivenessProbe != nil) {
		return c, fmt.Errorf("cannot define field 'health' and " +
			"'livenessProbe' or 'readinessProbe' together")
	}
	if c.Health != nil {
		c.LivenessProbe = c.Health
		c.ReadinessProbe = c.Health
		c.Health = nil
	}
	return c, nil
}

func searchConfigMap(cms []spec.ConfigMapMod, name string) (spec.ConfigMapMod, error) {
	for _, cm := range cms {
		if cm.Name == name {
			return cm, nil
		}
	}
	return spec.ConfigMapMod{}, fmt.Errorf("configMap %q not found", name)
}

func getSecretDataKeys(secrets []spec.SecretMod, name string) ([]string, error) {
	var dataKeys []string
	for _, secret := range secrets {
		if secret.Name == name {
			for dk := range secret.Data {
				dataKeys = append(dataKeys, dk)
			}
			for sdk := range secret.StringData {
				dataKeys = append(dataKeys, sdk)
			}
			return dataKeys, nil
		}
	}
	return nil, fmt.Errorf("secret %q not found", name)
}

func getMapKeys(m map[string]string) []string {
	var d []string
	for k := range m {
		d = append(d, k)
	}
	return d
}

func convertEnvFromToEnvs(envFrom []api_v1.EnvFromSource, cms []spec.ConfigMapMod, secrets []spec.SecretMod) ([]api_v1.EnvVar, error) {
	var envs []api_v1.EnvVar

	// we will iterate on all envFroms
	for ei, e := range envFrom {
		if e.ConfigMapRef != nil {

			cmName := e.ConfigMapRef.Name

			// see if the configMap name which is given actually exists
			cm, err := searchConfigMap(cms, cmName)
			if err != nil {
				return nil, errors.Wrapf(err, "envFrom[%d].configMapRef.name", ei)
			}
			// once that configMap is found extract all data from it and create a env out of it
			configMapKeys := getMapKeys(cm.Data)
			sort.Strings(configMapKeys)
			for _, key := range configMapKeys {
				envs = append(envs, api_v1.EnvVar{
					Name: key,
					ValueFrom: &api_v1.EnvVarSource{
						ConfigMapKeyRef: &api_v1.ConfigMapKeySelector{
							LocalObjectReference: api_v1.LocalObjectReference{
								Name: cmName,
							},
							Key: key,
						},
					},
				})
			}
		}

		if e.SecretRef != nil {
			rootSecretDataKeys, err := getSecretDataKeys(secrets, e.SecretRef.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "envFrom[%d].secretRef.name", ei)
			}

			sort.Strings(rootSecretDataKeys)
			for _, secretDataKey := range rootSecretDataKeys {
				envs = append(envs, api_v1.EnvVar{
					Name: secretDataKey,
					ValueFrom: &api_v1.EnvVarSource{
						SecretKeyRef: &api_v1.SecretKeySelector{
							LocalObjectReference: api_v1.LocalObjectReference{
								Name: e.SecretRef.Name,
							},
							Key: secretDataKey,
						},
					},
				})
			}
		}
	}
	return envs, nil
}

func populateEnvFrom(c spec.Container, cms []spec.ConfigMapMod, secrets []spec.SecretMod) (spec.Container, error) {
	// now do the env from
	envs, err := convertEnvFromToEnvs(c.EnvFrom, cms, secrets)
	if err != nil {
		return c, err
	}
	// Since we are not supporting envFrom in our generated Kubernetes
	// artifacts right now, we need to set it as nil for every container.
	// This makes sure that Kubernetes artifacts do not contain an
	// envFrom field.
	// This is safe to set since all of the data from envFrom has been
	// extracted till this point.
	c.EnvFrom = nil
	// we collect all the envs from configMap before
	// envs provided inside the container
	envs = append(envs, c.Env...)
	c.Env = envs
	return c, nil
}

func populateContainers(containers []spec.Container, cms []spec.ConfigMapMod, secrets []spec.SecretMod) ([]api_v1.Container, error) {
	var cnts []api_v1.Container

	for cn, c := range containers {
		// process health field
		c, err := populateProbes(c)
		if err != nil {
			return cnts, errors.Wrapf(err, "error converting 'health' to 'probes', app.containers[%d]", cn)
		}

		// process envFrom field
		c, err = populateEnvFrom(c, cms, secrets)
		if err != nil {
			return cnts, fmt.Errorf("error converting 'envFrom' to 'envs', app.containers[%d].%s", cn, err.Error())
		}

		// this is where we are only taking apart upstream container
		// and not our own remix of containers
		cnts = append(cnts, c.Container)
	}

	b, _ := json.MarshalIndent(cnts, "", "  ")
	log.Debugf("containers after populating health: %s", string(b))
	return cnts, nil
}
