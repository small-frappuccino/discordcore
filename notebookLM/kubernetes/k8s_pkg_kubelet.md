# Domain Architecture: pkg/kubelet

## Layout Topology
```text
pkg/kubelet/
├── allocation
│   ├── state
│   │   ├── checkpoint.go
│   │   ├── state.go
│   │   ├── state_checkpoint.go
│   │   └── state_mem.go
│   ├── OWNERS
│   ├── allocation_manager.go
│   ├── doc.go
│   ├── features_linux.go
│   ├── features_unsupported.go
│   ├── features_windows.go
│   └── handlers.go
├── apis
│   ├── config
│   │   ├── fuzzer
│   │   │   └── fuzzer.go
│   │   ├── scheme
│   │   │   └── scheme.go
│   │   ├── v1
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   ├── zz_generated.conversion.go
│   │   │   └── zz_generated.defaults.go
│   │   ├── v1alpha1
│   │   │   ├── conversion.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   ├── zz_generated.conversion.go
│   │   │   └── zz_generated.defaults.go
│   │   ├── v1beta1
│   │   │   ├── conversion.go
│   │   │   ├── defaults.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   ├── zz_generated.conversion.go
│   │   │   └── zz_generated.defaults.go
│   │   ├── validation
│   │   │   ├── validation.go
│   │   │   ├── validation_linux.go
│   │   │   ├── validation_others.go
│   │   │   ├── validation_reserved_memory.go
│   │   │   └── validation_windows.go
│   │   ├── OWNERS
│   │   ├── doc.go
│   │   ├── helpers.go
│   │   ├── register.go
│   │   ├── types.go
│   │   └── zz_generated.deepcopy.go
│   ├── grpc
│   │   └── ratelimit.go
│   ├── podresources
│   │   ├── testing
│   │   │   └── mocks.go
│   │   ├── OWNERS
│   │   ├── client.go
│   │   ├── constants.go
│   │   ├── server_v1.go
│   │   ├── server_v1alpha1.go
│   │   └── types.go
│   └── pods
│       ├── constants.go
│       └── server.go
├── cadvisor
│   ├── testing
│   │   ├── cadvisor_fake.go
│   │   └── mocks.go
│   ├── cadvisor_linux.go
│   ├── cadvisor_unsupported.go
│   ├── cadvisor_windows.go
│   ├── doc.go
│   ├── helpers_linux.go
│   ├── helpers_unsupported.go
│   ├── types.go
│   └── util.go
├── certificate
│   ├── bootstrap
│   │   └── bootstrap.go
│   ├── OWNERS
│   ├── kubelet.go
│   └── transport.go
├── checkpointmanager
│   ├── checksum
│   │   └── checksum.go
│   ├── errors
│   │   └── errors.go
│   ├── testing
│   │   ├── example_checkpoint_formats
│   │   │   └── v1
│   │   │       └── types.go
│   │   └── util.go
│   ├── README.md
│   └── checkpoint_manager.go
├── client
│   └── kubelet_client.go
├── clustertrustbundle
│   └── clustertrustbundle_manager.go
├── cm
│   ├── admission
│   │   └── errors.go
│   ├── containermap
│   │   └── container_map.go
│   ├── cpumanager
│   │   ├── state
│   │   │   ├── testing
│   │   │   │   └── util.go
│   │   │   ├── checkpoint.go
│   │   │   ├── state.go
│   │   │   ├── state_checkpoint.go
│   │   │   └── state_mem.go
│   │   ├── topology
│   │   │   ├── alignment.go
│   │   │   ├── doc.go
│   │   │   └── topology.go
│   │   ├── OWNERS
│   │   ├── cpu_assignment.go
│   │   ├── cpu_manager.go
│   │   ├── cpu_manager_others.go
│   │   ├── cpu_manager_windows.go
│   │   ├── fake_cpu_manager.go
│   │   ├── policy.go
│   │   ├── policy_none.go
│   │   ├── policy_options.go
│   │   └── policy_static.go
│   ├── devicemanager
│   │   ├── checkpoint
│   │   │   └── checkpoint.go
│   │   ├── plugin
│   │   │   └── v1beta1
│   │   │       ├── api.go
│   │   │       ├── client.go
│   │   │       ├── handler.go
│   │   │       ├── server.go
│   │   │       └── stub.go
│   │   ├── OWNERS
│   │   ├── endpoint.go
│   │   ├── manager.go
│   │   ├── pod_devices.go
│   │   ├── topology_hints.go
│   │   └── types.go
│   ├── dra
│   │   ├── plugin
│   │   │   ├── dra_plugin.go
│   │   │   ├── dra_plugin_manager.go
│   │   │   └── types.go
│   │   ├── state
│   │   │   ├── checkpoint.go
│   │   │   ├── checkpointer.go
│   │   │   ├── state.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── OWNERS
│   │   ├── claiminfo.go
│   │   ├── healthinfo.go
│   │   ├── manager.go
│   │   ├── types.go
│   │   └── zz_generated.deepcopy.go
│   ├── memorymanager
│   │   ├── state
│   │   │   ├── checkpoint.go
│   │   │   ├── state.go
│   │   │   ├── state_checkpoint.go
│   │   │   └── state_mem.go
│   │   ├── fake_memory_manager.go
│   │   ├── memory_manager.go
│   │   ├── policy.go
│   │   ├── policy_best_effort.go
│   │   ├── policy_none.go
│   │   └── policy_static.go
│   ├── qos
│   │   ├── doc.go
│   │   ├── helpers.go
│   │   └── types.go
│   ├── resourceupdates
│   │   └── updates.go
│   ├── testing
│   │   └── mocks.go
│   ├── topologymanager
│   │   ├── bitmask
│   │   │   └── bitmask.go
│   │   ├── OWNERS
│   │   ├── fake_topology_manager.go
│   │   ├── numa_info.go
│   │   ├── policy.go
│   │   ├── policy_best_effort.go
│   │   ├── policy_none.go
│   │   ├── policy_options.go
│   │   ├── policy_restricted.go
│   │   ├── policy_single_numa_node.go
│   │   ├── scope.go
│   │   ├── scope_container.go
│   │   ├── scope_none.go
│   │   ├── scope_pod.go
│   │   └── topology_manager.go
│   ├── util
│   │   ├── cgroups_linux.go
│   │   └── cgroups_unsupported.go
│   ├── OWNERS
│   ├── cgroup_manager_linux.go
│   ├── cgroup_manager_unsupported.go
│   ├── cgroup_v1_manager_linux.go
│   ├── cgroup_v2_manager_linux.go
│   ├── container_manager.go
│   ├── container_manager_linux.go
│   ├── container_manager_stub.go
│   ├── container_manager_unsupported.go
│   ├── container_manager_windows.go
│   ├── doc.go
│   ├── fake_container_manager.go
│   ├── fake_internal_container_lifecycle.go
│   ├── fake_pod_container_manager.go
│   ├── helpers.go
│   ├── helpers_linux.go
│   ├── helpers_unsupported.go
│   ├── internal_container_lifecycle.go
│   ├── internal_container_lifecycle_linux.go
│   ├── internal_container_lifecycle_unsupported.go
│   ├── internal_container_lifecycle_windows.go
│   ├── node_container_manager_linux.go
│   ├── pod_container_manager_linux.go
│   ├── pod_container_manager_stub.go
│   ├── qos_container_manager_linux.go
│   └── types.go
├── config
│   ├── apiserver.go
│   ├── common.go
│   ├── config.go
│   ├── doc.go
│   ├── file.go
│   ├── file_linux.go
│   ├── file_unsupported.go
│   ├── http.go
│   ├── mux.go
│   └── sources.go
├── configmap
│   ├── configmap_manager.go
│   └── fake_manager.go
├── container
│   ├── testing
│   │   ├── fake_cache.go
│   │   ├── fake_ready_provider.go
│   │   ├── fake_runtime.go
│   │   ├── fake_runtime_helper.go
│   │   ├── mockdirentry.go
│   │   ├── mocks.go
│   │   └── os.go
│   ├── cache.go
│   ├── container_gc.go
│   ├── helpers.go
│   ├── os.go
│   ├── ref.go
│   ├── runtime.go
│   ├── runtime_cache.go
│   ├── runtime_cache_fake.go
│   └── sync_result.go
├── envvars
│   ├── doc.go
│   └── envvars.go
├── events
│   ├── event.go
│   └── resize.go
├── eviction
│   ├── api
│   │   └── types.go
│   ├── defaults_linux.go
│   ├── defaults_others.go
│   ├── defaults_windows.go
│   ├── doc.go
│   ├── eviction_manager.go
│   ├── helpers.go
│   ├── helpers_others.go
│   ├── helpers_windows.go
│   ├── memory_threshold_notifier.go
│   ├── memory_threshold_notifier_others.go
│   ├── memory_threshold_notifier_windows.go
│   ├── threshold_notifier_linux.go
│   ├── threshold_notifier_unsupported.go
│   └── types.go
├── images
│   ├── pullmanager
│   │   ├── doc.go
│   │   ├── fs_pullrecords.go
│   │   ├── image_pull_manager.go
│   │   ├── image_pull_policies.go
│   │   ├── interfaces.go
│   │   ├── locks.go
│   │   ├── mem_pullrecords.go
│   │   ├── metrics.go
│   │   └── noop_pull_manager.go
│   ├── doc.go
│   ├── helpers.go
│   ├── image_gc_manager.go
│   ├── image_manager.go
│   ├── metrics.go
│   ├── puller.go
│   └── types.go
├── kubeletconfig
│   ├── configfiles
│   │   └── configfiles.go
│   ├── util
│   │   └── codec
│   │       └── codec.go
│   ├── OWNERS
│   ├── defaults.go
│   └── types.go
├── kuberuntime
│   ├── util
│   │   └── util.go
│   ├── convert.go
│   ├── doc.go
│   ├── fake_kuberuntime_manager.go
│   ├── helpers.go
│   ├── helpers_linux.go
│   ├── helpers_unsupported.go
│   ├── instrumented_services.go
│   ├── kuberuntime_container.go
│   ├── kuberuntime_container_linux.go
│   ├── kuberuntime_container_unsupported.go
│   ├── kuberuntime_container_windows.go
│   ├── kuberuntime_gc.go
│   ├── kuberuntime_image.go
│   ├── kuberuntime_logs.go
│   ├── kuberuntime_manager.go
│   ├── kuberuntime_sandbox.go
│   ├── kuberuntime_sandbox_linux.go
│   ├── kuberuntime_sandbox_unsupported.go
│   ├── kuberuntime_sandbox_windows.go
│   ├── kuberuntime_termination_order.go
│   ├── labels.go
│   ├── legacy.go
│   ├── security_context.go
│   ├── security_context_others.go
│   └── security_context_windows.go
├── lifecycle
│   ├── admission_failure_handler_stub.go
│   ├── doc.go
│   ├── features_linux.go
│   ├── features_unsupported.go
│   ├── features_windows.go
│   ├── handlers.go
│   ├── interfaces.go
│   └── predicate.go
├── logs
│   ├── container_log_manager.go
│   └── container_log_manager_stub.go
├── metrics
│   ├── collectors
│   │   ├── cri_metrics.go
│   │   ├── log_metrics.go
│   │   ├── podcertificate_metrics.go
│   │   ├── resource_metrics.go
│   │   └── volume_stats.go
│   ├── OWNERS
│   └── metrics.go
├── network
│   ├── dns
│   │   ├── OWNERS
│   │   ├── dns.go
│   │   ├── dns_other.go
│   │   └── dns_windows.go
│   └── OWNERS
├── nodeshutdown
│   ├── systemd
│   │   ├── doc.go
│   │   ├── inhibit_linux.go
│   │   └── inhibit_others.go
│   ├── nodeshutdown_manager.go
│   ├── nodeshutdown_manager_linux.go
│   ├── nodeshutdown_manager_others.go
│   ├── nodeshutdown_manager_windows.go
│   └── storage.go
├── nodestatus
│   └── setters.go
├── oom
│   ├── oom_watcher_linux.go
│   ├── oom_watcher_unsupported.go
│   └── types.go
├── pleg
│   ├── doc.go
│   ├── evented.go
│   ├── generic.go
│   └── pleg.go
├── pluginmanager
│   ├── cache
│   │   ├── actual_state_of_world.go
│   │   ├── desired_state_of_world.go
│   │   └── types.go
│   ├── metrics
│   │   └── metrics.go
│   ├── operationexecutor
│   │   ├── operation_executor.go
│   │   └── operation_generator.go
│   ├── pluginwatcher
│   │   ├── example_plugin_apis
│   │   │   ├── v1beta1
│   │   │   │   ├── api.pb.go
│   │   │   │   ├── api.proto
│   │   │   │   └── api_grpc.pb.go
│   │   │   └── v1beta2
│   │   │       ├── api.pb.go
│   │   │       ├── api.proto
│   │   │       └── api_grpc.pb.go
│   │   ├── README.md
│   │   ├── example_handler.go
│   │   ├── example_plugin.go
│   │   ├── plugin_watcher.go
│   │   ├── plugin_watcher_others.go
│   │   └── plugin_watcher_windows.go
│   ├── reconciler
│   │   └── reconciler.go
│   ├── OWNERS
│   └── plugin_manager.go
├── pod
│   ├── testing
│   │   ├── fake_mirror_client.go
│   │   └── mocks.go
│   ├── mirror_client.go
│   └── pod_manager.go
├── podcertificate
│   └── podcertificatemanager.go
├── preemption
│   └── preemption.go
├── prober
│   ├── results
│   │   └── results_manager.go
│   ├── testing
│   │   └── fake_manager.go
│   ├── prober.go
│   ├── prober_manager.go
│   └── worker.go
├── qos
│   ├── doc.go
│   ├── helpers.go
│   └── policy.go
├── runtimeclass
│   ├── testing
│   │   └── fake_manager.go
│   └── runtimeclass_manager.go
├── secret
│   ├── fake_manager.go
│   └── secret_manager.go
├── server
│   ├── metrics
│   │   └── metrics.go
│   ├── stats
│   │   ├── testing
│   │   │   └── mocks.go
│   │   ├── doc.go
│   │   ├── fs_resource_analyzer.go
│   │   ├── handler.go
│   │   ├── resource_analyzer.go
│   │   ├── summary.go
│   │   ├── summary_sys_containers.go
│   │   ├── summary_sys_containers_windows.go
│   │   └── volume_stat_calculator.go
│   ├── OWNERS
│   ├── auth.go
│   ├── doc.go
│   └── server.go
├── stats
│   ├── pidlimit
│   │   ├── pidlimit.go
│   │   ├── pidlimit_linux.go
│   │   └── pidlimit_unsupported.go
│   ├── OWNERS
│   ├── cadvisor_stats_provider.go
│   ├── cri_stats_provider.go
│   ├── cri_stats_provider_linux.go
│   ├── cri_stats_provider_others.go
│   ├── cri_stats_provider_windows.go
│   ├── helper.go
│   ├── host_stats_provider.go
│   ├── host_stats_provider_fake.go
│   └── provider.go
├── status
│   ├── testing
│   │   ├── fake_pod_deletion_safety.go
│   │   └── mocks.go
│   ├── OWNERS
│   ├── generate.go
│   └── status_manager.go
├── sysctl
│   ├── allowlist.go
│   ├── safe_sysctls.go
│   └── util.go
├── token
│   ├── OWNERS
│   └── token_manager.go
├── types
│   ├── constants.go
│   ├── doc.go
│   ├── pod_status.go
│   ├── pod_update.go
│   └── types.go
├── userns
│   ├── types.go
│   ├── userns_manager.go
│   └── userns_manager_windows.go
├── util
│   ├── cache
│   │   └── object_cache.go
│   ├── env
│   │   └── env_util.go
│   ├── format
│   │   └── pod.go
│   ├── ioutils
│   │   └── ioutils.go
│   ├── manager
│   │   ├── cache_based_manager.go
│   │   ├── manager.go
│   │   └── watch_based_manager.go
│   ├── queue
│   │   └── work_queue.go
│   ├── sliceutils
│   │   └── sliceutils.go
│   ├── store
│   │   ├── doc.go
│   │   ├── filestore.go
│   │   └── store.go
│   ├── swap
│   │   └── swap_util.go
│   ├── boottime_util_darwin.go
│   ├── boottime_util_freebsd.go
│   ├── boottime_util_linux.go
│   ├── doc.go
│   ├── node_startup_latency_tracker.go
│   ├── nodelease.go
│   ├── pod_startup_latency_tracker.go
│   ├── util.go
│   ├── util_linux.go
│   ├── util_others.go
│   ├── util_unix.go
│   ├── util_unsupported.go
│   └── util_windows.go
├── volumemanager
│   ├── cache
│   │   ├── actual_state_of_world.go
│   │   ├── desired_state_of_wold_selinux_metrics.go
│   │   └── desired_state_of_world.go
│   ├── metrics
│   │   └── metrics.go
│   ├── populator
│   │   └── desired_state_of_world_populator.go
│   ├── reconciler
│   │   ├── reconciler.go
│   │   ├── reconciler_common.go
│   │   ├── reconstruct.go
│   │   └── reconstruct_common.go
│   ├── OWNERS
│   ├── volume_manager.go
│   └── volume_manager_fake.go
├── watchdog
│   ├── types.go
│   ├── watchdog_linux.go
│   └── watchdog_unsupported.go
├── winstats
│   ├── cpu_topology.go
│   ├── doc.go
│   ├── network_stats.go
│   ├── perfcounter_nodestats_windows.go
│   ├── perfcounters.go
│   ├── version.go
│   └── winstats.go
├── OWNERS
├── active_deadline.go
├── doc.go
├── errors.go
├── kubelet.go
├── kubelet_getters.go
├── kubelet_linux.go
├── kubelet_network.go
├── kubelet_network_linux.go
├── kubelet_network_others.go
├── kubelet_node_declared_features.go
├── kubelet_node_status.go
├── kubelet_node_status_others.go
├── kubelet_node_status_windows.go
├── kubelet_nodecache.go
├── kubelet_others.go
├── kubelet_pods.go
├── kubelet_resources.go
├── kubelet_server_journal.go
├── kubelet_server_journal_linux.go
├── kubelet_server_journal_others.go
├── kubelet_server_journal_windows.go
├── kubelet_volumes.go
├── pod_container_deletor.go
├── pod_workers.go
├── reason_cache.go
├── runtime.go
└── volume_host.go
```

## Source Stream Aggregation

// === FILE: references!/kubernetes/pkg/kubelet/cm/cpumanager/cpu_manager.go ===
```go
/*
Copyright 2017 The Kubernetes Authors.

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

package cpumanager

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-logr/logr"
	cadvisorapi "github.com/google/cadvisor/info/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	resourcehelper "k8s.io/component-helpers/resource"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	kubefeatures "k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet/cm/containermap"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/state"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	cmqos "k8s.io/kubernetes/pkg/kubelet/cm/qos"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	"k8s.io/kubernetes/pkg/kubelet/config"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/status"
	"k8s.io/utils/cpuset"
)

// ActivePodsFunc is a function that returns a list of pods to reconcile.
type ActivePodsFunc func() []*v1.Pod

type runtimeService interface {
	UpdateContainerResources(ctx context.Context, id string, resources *runtimeapi.ContainerResources) error
}

type policyName string

// cpuManagerStateFileName is the file name where cpu manager stores its state
const cpuManagerStateFileName = "cpu_manager_state"

// Manager interface provides methods for Kubelet to manage pod cpus.
type Manager interface {
	// Start is called during Kubelet initialization.
	// Start takes a `Context` because it may possibly spin the reconcileState helper, which in turn
	// needs to update container state, which takes a context.
	Start(ctx context.Context, activePods ActivePodsFunc, sourcesReady config.SourcesReady, podStatusProvider status.PodStatusProvider, containerRuntime runtimeService, initialContainers containermap.ContainerMap) error

	// Called to trigger the allocation of CPUs to a container. This must be
	// called at some point prior to the AddContainer() call for a container,
	// e.g. at pod admission time.
	Allocate(ctx context.Context, pod *v1.Pod, container *v1.Container) error

	// AddContainer adds the mapping between container ID to pod UID and the container name
	// The mapping used to remove the CPU allocation during the container removal
	AddContainer(logger logr.Logger, p *v1.Pod, c *v1.Container, containerID string)

	// RemoveContainer is called after Kubelet decides to kill or delete a
	// container. After this call, the CPU manager stops trying to reconcile
	// that container and any CPUs dedicated to the container are freed.
	RemoveContainer(logger logr.Logger, containerID string) error

	// State returns a read-only interface to the internal CPU manager state.
	State() state.Reader

	// GetTopologyHints implements the topologymanager.HintProvider Interface
	// and is consulted to achieve NUMA aware resource alignment among this
	// and other resource controllers.
	GetTopologyHints(logger logr.Logger, pod *v1.Pod, container *v1.Container) map[string][]topologymanager.TopologyHint

	// GetExclusiveCPUs implements the podresources.CPUsProvider interface to provide
	// exclusively allocated cpus for the container
	GetExclusiveCPUs(podUID, containerName string) cpuset.CPUSet

	// GetPodTopologyHints implements the topologymanager.HintProvider Interface
	// and is consulted to achieve NUMA aware resource alignment per Pod
	// among this and other resource controllers.
	GetPodTopologyHints(logger logr.Logger, pod *v1.Pod) map[string][]topologymanager.TopologyHint

	// AllocatePod is called to trigger the allocation of CPUs to a pod.
	AllocatePod(logger logr.Logger, pod *v1.Pod) error

	// GetAllocatableCPUs returns the total set of CPUs available for allocation.
	GetAllocatableCPUs() cpuset.CPUSet

	// GetCPUAffinity returns cpuset which includes cpus from shared pools
	// as well as exclusively allocated cpus
	GetCPUAffinity(podUID, containerName string) cpuset.CPUSet

	// GetAllCPUs returns all the CPUs known by cpumanager, as reported by the
	// hardware discovery. Maps to the CPU capacity.
	GetAllCPUs() cpuset.CPUSet

	// GetResourceIsolationLevel returns the isolation level of the container.
	GetResourceIsolationLevel(pod *v1.Pod, container *v1.Container) cmqos.ResourceIsolationLevel
}

type manager struct {
	sync.Mutex
	policy Policy

	// reconcilePeriod is the duration between calls to reconcileState.
	reconcilePeriod time.Duration

	// state allows pluggable CPU assignment policies while sharing a common
	// representation of state for the system to inspect and reconcile.
	state state.State

	// lastUpdatedstate holds state for each container from the last time it was updated.
	lastUpdateState state.State

	// containerRuntime is the container runtime service interface needed
	// to make UpdateContainerResources() calls against the containers.
	containerRuntime runtimeService

	// activePods is a method for listing active pods on the node
	// so all the containers can be updated in the reconciliation loop.
	activePods ActivePodsFunc

	// podStatusProvider provides a method for obtaining pod statuses
	// and the containerID of their containers
	podStatusProvider status.PodStatusProvider

	// containerMap provides a mapping from (pod, container) -> containerID
	// for all containers a pod
	containerMap containermap.ContainerMap

	topology *topology.CPUTopology

	nodeAllocatableReservation v1.ResourceList

	// sourcesReady provides the readiness of kubelet configuration sources such as apiserver update readiness.
	// We use it to determine when we can purge inactive pods from checkpointed state.
	sourcesReady config.SourcesReady

	// stateFileDirectory holds the directory where the state file for checkpoints is held.
	stateFileDirectory string

	// allCPUs is the set of online CPUs as reported by the system
	allCPUs cpuset.CPUSet

	// allocatableCPUs is the set of online CPUs as reported by the system,
	// and available for allocation, minus the reserved set
	allocatableCPUs cpuset.CPUSet
}

var _ Manager = &manager{}

type sourcesReadyStub struct{}

func (s *sourcesReadyStub) AddSource(source string) {}
func (s *sourcesReadyStub) AllReady() bool          { return true }

// NewManager creates new cpu manager based on provided policy
func NewManager(logger logr.Logger, cpuPolicyName string, cpuPolicyOptions map[string]string, reconcilePeriod time.Duration, machineInfo *cadvisorapi.MachineInfo, specificCPUs cpuset.CPUSet, nodeAllocatableReservation v1.ResourceList, stateFileDirectory string, affinity topologymanager.Store) (Manager, error) {
	var topo *topology.CPUTopology
	var policy Policy
	var err error

	topo, err = topology.Discover(logger, machineInfo)
	if err != nil {
		return nil, err
	}

	switch policyName(cpuPolicyName) {

	case PolicyNone:
		policy, err = NewNonePolicy(cpuPolicyOptions)
		if err != nil {
			return nil, fmt.Errorf("new none policy error: %w", err)
		}

	case PolicyStatic:
		logger.Info("Detected CPU topology", "topology", topo)

		reservedCPUs, ok := nodeAllocatableReservation[v1.ResourceCPU]
		if !ok {
			// The static policy cannot initialize without this information.
			return nil, fmt.Errorf("[cpumanager] unable to determine reserved CPU resources for static policy")
		}
		if reservedCPUs.IsZero() {
			// The static policy requires this to be nonzero. Zero CPU reservation
			// would allow the shared pool to be completely exhausted. At that point
			// either we would violate our guarantee of exclusivity or need to evict
			// any pod that has at least one container that requires zero CPUs.
			// See the comments in policy_static.go for more details.
			return nil, fmt.Errorf("[cpumanager] the static policy requires systemreserved.cpu + kubereserved.cpu to be greater than zero")
		}

		// Take the ceiling of the reservation, since fractional CPUs cannot be
		// exclusively allocated.
		reservedCPUsFloat := float64(reservedCPUs.MilliValue()) / 1000
		numReservedCPUs := int(math.Ceil(reservedCPUsFloat))
		policy, err = NewStaticPolicy(logger, topo, numReservedCPUs, specificCPUs, affinity, cpuPolicyOptions)
		if err != nil {
			return nil, fmt.Errorf("new static policy error: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown policy: \"%s\"", cpuPolicyName)
	}

	manager := &manager{
		policy:                     policy,
		reconcilePeriod:            reconcilePeriod,
		lastUpdateState:            state.NewMemoryState(logger),
		topology:                   topo,
		nodeAllocatableReservation: nodeAllocatableReservation,
		stateFileDirectory:         stateFileDirectory,
		allCPUs:                    topo.CPUDetails.CPUs(),
	}
	manager.sourcesReady = &sourcesReadyStub{}
	return manager, nil
}

func (m *manager) Start(ctx context.Context, activePods ActivePodsFunc, sourcesReady config.SourcesReady, podStatusProvider status.PodStatusProvider, containerRuntime runtimeService, initialContainers containermap.ContainerMap) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting", "policy", m.policy.Name())
	logger.Info("Reconciling", "reconcilePeriod", m.reconcilePeriod)
	m.sourcesReady = sourcesReady
	m.activePods = activePods
	m.podStatusProvider = podStatusProvider
	m.containerRuntime = containerRuntime
	m.containerMap = initialContainers

	stateImpl, err := state.NewCheckpointState(logger, m.stateFileDirectory, cpuManagerStateFileName, m.policy.Name(), m.containerMap)
	if err != nil {
		logger.Error(err, "Could not initialize checkpoint manager, please drain node and remove policy state file")
		return err
	}
	m.state = stateImpl

	err = m.policy.Start(logger, m.state)
	if err != nil {
		logger.Error(err, "Policy start error")
		return err
	}

	logger.V(4).Info("CPU manager started", "policy", m.policy.Name())

	m.allocatableCPUs = m.policy.GetAllocatableCPUs(m.state)

	if m.policy.Name() == string(PolicyNone) {
		return nil
	}
	// Periodically call m.reconcileState() to continue to keep the CPU sets of
	// all pods in sync with and guaranteed CPUs handed out among them.
	go wait.Until(func() { m.reconcileState(ctx) }, m.reconcilePeriod, wait.NeverStop)
	return nil
}

func (m *manager) Allocate(ctx context.Context, p *v1.Pod, c *v1.Container) error {
	logger := klog.FromContext(ctx)

	// Garbage collect any stranded resources before allocating CPUs.
	m.removeStaleState(logger)

	m.Lock()
	defer m.Unlock()

	// Call down into the policy to assign this container CPUs if required.
	err := m.policy.Allocate(logger, m.state, p, c)
	if err != nil {
		logger.Error(err, "policy error")
		return err
	}

	return nil
}

func (m *manager) AllocatePod(logger logr.Logger, pod *v1.Pod) error {
	// Garbage collect any stranded resources before allocating CPUs.
	m.removeStaleState(logger)

	m.Lock()
	defer m.Unlock()

	// Call down into the policy to assign this container CPUs if required.
	if err := m.policy.AllocatePod(logger, m.state, pod); err != nil {
		logger.Error(err, "AllocatePod error", "pod", klog.KObj(pod))
		return err
	}
	return nil
}

func (m *manager) AddContainer(logger logr.Logger, pod *v1.Pod, container *v1.Container, containerID string) {
	m.Lock()
	defer m.Unlock()
	if cset, exists := m.state.GetCPUSet(string(pod.UID), container.Name); exists {
		m.lastUpdateState.SetCPUSet(string(pod.UID), container.Name, cset)
	}
	m.containerMap.Add(string(pod.UID), container.Name, containerID)
	logger.V(4).Info("Added Container", "pod", klog.KObj(pod), "podUID", pod.UID, "containerName", container.Name, "containerID", containerID)
}

func (m *manager) RemoveContainer(logger logr.Logger, containerID string) error {
	m.Lock()
	defer m.Unlock()

	err := m.policyRemoveContainerByID(logger, containerID)
	if err != nil {
		logger.Error(err, "RemoveContainer error")
		return err
	}

	return nil
}

func (m *manager) policyRemoveContainerByID(logger logr.Logger, containerID string) error {
	podUID, containerName, err := m.containerMap.GetContainerRef(containerID)
	if err != nil {
		return nil
	}

	err = m.policy.RemoveContainer(logger, m.state, podUID, containerName)
	if err == nil {
		m.lastUpdateState.Delete(podUID, containerName)
		m.containerMap.RemoveByContainerID(containerID)
	}

	return err
}

func (m *manager) policyRemoveContainerByRef(logger logr.Logger, podUID string, containerName string) error {
	err := m.policy.RemoveContainer(logger, m.state, podUID, containerName)
	if err == nil {
		m.lastUpdateState.Delete(podUID, containerName)
		m.containerMap.RemoveByContainerRef(podUID, containerName)
	}

	return err
}

func (m *manager) State() state.Reader {
	return m.state
}

func (m *manager) GetTopologyHints(logger logr.Logger, pod *v1.Pod, container *v1.Container) map[string][]topologymanager.TopologyHint {
	// Garbage collect any stranded resources before providing TopologyHints
	m.removeStaleState(logger)
	// Delegate to active policy
	return m.policy.GetTopologyHints(logger, m.state, pod, container)
}

func (m *manager) GetPodTopologyHints(logger logr.Logger, pod *v1.Pod) map[string][]topologymanager.TopologyHint {
	// Garbage collect any stranded resources before providing TopologyHints
	m.removeStaleState(logger)
	// Delegate to active policy
	return m.policy.GetPodTopologyHints(logger, m.state, pod)
}

func (m *manager) GetAllocatableCPUs() cpuset.CPUSet {
	return m.allocatableCPUs.Clone()
}

func (m *manager) GetAllCPUs() cpuset.CPUSet {
	return m.allCPUs.Clone()
}

type reconciledContainer struct {
	podName       string
	containerName string
	containerID   string
}

func (m *manager) removeStaleState(rootLogger logr.Logger) {
	// Only once all sources are ready do we attempt to remove any stale state.
	// This ensures that the call to `m.activePods()` below will succeed with
	// the actual active pods list.
	if !m.sourcesReady.AllReady() {
		return
	}

	// We grab the lock to ensure that no new containers will grab CPUs while
	// executing the code below. Without this lock, its possible that we end up
	// removing state that is newly added by an asynchronous call to
	// AddContainer() during the execution of this code.
	m.Lock()
	defer m.Unlock()

	// Get the list of active pods.
	activePods := m.activePods()

	// Build a list of (podUID, containerName) pairs for all containers in all active Pods.
	activeContainers := make(map[string]map[string]struct{})
	for _, pod := range activePods {
		activeContainers[string(pod.UID)] = make(map[string]struct{})
		for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			activeContainers[string(pod.UID)][container.Name] = struct{}{}
		}
	}

	// Loop through the CPUManager state. Remove any state for containers not
	// in the `activeContainers` list built above.
	assignments := m.state.GetCPUAssignments()
	for podUID := range assignments {
		for containerName := range assignments[podUID] {
			logger := klog.LoggerWithValues(rootLogger, "podUID", podUID, "containerName", containerName)

			if _, ok := activeContainers[podUID][containerName]; ok {
				logger.V(5).Info("container still active")
				continue
			}

			logger.V(2).Info("removing container")
			err := m.policyRemoveContainerByRef(logger, podUID, containerName)
			if err != nil {
				logger.Error(err, "failed to remove container")
			}
		}
	}

	m.containerMap.Visit(func(podUID, containerName, containerID string) {
		logger := klog.LoggerWithValues(rootLogger, "podUID", podUID, "containerName", containerName)
		if _, ok := activeContainers[podUID][containerName]; ok {
			logger.V(5).Info("containerMap: container still active")
			return
		}
		logger.V(2).Info("containerMap: removing container")
		err := m.policyRemoveContainerByRef(logger, podUID, containerName)
		if err != nil {
			logger.Error(err, "containerMap: failed to remove container")
		}
	})
}

func (m *manager) reconcileState(ctx context.Context) (success []reconciledContainer, failure []reconciledContainer) {
	success = []reconciledContainer{}
	failure = []reconciledContainer{}

	rootLogger := klog.FromContext(ctx)

	m.removeStaleState(rootLogger)
	for _, pod := range m.activePods() {
		podLogger := klog.LoggerWithValues(rootLogger, "pod", klog.KObj(pod))

		pstatus, ok := m.podStatusProvider.GetPodStatus(pod.UID)
		if !ok {
			podLogger.V(5).Info("skipping pod; status not found")
			failure = append(failure, reconciledContainer{pod.Name, "", ""})
			continue
		}

		allContainers := pod.Spec.InitContainers
		allContainers = append(allContainers, pod.Spec.Containers...)
		for _, container := range allContainers {
			logger := klog.LoggerWithValues(podLogger, "containerName", container.Name)

			containerID, err := findContainerIDByName(&pstatus, container.Name)
			if err != nil {
				logger.V(5).Info("skipping container; ID not found in pod status", "err", err)
				failure = append(failure, reconciledContainer{pod.Name, container.Name, ""})
				continue
			}

			cstatus, err := findContainerStatusByName(&pstatus, container.Name)
			if err != nil {
				logger.V(5).Info("skipping container; container status not found in pod status", "err", err)
				failure = append(failure, reconciledContainer{pod.Name, container.Name, ""})
				continue
			}

			if cstatus.State.Waiting != nil ||
				(cstatus.State.Waiting == nil && cstatus.State.Running == nil && cstatus.State.Terminated == nil) {
				logger.V(4).Info("skipping container; container still in the waiting state", "err", err)
				failure = append(failure, reconciledContainer{pod.Name, container.Name, ""})
				continue
			}

			m.Lock()
			if cstatus.State.Terminated != nil {
				// The container is terminated but we can't call m.RemoveContainer()
				// here because it could remove the allocated cpuset for the container
				// which may be in the process of being restarted.  That would result
				// in the container losing any exclusively-allocated CPUs that it
				// was allocated.
				_, _, err := m.containerMap.GetContainerRef(containerID)
				if err == nil {
					logger.V(4).Info("ignoring terminated container", "containerID", containerID)
				}
				m.Unlock()
				continue
			}

			// Once we make it here we know we have a running container.
			// Idempotently add it to the containerMap incase it is missing.
			// This can happen after a kubelet restart, for example.
			m.containerMap.Add(string(pod.UID), container.Name, containerID)
			m.Unlock()

			cset := m.state.GetCPUSetOrDefault(string(pod.UID), container.Name)
			if cset.IsEmpty() {
				// NOTE: This should not happen outside of tests.
				logger.V(2).Info("ReconcileState: skipping container; empty cpuset assigned")
				failure = append(failure, reconciledContainer{pod.Name, container.Name, containerID})
				continue
			}

			lcset := m.lastUpdateState.GetCPUSetOrDefault(string(pod.UID), container.Name)
			if !cset.Equals(lcset) {
				logger.V(5).Info("updating container", "containerID", containerID, "cpuSet", cset)
				err = m.updateContainerCPUSet(ctx, containerID, cset)
				if err != nil {
					logger.Error(err, "failed to update container", "containerID", containerID, "cpuSet", cset)
					failure = append(failure, reconciledContainer{pod.Name, container.Name, containerID})
					continue
				}
				m.lastUpdateState.SetCPUSet(string(pod.UID), container.Name, cset)
			}
			success = append(success, reconciledContainer{pod.Name, container.Name, containerID})
		}
	}
	return success, failure
}

func findContainerIDByName(status *v1.PodStatus, name string) (string, error) {
	allStatuses := status.InitContainerStatuses
	allStatuses = append(allStatuses, status.ContainerStatuses...)
	for _, container := range allStatuses {
		if container.Name == name && container.ContainerID != "" {
			cid := &kubecontainer.ContainerID{}
			err := cid.ParseString(container.ContainerID)
			if err != nil {
				return "", err
			}
			return cid.ID, nil
		}
	}
	return "", fmt.Errorf("unable to find ID for container with name %v in pod status (it may not be running)", name)
}

func findContainerStatusByName(status *v1.PodStatus, name string) (*v1.ContainerStatus, error) {
	for _, containerStatus := range append(status.InitContainerStatuses, status.ContainerStatuses...) {
		if containerStatus.Name == name {
			return &containerStatus, nil
		}
	}
	return nil, fmt.Errorf("unable to find status for container with name %v in pod status (it may not be running)", name)
}

func (m *manager) GetExclusiveCPUs(podUID, containerName string) cpuset.CPUSet {
	if result, ok := m.state.GetCPUSet(podUID, containerName); ok {
		return result
	}
	return cpuset.New()
}

func (m *manager) GetCPUAffinity(podUID, containerName string) cpuset.CPUSet {
	return m.state.GetCPUSetOrDefault(podUID, containerName)
}

func resourcesQualifyForExclusiveCPUs(container *v1.Container) bool {
	if !cmqos.IsContainerEquivalentQOSGuaranteed(container) {
		return false
	}

	cpuLimit := container.Resources.Limits[v1.ResourceCPU]
	return cpuLimit.Value()*1000 == cpuLimit.MilliValue()
}

func (m *manager) GetResourceIsolationLevel(pod *v1.Pod, container *v1.Container) cmqos.ResourceIsolationLevel {
	if _, ok := m.state.GetCPUSet(string(pod.UID), container.Name); !ok {
		return cmqos.ResourceIsolationHost
	}

	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.PodLevelResourceManagers) && resourcehelper.IsPodLevelResourcesSet(pod) && !resourcesQualifyForExclusiveCPUs(container) {
		return cmqos.ResourceIsolationPod
	}

	return cmqos.ResourceIsolationContainer
}

```

// === FILE: references!/kubernetes/pkg/kubelet/pleg/pleg.go ===
```go
/*
Copyright 2015 The Kubernetes Authors.

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

package pleg

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// PodLifeCycleEventType define the event type of pod life cycle events.
type PodLifeCycleEventType string

type RelistDuration struct {
	// The period for relisting.
	RelistPeriod time.Duration
	// The relisting threshold needs to be greater than the relisting period +
	// the relisting time, which can vary significantly. Set a conservative
	// threshold to avoid flipping between healthy and unhealthy.
	RelistThreshold time.Duration
}

const (
	// ContainerStarted - event type when the new state of container is running.
	ContainerStarted PodLifeCycleEventType = "ContainerStarted"
	// ContainerDied - event type when the new state of container is exited.
	ContainerDied PodLifeCycleEventType = "ContainerDied"
	// ContainerRemoved - event type when the old state of container is exited.
	ContainerRemoved PodLifeCycleEventType = "ContainerRemoved"
	// PodSync is used to trigger syncing of a pod when the observed change of
	// the state of the pod cannot be captured by any single event above.
	PodSync PodLifeCycleEventType = "PodSync"
	// ContainerChanged - event type when the new state of container is unknown.
	ContainerChanged PodLifeCycleEventType = "ContainerChanged"
)

// PodLifecycleEvent is an event that reflects the change of the pod state.
type PodLifecycleEvent struct {
	// The pod ID.
	ID types.UID
	// The type of the event.
	Type PodLifeCycleEventType
	// The accompanied data which varies based on the event type.
	//   - ContainerStarted/ContainerStopped: the container name (string).
	//   - All other event types: unused.
	Data interface{}
}

// PodLifecycleEventGenerator contains functions for generating pod life cycle events.
type PodLifecycleEventGenerator interface {
	Start(ctx context.Context)
	Watch() chan *PodLifecycleEvent
	Healthy() (bool, error)
	// RequestReinspect flags the pod for reinspection on the next Relist iteration.
	RequestReinspect(podUID types.UID)
	// RequestRelist queues up the pod for an on-demand relist.
	RequestRelist(logger klog.Logger, podUID types.UID)
}

// podLifecycleEventGeneratorHandler contains functions that are useful for different PLEGs
// and need not be exposed to rest of the kubelet
type podLifecycleEventGeneratorHandler interface {
	PodLifecycleEventGenerator
	Stop()
	Update(relistDuration *RelistDuration)
	Relist(ctx context.Context)
}

```

