/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"encoding/json"
	"fmt"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/spark-on-k8s-operator/pkg/apis/sparkoperator.k8s.io/v1beta1"
	"github.com/GoogleCloudPlatform/spark-on-k8s-operator/pkg/config"
)

func TestPatchSparkPod_OwnerReference(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	// Test patching a pod without existing OwnerReference and Volume.
	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(modifiedPod.OwnerReferences))

	// Test patching a pod with existing OwnerReference and Volume.
	pod.OwnerReferences = append(pod.OwnerReferences, metav1.OwnerReference{Name: "owner-reference1"})

	modifiedPod, err = getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(modifiedPod.OwnerReferences))
}

func TestPatchSparkPod_Volumes(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Volumes: []corev1.Volume{
				corev1.Volume{
					Name: "spark",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/spark",
						},
					},
				},
			},
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "spark",
							MountPath: "/mnt/spark",
						},
					},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	// Test patching a pod without existing OwnerReference and Volume.
	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(modifiedPod.Spec.Volumes))
	assert.Equal(t, app.Spec.Volumes[0], modifiedPod.Spec.Volumes[0])
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, app.Spec.Driver.VolumeMounts[0], modifiedPod.Spec.Containers[0].VolumeMounts[0])

	// Test patching a pod with existing OwnerReference and Volume.
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "volume1"})
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name: "volume1",
	})

	modifiedPod, err = getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(modifiedPod.Spec.Volumes))
	assert.Equal(t, app.Spec.Volumes[0], modifiedPod.Spec.Volumes[1])
	assert.Equal(t, 2, len(modifiedPod.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, app.Spec.Driver.VolumeMounts[0], modifiedPod.Spec.Containers[0].VolumeMounts[1])
}

func TestPatchSparkPod_Affinity(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{config.SparkRoleLabel: config.SparkDriverRole},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	// Test patching a pod with a pod Affinity.
	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, modifiedPod.Spec.Affinity != nil)
	assert.Equal(t, 1,
		len(modifiedPod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution))
	assert.Equal(t, "kubernetes.io/hostname",
		modifiedPod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey)
}

func TestPatchSparkPod_ConfigMaps(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					ConfigMaps: []v1beta1.NamePath{{Name: "foo", Path: "/path/to/foo"}},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(modifiedPod.Spec.Volumes))
	assert.Equal(t, "foo-vol", modifiedPod.Spec.Volumes[0].Name)
	assert.True(t, modifiedPod.Spec.Volumes[0].ConfigMap != nil)
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, "/path/to/foo", modifiedPod.Spec.Containers[0].VolumeMounts[0].MountPath)
}

func TestPatchSparkPod_SparkConfigMap(t *testing.T) {
	sparkConfMapName := "spark-conf"
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			SparkConfigMap: &sparkConfMapName,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(modifiedPod.Spec.Volumes))
	assert.Equal(t, config.SparkConfigMapVolumeName, modifiedPod.Spec.Volumes[0].Name)
	assert.True(t, modifiedPod.Spec.Volumes[0].ConfigMap != nil)
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, config.DefaultSparkConfDir, modifiedPod.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].Env))
	assert.Equal(t, config.DefaultSparkConfDir, modifiedPod.Spec.Containers[0].Env[0].Value)
}

func TestPatchSparkPod_HadoopConfigMap(t *testing.T) {
	hadoopConfMapName := "hadoop-conf"
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			HadoopConfigMap: &hadoopConfMapName,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(modifiedPod.Spec.Volumes))
	assert.Equal(t, config.HadoopConfigMapVolumeName, modifiedPod.Spec.Volumes[0].Name)
	assert.True(t, modifiedPod.Spec.Volumes[0].ConfigMap != nil)
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, config.DefaultHadoopConfDir, modifiedPod.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].Env))
	assert.Equal(t, config.DefaultHadoopConfDir, modifiedPod.Spec.Containers[0].Env[0].Value)
}

func TestPatchSparkPod_PrometheusConfigMaps(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Monitoring: &v1beta1.MonitoringSpec{
				Prometheus:          &v1beta1.PrometheusSpec{},
				ExposeDriverMetrics: true,
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	expectedConfigMapName := config.GetPrometheusConfigMapName(app)
	expectedVolumeName := expectedConfigMapName + "-vol"
	assert.Equal(t, 1, len(modifiedPod.Spec.Volumes))
	assert.Equal(t, expectedVolumeName, modifiedPod.Spec.Volumes[0].Name)
	assert.True(t, modifiedPod.Spec.Volumes[0].ConfigMap != nil)
	assert.Equal(t, expectedConfigMapName, modifiedPod.Spec.Volumes[0].ConfigMap.Name)
	assert.Equal(t, 1, len(modifiedPod.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, expectedVolumeName, modifiedPod.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, config.PrometheusConfigMapMountPath, modifiedPod.Spec.Containers[0].VolumeMounts[0].MountPath)
}

func TestPatchSparkPod_Tolerations(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "Key",
							Operator: "Equal",
							Value:    "Value",
							Effect:   "NoEffect",
						},
					},
				},
			},
		},
	}

	// Test patching a pod with a Toleration.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedPod, err := getModifiedPod(pod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(modifiedPod.Spec.Tolerations))
	assert.Equal(t, app.Spec.Driver.Tolerations[0], modifiedPod.Spec.Tolerations[0])
}

func TestPatchSparkPod_SecurityContext(t *testing.T) {
	var user int64 = 1000
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					SecurityContenxt: &corev1.PodSecurityContext{
						RunAsUser: &user,
					},
				},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					SecurityContenxt: &corev1.PodSecurityContext{
						RunAsUser: &user,
					},
				},
			},
		},
	}

	driverPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedDriverPod, err := getModifiedPod(driverPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, app.Spec.Driver.SecurityContenxt, modifiedDriverPod.Spec.SecurityContext)

	executorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-executor",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkExecutorRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkExecutorContainerName,
					Image: "spark-executor:latest",
				},
			},
		},
	}

	modifiedExecutorPod, err := getModifiedPod(executorPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, app.Spec.Executor.SecurityContenxt, modifiedExecutorPod.Spec.SecurityContext)
}

func TestPatchSparkPod_SchedulerName(t *testing.T) {
	var schedulerName = "another_scheduler"
	var defaultScheduler = "default-scheduler"

	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test-patch-schedulername",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					SchedulerName: &schedulerName,
				},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{},
			},
		},
	}

	driverPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			SchedulerName: defaultScheduler,
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedDriverPod, err := getModifiedPod(driverPod, app)
	if err != nil {
		t.Fatal(err)
	}
	//Driver scheduler name should be updated when specified.
	assert.Equal(t, schedulerName, modifiedDriverPod.Spec.SchedulerName)

	executorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-executor",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkExecutorRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			SchedulerName: defaultScheduler,
			Containers: []corev1.Container{
				{
					Name:  config.SparkExecutorContainerName,
					Image: "spark-executor:latest",
				},
			},
		},
	}

	modifiedExecutorPod, err := getModifiedPod(executorPod, app)
	if err != nil {
		t.Fatal(err)
	}
	//Executor scheduler name should remain the same as before when not specified in SparkApplicationSpec
	assert.Equal(t, defaultScheduler, modifiedExecutorPod.Spec.SchedulerName)
}

func TestPatchSparkPod_Sidecars(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					Sidecars: []corev1.Container{
						{
							Name:  "sidecar1",
							Image: "sidecar1:latest",
						},
						{
							Name:  "sidecar2",
							Image: "sidecar2:latest",
						},
					},
				},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					Sidecars: []corev1.Container{
						{
							Name:  "sidecar1",
							Image: "sidecar1:latest",
						},
						{
							Name:  "sidecar2",
							Image: "sidecar2:latest",
						},
					},
				},
			},
		},
	}

	driverPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedDriverPod, err := getModifiedPod(driverPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 3, len(modifiedDriverPod.Spec.Containers))
	assert.Equal(t, "sidecar1", modifiedDriverPod.Spec.Containers[1].Name)
	assert.Equal(t, "sidecar2", modifiedDriverPod.Spec.Containers[2].Name)

	executorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-executor",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkExecutorRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkExecutorContainerName,
					Image: "spark-executor:latest",
				},
			},
		},
	}

	modifiedExecutorPod, err := getModifiedPod(executorPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 3, len(modifiedExecutorPod.Spec.Containers))
	assert.Equal(t, "sidecar1", modifiedExecutorPod.Spec.Containers[1].Name)
	assert.Equal(t, "sidecar2", modifiedExecutorPod.Spec.Containers[2].Name)
}

func TestPatchSparkPod_DNSConfig(t *testing.T) {
	aVal := "5"
	sampleDNSConfig := &corev1.PodDNSConfig{
		Nameservers: []string{"8.8.8.8", "4.4.4.4"},
		Searches:    []string{"svc.cluster.local", "cluster.local"},
		Options: []corev1.PodDNSConfigOption{
			corev1.PodDNSConfigOption{Name: "ndots", Value: &aVal},
		},
	}

	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{DNSConfig: sampleDNSConfig},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{DNSConfig: sampleDNSConfig},
			},
		},
	}

	driverPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedDriverPod, err := getModifiedPod(driverPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, modifiedDriverPod.Spec.DNSConfig)
	assert.Equal(t, sampleDNSConfig, modifiedDriverPod.Spec.DNSConfig)

	executorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-executor",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkExecutorRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkExecutorContainerName,
					Image: "spark-executor:latest",
				},
			},
		},
	}

	modifiedExecutorPod, err := getModifiedPod(executorPod, app)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, modifiedExecutorPod.Spec.DNSConfig)
	assert.Equal(t, sampleDNSConfig, modifiedExecutorPod.Spec.DNSConfig)

}

func TestPatchSparkPod_NodeSector(t *testing.T) {
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					NodeSelector: map[string]string{"disk": "ssd", "secondkey": "secondvalue"},
				},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{
					NodeSelector: map[string]string{"nodeType": "gpu", "secondkey": "secondvalue"},
				},
			},
		},
	}

	driverPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-driver",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkDriverRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkDriverContainerName,
					Image: "spark-driver:latest",
				},
			},
		},
	}

	modifiedDriverPod, err := getModifiedPod(driverPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(modifiedDriverPod.Spec.NodeSelector))
	assert.Equal(t, "ssd", modifiedDriverPod.Spec.NodeSelector["disk"])
	assert.Equal(t, "secondvalue", modifiedDriverPod.Spec.NodeSelector["secondkey"])

	executorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-executor",
			Labels: map[string]string{
				config.SparkRoleLabel:               config.SparkExecutorRole,
				config.LaunchedBySparkOperatorLabel: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  config.SparkExecutorContainerName,
					Image: "spark-executor:latest",
				},
			},
		},
	}

	modifiedExecutorPod, err := getModifiedPod(executorPod, app)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(modifiedExecutorPod.Spec.NodeSelector))
	assert.Equal(t, "gpu", modifiedExecutorPod.Spec.NodeSelector["nodeType"])
	assert.Equal(t, "secondvalue", modifiedExecutorPod.Spec.NodeSelector["secondkey"])
}

func TestPatchSparkPod_GPU(t *testing.T) {
	cpuLimit := int64(10)
	cpuRequest := int64(5)

	type testcase struct {
		gpuSpec     *v1beta1.GPUSpec
		cpuLimits   *int64
		cpuRequests *int64
	}

	getResourceRequirements := func(test testcase) *corev1.ResourceRequirements {
		if test.cpuLimits == nil && test.cpuRequests == nil {
			return nil
		}
		ret := corev1.ResourceRequirements{}
		if test.cpuLimits != nil {
			ret.Limits = corev1.ResourceList{}
			ret.Limits[corev1.ResourceCPU] = *resource.NewQuantity(*test.cpuLimits, resource.DecimalSI)
		}
		if test.cpuRequests != nil {
			ret.Requests = corev1.ResourceList{}
			ret.Requests[corev1.ResourceCPU] = *resource.NewQuantity(*test.cpuRequests, resource.DecimalSI)
		}
		return &ret
	}

	assertFn := func(t *testing.T, modifiedPod *corev1.Pod, test testcase) {
		// check GPU Limits
		if test.gpuSpec != nil && test.gpuSpec.Name != "" && test.gpuSpec.Quantity > 0 {
			quantity := modifiedPod.Spec.Containers[0].Resources.Limits[corev1.ResourceName(test.gpuSpec.Name)]
			count, succeed := (&quantity).AsInt64()
			if succeed != true {
				t.Fatal(fmt.Errorf("value cannot be represented in an int64 OR would result in a loss of precision."))
			}
			assert.Equal(t, test.gpuSpec.Quantity, count)
		}

		// check CPU Requests
		if test.cpuRequests != nil {
			quantity := modifiedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
			count, succeed := (&quantity).AsInt64()
			if succeed != true {
				t.Fatal(fmt.Errorf("value cannot be represented in an int64 OR would result in a loss of precision."))
			}
			assert.Equal(t, *test.cpuRequests, count)
		}

		// check CPU Limits
		if test.cpuLimits != nil {
			quantity := modifiedPod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU]
			count, succeed := (&quantity).AsInt64()
			if succeed != true {
				t.Fatal(fmt.Errorf("value cannot be represented in an int64 OR would result in a loss of precision."))
			}
			assert.Equal(t, *test.cpuLimits, count)
		}
	}
	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{},
			},
		},
	}
	tests := []testcase{
		{
			nil,
			nil,
			nil,
		},
		{
			nil,
			&cpuLimit,
			nil,
		},
		{
			nil,
			nil,
			&cpuRequest,
		},
		{
			nil,
			&cpuLimit,
			&cpuRequest,
		},
		{
			&v1beta1.GPUSpec{},
			nil,
			nil,
		},
		{
			&v1beta1.GPUSpec{},
			&cpuLimit,
			nil,
		},
		{
			&v1beta1.GPUSpec{},
			nil,
			&cpuRequest,
		},
		{
			&v1beta1.GPUSpec{},
			&cpuLimit,
			&cpuRequest,
		},
		{
			&v1beta1.GPUSpec{"example.com/gpu", 1},
			nil,
			nil,
		},
		{
			&v1beta1.GPUSpec{"example.com/gpu", 1},
			&cpuLimit,
			nil,
		},
		{
			&v1beta1.GPUSpec{"example.com/gpu", 1},
			nil,
			&cpuRequest,
		},
		{
			&v1beta1.GPUSpec{"example.com/gpu", 1},
			&cpuLimit,
			&cpuRequest,
		},
	}

	for _, test := range tests {
		app.Spec.Driver.GPU = test.gpuSpec
		app.Spec.Executor.GPU = test.gpuSpec
		driverPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "spark-driver",
				Labels: map[string]string{
					config.SparkRoleLabel:               config.SparkDriverRole,
					config.LaunchedBySparkOperatorLabel: "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  config.SparkDriverContainerName,
						Image: "spark-driver:latest",
					},
				},
			},
		}

		if getResourceRequirements(test) != nil {
			driverPod.Spec.Containers[0].Resources = *getResourceRequirements(test)
		}
		modifiedDriverPod, err := getModifiedPod(driverPod, app)
		if err != nil {
			t.Fatal(err)
		}

		assertFn(t, modifiedDriverPod, test)
		executorPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "spark-executor",
				Labels: map[string]string{
					config.SparkRoleLabel:               config.SparkExecutorRole,
					config.LaunchedBySparkOperatorLabel: "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  config.SparkExecutorContainerName,
						Image: "spark-executor:latest",
					},
				},
			},
		}
		if getResourceRequirements(test) != nil {
			executorPod.Spec.Containers[0].Resources = *getResourceRequirements(test)
		}
		modifiedExecutorPod, err := getModifiedPod(executorPod, app)
		if err != nil {
			t.Fatal(err)
		}
		assertFn(t, modifiedExecutorPod, test)
	}
}

func TestPatchSparkPod_HostNetwork(t *testing.T) {
	var hostNetwork = true
	var defaultNetwork = false

	app := &v1beta1.SparkApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spark-test-hostNetwork",
			UID:  "spark-test-1",
		},
		Spec: v1beta1.SparkApplicationSpec{
			Driver: v1beta1.DriverSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{},
			},
			Executor: v1beta1.ExecutorSpec{
				SparkPodSpec: v1beta1.SparkPodSpec{},
			},
		},
	}

	tests := []*bool{
		nil,
		&defaultNetwork,
		&hostNetwork,
	}

	for _, test := range tests {
		app.Spec.Driver.HostNetwork = test
		app.Spec.Executor.HostNetwork = test
		driverPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "spark-driver",
				Labels: map[string]string{
					config.SparkRoleLabel:               config.SparkDriverRole,
					config.LaunchedBySparkOperatorLabel: "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  config.SparkDriverContainerName,
						Image: "spark-driver:latest",
					},
				},
			},
		}

		modifiedDriverPod, err := getModifiedPod(driverPod, app)
		if err != nil {
			t.Fatal(err)
		}
		if test == nil || *test == false {
			assert.Equal(t, false, modifiedDriverPod.Spec.HostNetwork)
		} else {
			assert.Equal(t, true, modifiedDriverPod.Spec.HostNetwork)
			assert.Equal(t, corev1.DNSClusterFirstWithHostNet, modifiedDriverPod.Spec.DNSPolicy)
		}
		executorPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "spark-executor",
				Labels: map[string]string{
					config.SparkRoleLabel:               config.SparkExecutorRole,
					config.LaunchedBySparkOperatorLabel: "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  config.SparkExecutorContainerName,
						Image: "spark-executor:latest",
					},
				},
			},
		}

		modifiedExecutorPod, err := getModifiedPod(executorPod, app)
		if err != nil {
			t.Fatal(err)
		}
		if test == nil || *test == false {
			assert.Equal(t, false, modifiedExecutorPod.Spec.HostNetwork)
		} else {
			assert.Equal(t, true, modifiedExecutorPod.Spec.HostNetwork)
			assert.Equal(t, corev1.DNSClusterFirstWithHostNet, modifiedExecutorPod.Spec.DNSPolicy)
		}
	}
}

func getModifiedPod(pod *corev1.Pod, app *v1beta1.SparkApplication) (*corev1.Pod, error) {
	patchOps := patchSparkPod(pod, app)
	patchBytes, err := json.Marshal(patchOps)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, err
	}

	original, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	modified, err := patch.Apply(original)
	if err != nil {
		return nil, err
	}
	modifiedPod := &corev1.Pod{}
	if err := json.Unmarshal(modified, modifiedPod); err != nil {
		return nil, err
	}

	return modifiedPod, nil
}
