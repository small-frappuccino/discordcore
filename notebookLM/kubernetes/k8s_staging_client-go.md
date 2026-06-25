# Domain Architecture: staging/src/k8s.io/client-go

## Layout Topology
```text
staging/src/k8s.io/client-go/
├── applyconfigurations
│   ├── admissionregistration
│   │   ├── v1
│   │   │   ├── applyconfiguration.go
│   │   │   ├── auditannotation.go
│   │   │   ├── expressionwarning.go
│   │   │   ├── jsonpatch.go
│   │   │   ├── matchcondition.go
│   │   │   ├── matchresources.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── mutatingadmissionpolicybindingspec.go
│   │   │   ├── mutatingadmissionpolicyspec.go
│   │   │   ├── mutatingwebhook.go
│   │   │   ├── mutatingwebhookconfiguration.go
│   │   │   ├── mutation.go
│   │   │   ├── namedrulewithoperations.go
│   │   │   ├── paramkind.go
│   │   │   ├── paramref.go
│   │   │   ├── rule.go
│   │   │   ├── rulewithoperations.go
│   │   │   ├── servicereference.go
│   │   │   ├── typechecking.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   ├── validatingadmissionpolicybinding.go
│   │   │   ├── validatingadmissionpolicybindingspec.go
│   │   │   ├── validatingadmissionpolicyspec.go
│   │   │   ├── validatingadmissionpolicystatus.go
│   │   │   ├── validatingwebhook.go
│   │   │   ├── validatingwebhookconfiguration.go
│   │   │   ├── validation.go
│   │   │   ├── variable.go
│   │   │   └── webhookclientconfig.go
│   │   ├── v1alpha1
│   │   │   ├── applyconfiguration.go
│   │   │   ├── auditannotation.go
│   │   │   ├── expressionwarning.go
│   │   │   ├── jsonpatch.go
│   │   │   ├── matchcondition.go
│   │   │   ├── matchresources.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── mutatingadmissionpolicybindingspec.go
│   │   │   ├── mutatingadmissionpolicyspec.go
│   │   │   ├── mutation.go
│   │   │   ├── namedrulewithoperations.go
│   │   │   ├── paramkind.go
│   │   │   ├── paramref.go
│   │   │   ├── typechecking.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   ├── validatingadmissionpolicybinding.go
│   │   │   ├── validatingadmissionpolicybindingspec.go
│   │   │   ├── validatingadmissionpolicyspec.go
│   │   │   ├── validatingadmissionpolicystatus.go
│   │   │   ├── validation.go
│   │   │   └── variable.go
│   │   └── v1beta1
│   │       ├── applyconfiguration.go
│   │       ├── auditannotation.go
│   │       ├── expressionwarning.go
│   │       ├── jsonpatch.go
│   │       ├── matchcondition.go
│   │       ├── matchresources.go
│   │       ├── mutatingadmissionpolicy.go
│   │       ├── mutatingadmissionpolicybinding.go
│   │       ├── mutatingadmissionpolicybindingspec.go
│   │       ├── mutatingadmissionpolicyspec.go
│   │       ├── mutatingwebhook.go
│   │       ├── mutatingwebhookconfiguration.go
│   │       ├── mutation.go
│   │       ├── namedrulewithoperations.go
│   │       ├── paramkind.go
│   │       ├── paramref.go
│   │       ├── servicereference.go
│   │       ├── typechecking.go
│   │       ├── validatingadmissionpolicy.go
│   │       ├── validatingadmissionpolicybinding.go
│   │       ├── validatingadmissionpolicybindingspec.go
│   │       ├── validatingadmissionpolicyspec.go
│   │       ├── validatingadmissionpolicystatus.go
│   │       ├── validatingwebhook.go
│   │       ├── validatingwebhookconfiguration.go
│   │       ├── validation.go
│   │       ├── variable.go
│   │       └── webhookclientconfig.go
│   ├── apiserverinternal
│   │   └── v1alpha1
│   │       ├── serverstorageversion.go
│   │       ├── storageversion.go
│   │       ├── storageversioncondition.go
│   │       └── storageversionstatus.go
│   ├── apps
│   │   ├── v1
│   │   │   ├── controllerrevision.go
│   │   │   ├── daemonset.go
│   │   │   ├── daemonsetcondition.go
│   │   │   ├── daemonsetspec.go
│   │   │   ├── daemonsetstatus.go
│   │   │   ├── daemonsetupdatestrategy.go
│   │   │   ├── deployment.go
│   │   │   ├── deploymentcondition.go
│   │   │   ├── deploymentspec.go
│   │   │   ├── deploymentstatus.go
│   │   │   ├── deploymentstrategy.go
│   │   │   ├── replicaset.go
│   │   │   ├── replicasetcondition.go
│   │   │   ├── replicasetspec.go
│   │   │   ├── replicasetstatus.go
│   │   │   ├── rollingupdatedaemonset.go
│   │   │   ├── rollingupdatedeployment.go
│   │   │   ├── rollingupdatestatefulsetstrategy.go
│   │   │   ├── statefulset.go
│   │   │   ├── statefulsetcondition.go
│   │   │   ├── statefulsetordinals.go
│   │   │   ├── statefulsetpersistentvolumeclaimretentionpolicy.go
│   │   │   ├── statefulsetspec.go
│   │   │   ├── statefulsetstatus.go
│   │   │   └── statefulsetupdatestrategy.go
│   │   ├── v1beta1
│   │   │   ├── controllerrevision.go
│   │   │   ├── deployment.go
│   │   │   ├── deploymentcondition.go
│   │   │   ├── deploymentspec.go
│   │   │   ├── deploymentstatus.go
│   │   │   ├── deploymentstrategy.go
│   │   │   ├── rollbackconfig.go
│   │   │   ├── rollingupdatedeployment.go
│   │   │   ├── rollingupdatestatefulsetstrategy.go
│   │   │   ├── statefulset.go
│   │   │   ├── statefulsetcondition.go
│   │   │   ├── statefulsetordinals.go
│   │   │   ├── statefulsetpersistentvolumeclaimretentionpolicy.go
│   │   │   ├── statefulsetspec.go
│   │   │   ├── statefulsetstatus.go
│   │   │   └── statefulsetupdatestrategy.go
│   │   └── v1beta2
│   │       ├── controllerrevision.go
│   │       ├── daemonset.go
│   │       ├── daemonsetcondition.go
│   │       ├── daemonsetspec.go
│   │       ├── daemonsetstatus.go
│   │       ├── daemonsetupdatestrategy.go
│   │       ├── deployment.go
│   │       ├── deploymentcondition.go
│   │       ├── deploymentspec.go
│   │       ├── deploymentstatus.go
│   │       ├── deploymentstrategy.go
│   │       ├── replicaset.go
│   │       ├── replicasetcondition.go
│   │       ├── replicasetspec.go
│   │       ├── replicasetstatus.go
│   │       ├── rollingupdatedaemonset.go
│   │       ├── rollingupdatedeployment.go
│   │       ├── rollingupdatestatefulsetstrategy.go
│   │       ├── scale.go
│   │       ├── statefulset.go
│   │       ├── statefulsetcondition.go
│   │       ├── statefulsetordinals.go
│   │       ├── statefulsetpersistentvolumeclaimretentionpolicy.go
│   │       ├── statefulsetspec.go
│   │       ├── statefulsetstatus.go
│   │       └── statefulsetupdatestrategy.go
│   ├── autoscaling
│   │   ├── v1
│   │   │   ├── crossversionobjectreference.go
│   │   │   ├── horizontalpodautoscaler.go
│   │   │   ├── horizontalpodautoscalerspec.go
│   │   │   ├── horizontalpodautoscalerstatus.go
│   │   │   ├── scale.go
│   │   │   ├── scalespec.go
│   │   │   └── scalestatus.go
│   │   └── v2
│   │       ├── containerresourcemetricsource.go
│   │       ├── containerresourcemetricstatus.go
│   │       ├── crossversionobjectreference.go
│   │       ├── externalmetricsource.go
│   │       ├── externalmetricstatus.go
│   │       ├── horizontalpodautoscaler.go
│   │       ├── horizontalpodautoscalerbehavior.go
│   │       ├── horizontalpodautoscalercondition.go
│   │       ├── horizontalpodautoscalerspec.go
│   │       ├── horizontalpodautoscalerstatus.go
│   │       ├── hpascalingpolicy.go
│   │       ├── hpascalingrules.go
│   │       ├── metricidentifier.go
│   │       ├── metricspec.go
│   │       ├── metricstatus.go
│   │       ├── metrictarget.go
│   │       ├── metricvaluestatus.go
│   │       ├── objectmetricsource.go
│   │       ├── objectmetricstatus.go
│   │       ├── podsmetricsource.go
│   │       ├── podsmetricstatus.go
│   │       ├── resourcemetricsource.go
│   │       └── resourcemetricstatus.go
│   ├── batch
│   │   ├── v1
│   │   │   ├── cronjob.go
│   │   │   ├── cronjobspec.go
│   │   │   ├── cronjobstatus.go
│   │   │   ├── job.go
│   │   │   ├── jobcondition.go
│   │   │   ├── jobspec.go
│   │   │   ├── jobstatus.go
│   │   │   ├── jobtemplatespec.go
│   │   │   ├── podfailurepolicy.go
│   │   │   ├── podfailurepolicyonexitcodesrequirement.go
│   │   │   ├── podfailurepolicyonpodconditionspattern.go
│   │   │   ├── podfailurepolicyrule.go
│   │   │   ├── successpolicy.go
│   │   │   ├── successpolicyrule.go
│   │   │   └── uncountedterminatedpods.go
│   │   └── v1beta1
│   │       ├── cronjob.go
│   │       ├── cronjobspec.go
│   │       ├── cronjobstatus.go
│   │       └── jobtemplatespec.go
│   ├── certificates
│   │   ├── v1
│   │   │   ├── certificatesigningrequest.go
│   │   │   ├── certificatesigningrequestcondition.go
│   │   │   ├── certificatesigningrequestspec.go
│   │   │   └── certificatesigningrequeststatus.go
│   │   ├── v1alpha1
│   │   │   ├── clustertrustbundle.go
│   │   │   └── clustertrustbundlespec.go
│   │   └── v1beta1
│   │       ├── certificatesigningrequest.go
│   │       ├── certificatesigningrequestcondition.go
│   │       ├── certificatesigningrequestspec.go
│   │       ├── certificatesigningrequeststatus.go
│   │       ├── clustertrustbundle.go
│   │       ├── clustertrustbundlespec.go
│   │       ├── podcertificaterequest.go
│   │       ├── podcertificaterequestspec.go
│   │       └── podcertificaterequeststatus.go
│   ├── coordination
│   │   ├── v1
│   │   │   ├── lease.go
│   │   │   └── leasespec.go
│   │   ├── v1alpha2
│   │   │   ├── leasecandidate.go
│   │   │   └── leasecandidatespec.go
│   │   └── v1beta1
│   │       ├── lease.go
│   │       ├── leasecandidate.go
│   │       ├── leasecandidatespec.go
│   │       └── leasespec.go
│   ├── core
│   │   └── v1
│   │       ├── affinity.go
│   │       ├── apparmorprofile.go
│   │       ├── attachedvolume.go
│   │       ├── awselasticblockstorevolumesource.go
│   │       ├── azurediskvolumesource.go
│   │       ├── azurefilepersistentvolumesource.go
│   │       ├── azurefilevolumesource.go
│   │       ├── capabilities.go
│   │       ├── cephfspersistentvolumesource.go
│   │       ├── cephfsvolumesource.go
│   │       ├── cinderpersistentvolumesource.go
│   │       ├── cindervolumesource.go
│   │       ├── clientipconfig.go
│   │       ├── clustertrustbundleprojection.go
│   │       ├── componentcondition.go
│   │       ├── componentstatus.go
│   │       ├── configmap.go
│   │       ├── configmapenvsource.go
│   │       ├── configmapkeyselector.go
│   │       ├── configmapnodeconfigsource.go
│   │       ├── configmapprojection.go
│   │       ├── configmapvolumesource.go
│   │       ├── container.go
│   │       ├── containerextendedresourcerequest.go
│   │       ├── containerimage.go
│   │       ├── containerport.go
│   │       ├── containerresizepolicy.go
│   │       ├── containerrestartrule.go
│   │       ├── containerrestartruleonexitcodes.go
│   │       ├── containerstate.go
│   │       ├── containerstaterunning.go
│   │       ├── containerstateterminated.go
│   │       ├── containerstatewaiting.go
│   │       ├── containerstatus.go
│   │       ├── containeruser.go
│   │       ├── csipersistentvolumesource.go
│   │       ├── csivolumesource.go
│   │       ├── daemonendpoint.go
│   │       ├── downwardapiprojection.go
│   │       ├── downwardapivolumefile.go
│   │       ├── downwardapivolumesource.go
│   │       ├── emptydirvolumesource.go
│   │       ├── endpointaddress.go
│   │       ├── endpointport.go
│   │       ├── endpoints.go
│   │       ├── endpointsubset.go
│   │       ├── envfromsource.go
│   │       ├── envvar.go
│   │       ├── envvarsource.go
│   │       ├── ephemeralcontainer.go
│   │       ├── ephemeralcontainercommon.go
│   │       ├── ephemeralvolumesource.go
│   │       ├── event.go
│   │       ├── eventseries.go
│   │       ├── eventsource.go
│   │       ├── execaction.go
│   │       ├── fcvolumesource.go
│   │       ├── filekeyselector.go
│   │       ├── flexpersistentvolumesource.go
│   │       ├── flexvolumesource.go
│   │       ├── flockervolumesource.go
│   │       ├── gcepersistentdiskvolumesource.go
│   │       ├── gitrepovolumesource.go
│   │       ├── glusterfspersistentvolumesource.go
│   │       ├── glusterfsvolumesource.go
│   │       ├── grpcaction.go
│   │       ├── hostalias.go
│   │       ├── hostip.go
│   │       ├── hostpathvolumesource.go
│   │       ├── httpgetaction.go
│   │       ├── httpheader.go
│   │       ├── imagevolumesource.go
│   │       ├── imagevolumestatus.go
│   │       ├── iscsipersistentvolumesource.go
│   │       ├── iscsivolumesource.go
│   │       ├── keytopath.go
│   │       ├── lifecycle.go
│   │       ├── lifecyclehandler.go
│   │       ├── limitrange.go
│   │       ├── limitrangeitem.go
│   │       ├── limitrangespec.go
│   │       ├── linuxcontaineruser.go
│   │       ├── loadbalanceringress.go
│   │       ├── loadbalancerstatus.go
│   │       ├── localobjectreference.go
│   │       ├── localvolumesource.go
│   │       ├── modifyvolumestatus.go
│   │       ├── namespace.go
│   │       ├── namespacecondition.go
│   │       ├── namespacespec.go
│   │       ├── namespacestatus.go
│   │       ├── nfsvolumesource.go
│   │       ├── node.go
│   │       ├── nodeaddress.go
│   │       ├── nodeaffinity.go
│   │       ├── nodeallocatableresourceclaimstatus.go
│   │       ├── nodecondition.go
│   │       ├── nodeconfigsource.go
│   │       ├── nodeconfigstatus.go
│   │       ├── nodedaemonendpoints.go
│   │       ├── nodefeatures.go
│   │       ├── noderuntimehandler.go
│   │       ├── noderuntimehandlerfeatures.go
│   │       ├── nodeselector.go
│   │       ├── nodeselectorrequirement.go
│   │       ├── nodeselectorterm.go
│   │       ├── nodespec.go
│   │       ├── nodestatus.go
│   │       ├── nodeswapstatus.go
│   │       ├── nodesysteminfo.go
│   │       ├── objectfieldselector.go
│   │       ├── objectreference.go
│   │       ├── persistentvolume.go
│   │       ├── persistentvolumeclaim.go
│   │       ├── persistentvolumeclaimcondition.go
│   │       ├── persistentvolumeclaimspec.go
│   │       ├── persistentvolumeclaimstatus.go
│   │       ├── persistentvolumeclaimtemplate.go
│   │       ├── persistentvolumeclaimvolumesource.go
│   │       ├── persistentvolumesource.go
│   │       ├── persistentvolumespec.go
│   │       ├── persistentvolumestatus.go
│   │       ├── photonpersistentdiskvolumesource.go
│   │       ├── pod.go
│   │       ├── podaffinity.go
│   │       ├── podaffinityterm.go
│   │       ├── podantiaffinity.go
│   │       ├── podcertificateprojection.go
│   │       ├── podcondition.go
│   │       ├── poddnsconfig.go
│   │       ├── poddnsconfigoption.go
│   │       ├── podextendedresourceclaimstatus.go
│   │       ├── podip.go
│   │       ├── podos.go
│   │       ├── podreadinessgate.go
│   │       ├── podresourceclaim.go
│   │       ├── podresourceclaimstatus.go
│   │       ├── podschedulinggate.go
│   │       ├── podschedulinggroup.go
│   │       ├── podsecuritycontext.go
│   │       ├── podspec.go
│   │       ├── podstatus.go
│   │       ├── podtemplate.go
│   │       ├── podtemplatespec.go
│   │       ├── portstatus.go
│   │       ├── portworxvolumesource.go
│   │       ├── preferredschedulingterm.go
│   │       ├── probe.go
│   │       ├── probehandler.go
│   │       ├── projectedvolumesource.go
│   │       ├── quobytevolumesource.go
│   │       ├── rbdpersistentvolumesource.go
│   │       ├── rbdvolumesource.go
│   │       ├── replicationcontroller.go
│   │       ├── replicationcontrollercondition.go
│   │       ├── replicationcontrollerspec.go
│   │       ├── replicationcontrollerstatus.go
│   │       ├── resourceclaim.go
│   │       ├── resourcefieldselector.go
│   │       ├── resourcehealth.go
│   │       ├── resourcequota.go
│   │       ├── resourcequotaspec.go
│   │       ├── resourcequotastatus.go
│   │       ├── resourcerequirements.go
│   │       ├── resourcestatus.go
│   │       ├── scaleiopersistentvolumesource.go
│   │       ├── scaleiovolumesource.go
│   │       ├── scopedresourceselectorrequirement.go
│   │       ├── scopeselector.go
│   │       ├── seccompprofile.go
│   │       ├── secret.go
│   │       ├── secretenvsource.go
│   │       ├── secretkeyselector.go
│   │       ├── secretprojection.go
│   │       ├── secretreference.go
│   │       ├── secretvolumesource.go
│   │       ├── securitycontext.go
│   │       ├── selinuxoptions.go
│   │       ├── service.go
│   │       ├── serviceaccount.go
│   │       ├── serviceaccounttokenprojection.go
│   │       ├── serviceport.go
│   │       ├── servicespec.go
│   │       ├── servicestatus.go
│   │       ├── sessionaffinityconfig.go
│   │       ├── sleepaction.go
│   │       ├── storageospersistentvolumesource.go
│   │       ├── storageosvolumesource.go
│   │       ├── sysctl.go
│   │       ├── taint.go
│   │       ├── tcpsocketaction.go
│   │       ├── toleration.go
│   │       ├── topologyselectorlabelrequirement.go
│   │       ├── topologyselectorterm.go
│   │       ├── topologyspreadconstraint.go
│   │       ├── typedlocalobjectreference.go
│   │       ├── typedobjectreference.go
│   │       ├── volume.go
│   │       ├── volumedevice.go
│   │       ├── volumemount.go
│   │       ├── volumemountstatus.go
│   │       ├── volumenodeaffinity.go
│   │       ├── volumeprojection.go
│   │       ├── volumeresourcerequirements.go
│   │       ├── volumesource.go
│   │       ├── volumestatus.go
│   │       ├── vspherevirtualdiskvolumesource.go
│   │       ├── weightedpodaffinityterm.go
│   │       └── windowssecuritycontextoptions.go
│   ├── discovery
│   │   ├── v1
│   │   │   ├── endpoint.go
│   │   │   ├── endpointconditions.go
│   │   │   ├── endpointhints.go
│   │   │   ├── endpointport.go
│   │   │   ├── endpointslice.go
│   │   │   ├── fornode.go
│   │   │   └── forzone.go
│   │   └── v1beta1
│   │       ├── endpoint.go
│   │       ├── endpointconditions.go
│   │       ├── endpointhints.go
│   │       ├── endpointport.go
│   │       ├── endpointslice.go
│   │       ├── fornode.go
│   │       └── forzone.go
│   ├── events
│   │   ├── v1
│   │   │   ├── event.go
│   │   │   └── eventseries.go
│   │   └── v1beta1
│   │       ├── event.go
│   │       └── eventseries.go
│   ├── extensions
│   │   └── v1beta1
│   │       ├── daemonset.go
│   │       ├── daemonsetcondition.go
│   │       ├── daemonsetspec.go
│   │       ├── daemonsetstatus.go
│   │       ├── daemonsetupdatestrategy.go
│   │       ├── deployment.go
│   │       ├── deploymentcondition.go
│   │       ├── deploymentspec.go
│   │       ├── deploymentstatus.go
│   │       ├── deploymentstrategy.go
│   │       ├── httpingresspath.go
│   │       ├── httpingressrulevalue.go
│   │       ├── ingress.go
│   │       ├── ingressbackend.go
│   │       ├── ingressloadbalanceringress.go
│   │       ├── ingressloadbalancerstatus.go
│   │       ├── ingressportstatus.go
│   │       ├── ingressrule.go
│   │       ├── ingressrulevalue.go
│   │       ├── ingressspec.go
│   │       ├── ingressstatus.go
│   │       ├── ingresstls.go
│   │       ├── ipblock.go
│   │       ├── networkpolicy.go
│   │       ├── networkpolicyegressrule.go
│   │       ├── networkpolicyingressrule.go
│   │       ├── networkpolicypeer.go
│   │       ├── networkpolicyport.go
│   │       ├── networkpolicyspec.go
│   │       ├── replicaset.go
│   │       ├── replicasetcondition.go
│   │       ├── replicasetspec.go
│   │       ├── replicasetstatus.go
│   │       ├── rollbackconfig.go
│   │       ├── rollingupdatedaemonset.go
│   │       ├── rollingupdatedeployment.go
│   │       └── scale.go
│   ├── flowcontrol
│   │   ├── v1
│   │   │   ├── exemptprioritylevelconfiguration.go
│   │   │   ├── flowdistinguishermethod.go
│   │   │   ├── flowschema.go
│   │   │   ├── flowschemacondition.go
│   │   │   ├── flowschemaspec.go
│   │   │   ├── flowschemastatus.go
│   │   │   ├── groupsubject.go
│   │   │   ├── limitedprioritylevelconfiguration.go
│   │   │   ├── limitresponse.go
│   │   │   ├── nonresourcepolicyrule.go
│   │   │   ├── policyruleswithsubjects.go
│   │   │   ├── prioritylevelconfiguration.go
│   │   │   ├── prioritylevelconfigurationcondition.go
│   │   │   ├── prioritylevelconfigurationreference.go
│   │   │   ├── prioritylevelconfigurationspec.go
│   │   │   ├── prioritylevelconfigurationstatus.go
│   │   │   ├── queuingconfiguration.go
│   │   │   ├── resourcepolicyrule.go
│   │   │   ├── serviceaccountsubject.go
│   │   │   ├── subject.go
│   │   │   └── usersubject.go
│   │   ├── v1beta1
│   │   │   ├── exemptprioritylevelconfiguration.go
│   │   │   ├── flowdistinguishermethod.go
│   │   │   ├── flowschema.go
│   │   │   ├── flowschemacondition.go
│   │   │   ├── flowschemaspec.go
│   │   │   ├── flowschemastatus.go
│   │   │   ├── groupsubject.go
│   │   │   ├── limitedprioritylevelconfiguration.go
│   │   │   ├── limitresponse.go
│   │   │   ├── nonresourcepolicyrule.go
│   │   │   ├── policyruleswithsubjects.go
│   │   │   ├── prioritylevelconfiguration.go
│   │   │   ├── prioritylevelconfigurationcondition.go
│   │   │   ├── prioritylevelconfigurationreference.go
│   │   │   ├── prioritylevelconfigurationspec.go
│   │   │   ├── prioritylevelconfigurationstatus.go
│   │   │   ├── queuingconfiguration.go
│   │   │   ├── resourcepolicyrule.go
│   │   │   ├── serviceaccountsubject.go
│   │   │   ├── subject.go
│   │   │   └── usersubject.go
│   │   ├── v1beta2
│   │   │   ├── exemptprioritylevelconfiguration.go
│   │   │   ├── flowdistinguishermethod.go
│   │   │   ├── flowschema.go
│   │   │   ├── flowschemacondition.go
│   │   │   ├── flowschemaspec.go
│   │   │   ├── flowschemastatus.go
│   │   │   ├── groupsubject.go
│   │   │   ├── limitedprioritylevelconfiguration.go
│   │   │   ├── limitresponse.go
│   │   │   ├── nonresourcepolicyrule.go
│   │   │   ├── policyruleswithsubjects.go
│   │   │   ├── prioritylevelconfiguration.go
│   │   │   ├── prioritylevelconfigurationcondition.go
│   │   │   ├── prioritylevelconfigurationreference.go
│   │   │   ├── prioritylevelconfigurationspec.go
│   │   │   ├── prioritylevelconfigurationstatus.go
│   │   │   ├── queuingconfiguration.go
│   │   │   ├── resourcepolicyrule.go
│   │   │   ├── serviceaccountsubject.go
│   │   │   ├── subject.go
│   │   │   └── usersubject.go
│   │   └── v1beta3
│   │       ├── exemptprioritylevelconfiguration.go
│   │       ├── flowdistinguishermethod.go
│   │       ├── flowschema.go
│   │       ├── flowschemacondition.go
│   │       ├── flowschemaspec.go
│   │       ├── flowschemastatus.go
│   │       ├── groupsubject.go
│   │       ├── limitedprioritylevelconfiguration.go
│   │       ├── limitresponse.go
│   │       ├── nonresourcepolicyrule.go
│   │       ├── policyruleswithsubjects.go
│   │       ├── prioritylevelconfiguration.go
│   │       ├── prioritylevelconfigurationcondition.go
│   │       ├── prioritylevelconfigurationreference.go
│   │       ├── prioritylevelconfigurationspec.go
│   │       ├── prioritylevelconfigurationstatus.go
│   │       ├── queuingconfiguration.go
│   │       ├── resourcepolicyrule.go
│   │       ├── serviceaccountsubject.go
│   │       ├── subject.go
│   │       └── usersubject.go
│   ├── imagepolicy
│   │   └── v1alpha1
│   │       ├── imagereview.go
│   │       ├── imagereviewcontainerspec.go
│   │       ├── imagereviewspec.go
│   │       └── imagereviewstatus.go
│   ├── internal
│   │   └── internal.go
│   ├── meta
│   │   └── v1
│   │       ├── condition.go
│   │       ├── deleteoptions.go
│   │       ├── groupresource.go
│   │       ├── labelselector.go
│   │       ├── labelselectorrequirement.go
│   │       ├── managedfieldsentry.go
│   │       ├── objectmeta.go
│   │       ├── ownerreference.go
│   │       ├── preconditions.go
│   │       ├── typemeta.go
│   │       └── unstructured.go
│   ├── networking
│   │   ├── v1
│   │   │   ├── httpingresspath.go
│   │   │   ├── httpingressrulevalue.go
│   │   │   ├── ingress.go
│   │   │   ├── ingressbackend.go
│   │   │   ├── ingressclass.go
│   │   │   ├── ingressclassparametersreference.go
│   │   │   ├── ingressclassspec.go
│   │   │   ├── ingressloadbalanceringress.go
│   │   │   ├── ingressloadbalancerstatus.go
│   │   │   ├── ingressportstatus.go
│   │   │   ├── ingressrule.go
│   │   │   ├── ingressrulevalue.go
│   │   │   ├── ingressservicebackend.go
│   │   │   ├── ingressspec.go
│   │   │   ├── ingressstatus.go
│   │   │   ├── ingresstls.go
│   │   │   ├── ipaddress.go
│   │   │   ├── ipaddressspec.go
│   │   │   ├── ipblock.go
│   │   │   ├── networkpolicy.go
│   │   │   ├── networkpolicyegressrule.go
│   │   │   ├── networkpolicyingressrule.go
│   │   │   ├── networkpolicypeer.go
│   │   │   ├── networkpolicyport.go
│   │   │   ├── networkpolicyspec.go
│   │   │   ├── parentreference.go
│   │   │   ├── servicebackendport.go
│   │   │   ├── servicecidr.go
│   │   │   ├── servicecidrspec.go
│   │   │   └── servicecidrstatus.go
│   │   └── v1beta1
│   │       ├── httpingresspath.go
│   │       ├── httpingressrulevalue.go
│   │       ├── ingress.go
│   │       ├── ingressbackend.go
│   │       ├── ingressclass.go
│   │       ├── ingressclassparametersreference.go
│   │       ├── ingressclassspec.go
│   │       ├── ingressloadbalanceringress.go
│   │       ├── ingressloadbalancerstatus.go
│   │       ├── ingressportstatus.go
│   │       ├── ingressrule.go
│   │       ├── ingressrulevalue.go
│   │       ├── ingressspec.go
│   │       ├── ingressstatus.go
│   │       ├── ingresstls.go
│   │       ├── ipaddress.go
│   │       ├── ipaddressspec.go
│   │       ├── parentreference.go
│   │       ├── servicecidr.go
│   │       ├── servicecidrspec.go
│   │       └── servicecidrstatus.go
│   ├── node
│   │   ├── v1
│   │   │   ├── overhead.go
│   │   │   ├── runtimeclass.go
│   │   │   └── scheduling.go
│   │   ├── v1alpha1
│   │   │   ├── overhead.go
│   │   │   ├── runtimeclass.go
│   │   │   ├── runtimeclassspec.go
│   │   │   └── scheduling.go
│   │   └── v1beta1
│   │       ├── overhead.go
│   │       ├── runtimeclass.go
│   │       └── scheduling.go
│   ├── policy
│   │   ├── v1
│   │   │   ├── eviction.go
│   │   │   ├── poddisruptionbudget.go
│   │   │   ├── poddisruptionbudgetspec.go
│   │   │   └── poddisruptionbudgetstatus.go
│   │   └── v1beta1
│   │       ├── eviction.go
│   │       ├── poddisruptionbudget.go
│   │       ├── poddisruptionbudgetspec.go
│   │       └── poddisruptionbudgetstatus.go
│   ├── rbac
│   │   ├── v1
│   │   │   ├── aggregationrule.go
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── policyrule.go
│   │   │   ├── role.go
│   │   │   ├── rolebinding.go
│   │   │   ├── roleref.go
│   │   │   └── subject.go
│   │   ├── v1alpha1
│   │   │   ├── aggregationrule.go
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── policyrule.go
│   │   │   ├── role.go
│   │   │   ├── rolebinding.go
│   │   │   ├── roleref.go
│   │   │   └── subject.go
│   │   └── v1beta1
│   │       ├── aggregationrule.go
│   │       ├── clusterrole.go
│   │       ├── clusterrolebinding.go
│   │       ├── policyrule.go
│   │       ├── role.go
│   │       ├── rolebinding.go
│   │       ├── roleref.go
│   │       └── subject.go
│   ├── resource
│   │   ├── v1
│   │   │   ├── allocateddevicestatus.go
│   │   │   ├── allocationresult.go
│   │   │   ├── capacityrequestpolicy.go
│   │   │   ├── capacityrequestpolicyrange.go
│   │   │   ├── capacityrequirements.go
│   │   │   ├── celdeviceselector.go
│   │   │   ├── counter.go
│   │   │   ├── counterset.go
│   │   │   ├── device.go
│   │   │   ├── deviceallocationconfiguration.go
│   │   │   ├── deviceallocationresult.go
│   │   │   ├── deviceattribute.go
│   │   │   ├── devicecapacity.go
│   │   │   ├── deviceclaim.go
│   │   │   ├── deviceclaimconfiguration.go
│   │   │   ├── deviceclass.go
│   │   │   ├── deviceclassconfiguration.go
│   │   │   ├── deviceclassspec.go
│   │   │   ├── deviceconfiguration.go
│   │   │   ├── deviceconstraint.go
│   │   │   ├── devicecounterconsumption.go
│   │   │   ├── devicerequest.go
│   │   │   ├── devicerequestallocationresult.go
│   │   │   ├── deviceselector.go
│   │   │   ├── devicesubrequest.go
│   │   │   ├── devicetaint.go
│   │   │   ├── devicetoleration.go
│   │   │   ├── exactdevicerequest.go
│   │   │   ├── networkdevicedata.go
│   │   │   ├── nodeallocatableresourcemapping.go
│   │   │   ├── opaquedeviceconfiguration.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimconsumerreference.go
│   │   │   ├── resourceclaimspec.go
│   │   │   ├── resourceclaimstatus.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   ├── resourceclaimtemplatespec.go
│   │   │   ├── resourcepool.go
│   │   │   ├── resourceslice.go
│   │   │   └── resourceslicespec.go
│   │   ├── v1alpha3
│   │   │   ├── devicetaint.go
│   │   │   ├── devicetaintrule.go
│   │   │   ├── devicetaintrulespec.go
│   │   │   ├── devicetaintrulestatus.go
│   │   │   ├── devicetaintselector.go
│   │   │   ├── poolstatus.go
│   │   │   ├── resourcepoolstatusrequest.go
│   │   │   ├── resourcepoolstatusrequestspec.go
│   │   │   └── resourcepoolstatusrequeststatus.go
│   │   ├── v1beta1
│   │   │   ├── allocateddevicestatus.go
│   │   │   ├── allocationresult.go
│   │   │   ├── basicdevice.go
│   │   │   ├── capacityrequestpolicy.go
│   │   │   ├── capacityrequestpolicyrange.go
│   │   │   ├── capacityrequirements.go
│   │   │   ├── celdeviceselector.go
│   │   │   ├── counter.go
│   │   │   ├── counterset.go
│   │   │   ├── device.go
│   │   │   ├── deviceallocationconfiguration.go
│   │   │   ├── deviceallocationresult.go
│   │   │   ├── deviceattribute.go
│   │   │   ├── devicecapacity.go
│   │   │   ├── deviceclaim.go
│   │   │   ├── deviceclaimconfiguration.go
│   │   │   ├── deviceclass.go
│   │   │   ├── deviceclassconfiguration.go
│   │   │   ├── deviceclassspec.go
│   │   │   ├── deviceconfiguration.go
│   │   │   ├── deviceconstraint.go
│   │   │   ├── devicecounterconsumption.go
│   │   │   ├── devicerequest.go
│   │   │   ├── devicerequestallocationresult.go
│   │   │   ├── deviceselector.go
│   │   │   ├── devicesubrequest.go
│   │   │   ├── devicetaint.go
│   │   │   ├── devicetoleration.go
│   │   │   ├── networkdevicedata.go
│   │   │   ├── nodeallocatableresourcemapping.go
│   │   │   ├── opaquedeviceconfiguration.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimconsumerreference.go
│   │   │   ├── resourceclaimspec.go
│   │   │   ├── resourceclaimstatus.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   ├── resourceclaimtemplatespec.go
│   │   │   ├── resourcepool.go
│   │   │   ├── resourceslice.go
│   │   │   └── resourceslicespec.go
│   │   └── v1beta2
│   │       ├── allocateddevicestatus.go
│   │       ├── allocationresult.go
│   │       ├── capacityrequestpolicy.go
│   │       ├── capacityrequestpolicyrange.go
│   │       ├── capacityrequirements.go
│   │       ├── celdeviceselector.go
│   │       ├── counter.go
│   │       ├── counterset.go
│   │       ├── device.go
│   │       ├── deviceallocationconfiguration.go
│   │       ├── deviceallocationresult.go
│   │       ├── deviceattribute.go
│   │       ├── devicecapacity.go
│   │       ├── deviceclaim.go
│   │       ├── deviceclaimconfiguration.go
│   │       ├── deviceclass.go
│   │       ├── deviceclassconfiguration.go
│   │       ├── deviceclassspec.go
│   │       ├── deviceconfiguration.go
│   │       ├── deviceconstraint.go
│   │       ├── devicecounterconsumption.go
│   │       ├── devicerequest.go
│   │       ├── devicerequestallocationresult.go
│   │       ├── deviceselector.go
│   │       ├── devicesubrequest.go
│   │       ├── devicetaint.go
│   │       ├── devicetaintrule.go
│   │       ├── devicetaintrulespec.go
│   │       ├── devicetaintrulestatus.go
│   │       ├── devicetaintselector.go
│   │       ├── devicetoleration.go
│   │       ├── exactdevicerequest.go
│   │       ├── networkdevicedata.go
│   │       ├── nodeallocatableresourcemapping.go
│   │       ├── opaquedeviceconfiguration.go
│   │       ├── resourceclaim.go
│   │       ├── resourceclaimconsumerreference.go
│   │       ├── resourceclaimspec.go
│   │       ├── resourceclaimstatus.go
│   │       ├── resourceclaimtemplate.go
│   │       ├── resourceclaimtemplatespec.go
│   │       ├── resourcepool.go
│   │       ├── resourceslice.go
│   │       └── resourceslicespec.go
│   ├── scheduling
│   │   ├── v1
│   │   │   └── priorityclass.go
│   │   ├── v1alpha3
│   │   │   ├── disruptionmode.go
│   │   │   ├── gangschedulingpolicy.go
│   │   │   ├── podgroup.go
│   │   │   ├── podgroupresourceclaim.go
│   │   │   ├── podgroupresourceclaimstatus.go
│   │   │   ├── podgroupschedulingconstraints.go
│   │   │   ├── podgroupschedulingpolicy.go
│   │   │   ├── podgroupspec.go
│   │   │   ├── podgroupstatus.go
│   │   │   ├── podgrouptemplate.go
│   │   │   ├── podgrouptemplatereference.go
│   │   │   ├── topologyconstraint.go
│   │   │   ├── typedlocalobjectreference.go
│   │   │   ├── workload.go
│   │   │   ├── workloadpodgrouptemplatereference.go
│   │   │   └── workloadspec.go
│   │   └── v1beta1
│   │       └── priorityclass.go
│   ├── storage
│   │   ├── v1
│   │   │   ├── csidriver.go
│   │   │   ├── csidriverspec.go
│   │   │   ├── csinode.go
│   │   │   ├── csinodedriver.go
│   │   │   ├── csinodespec.go
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── storageclass.go
│   │   │   ├── tokenrequest.go
│   │   │   ├── volumeattachment.go
│   │   │   ├── volumeattachmentsource.go
│   │   │   ├── volumeattachmentspec.go
│   │   │   ├── volumeattachmentstatus.go
│   │   │   ├── volumeattributesclass.go
│   │   │   ├── volumeerror.go
│   │   │   └── volumenoderesources.go
│   │   ├── v1alpha1
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── volumeattachment.go
│   │   │   ├── volumeattachmentsource.go
│   │   │   ├── volumeattachmentspec.go
│   │   │   ├── volumeattachmentstatus.go
│   │   │   ├── volumeattributesclass.go
│   │   │   └── volumeerror.go
│   │   └── v1beta1
│   │       ├── csidriver.go
│   │       ├── csidriverspec.go
│   │       ├── csinode.go
│   │       ├── csinodedriver.go
│   │       ├── csinodespec.go
│   │       ├── csistoragecapacity.go
│   │       ├── storageclass.go
│   │       ├── tokenrequest.go
│   │       ├── volumeattachment.go
│   │       ├── volumeattachmentsource.go
│   │       ├── volumeattachmentspec.go
│   │       ├── volumeattachmentstatus.go
│   │       ├── volumeattributesclass.go
│   │       ├── volumeerror.go
│   │       └── volumenoderesources.go
│   ├── storagemigration
│   │   └── v1beta1
│   │       ├── storageversionmigration.go
│   │       ├── storageversionmigrationspec.go
│   │       └── storageversionmigrationstatus.go
│   ├── OWNERS
│   ├── doc.go
│   └── utils.go
├── discovery
│   ├── cached
│   │   ├── disk
│   │   │   ├── cached_discovery.go
│   │   │   └── round_tripper.go
│   │   ├── memory
│   │   │   └── memcache.go
│   │   └── legacy.go
│   ├── fake
│   │   └── discovery.go
│   ├── aggregated_discovery.go
│   ├── discovery_client.go
│   ├── doc.go
│   └── helper.go
├── dynamic
│   ├── dynamicinformer
│   │   ├── informer.go
│   │   └── interface.go
│   ├── dynamiclister
│   │   ├── interface.go
│   │   ├── lister.go
│   │   └── shim.go
│   ├── fake
│   │   └── simple.go
│   ├── interface.go
│   ├── scheme.go
│   └── simple.go
├── examples
│   ├── create-update-delete-deployment
│   │   ├── README.md
│   │   └── main.go
│   ├── dynamic-create-update-delete-deployment
│   │   ├── README.md
│   │   └── main.go
│   ├── fake-client
│   │   ├── README.md
│   │   └── doc.go
│   ├── in-cluster-client-configuration
│   │   ├── Dockerfile
│   │   ├── README.md
│   │   └── main.go
│   ├── leader-election
│   │   ├── README.md
│   │   └── main.go
│   ├── out-of-cluster-client-configuration
│   │   ├── README.md
│   │   └── main.go
│   ├── workqueue
│   │   ├── README.md
│   │   └── main.go
│   └── README.md
├── features
│   ├── testing
│   │   └── features.go
│   ├── envvar.go
│   ├── features.go
│   └── known_features.go
├── gentype
│   ├── fake.go
│   └── type.go
├── informers
│   ├── admissionregistration
│   │   ├── v1
│   │   │   ├── interface.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── mutatingwebhookconfiguration.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   ├── validatingadmissionpolicybinding.go
│   │   │   └── validatingwebhookconfiguration.go
│   │   ├── v1alpha1
│   │   │   ├── interface.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   └── validatingadmissionpolicybinding.go
│   │   ├── v1beta1
│   │   │   ├── interface.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── mutatingwebhookconfiguration.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   ├── validatingadmissionpolicybinding.go
│   │   │   └── validatingwebhookconfiguration.go
│   │   └── interface.go
│   ├── apiserverinternal
│   │   ├── v1alpha1
│   │   │   ├── interface.go
│   │   │   └── storageversion.go
│   │   └── interface.go
│   ├── apps
│   │   ├── v1
│   │   │   ├── controllerrevision.go
│   │   │   ├── daemonset.go
│   │   │   ├── deployment.go
│   │   │   ├── interface.go
│   │   │   ├── replicaset.go
│   │   │   └── statefulset.go
│   │   ├── v1beta1
│   │   │   ├── controllerrevision.go
│   │   │   ├── deployment.go
│   │   │   ├── interface.go
│   │   │   └── statefulset.go
│   │   ├── v1beta2
│   │   │   ├── controllerrevision.go
│   │   │   ├── daemonset.go
│   │   │   ├── deployment.go
│   │   │   ├── interface.go
│   │   │   ├── replicaset.go
│   │   │   └── statefulset.go
│   │   └── interface.go
│   ├── autoscaling
│   │   ├── v1
│   │   │   ├── horizontalpodautoscaler.go
│   │   │   └── interface.go
│   │   ├── v2
│   │   │   ├── horizontalpodautoscaler.go
│   │   │   └── interface.go
│   │   └── interface.go
│   ├── batch
│   │   ├── v1
│   │   │   ├── cronjob.go
│   │   │   ├── interface.go
│   │   │   └── job.go
│   │   ├── v1beta1
│   │   │   ├── cronjob.go
│   │   │   └── interface.go
│   │   └── interface.go
│   ├── certificates
│   │   ├── v1
│   │   │   ├── certificatesigningrequest.go
│   │   │   └── interface.go
│   │   ├── v1alpha1
│   │   │   ├── clustertrustbundle.go
│   │   │   └── interface.go
│   │   ├── v1beta1
│   │   │   ├── certificatesigningrequest.go
│   │   │   ├── clustertrustbundle.go
│   │   │   ├── interface.go
│   │   │   └── podcertificaterequest.go
│   │   └── interface.go
│   ├── coordination
│   │   ├── v1
│   │   │   ├── interface.go
│   │   │   └── lease.go
│   │   ├── v1alpha2
│   │   │   ├── interface.go
│   │   │   └── leasecandidate.go
│   │   ├── v1beta1
│   │   │   ├── interface.go
│   │   │   ├── lease.go
│   │   │   └── leasecandidate.go
│   │   └── interface.go
│   ├── core
│   │   ├── v1
│   │   │   ├── componentstatus.go
│   │   │   ├── configmap.go
│   │   │   ├── endpoints.go
│   │   │   ├── event.go
│   │   │   ├── interface.go
│   │   │   ├── limitrange.go
│   │   │   ├── namespace.go
│   │   │   ├── node.go
│   │   │   ├── persistentvolume.go
│   │   │   ├── persistentvolumeclaim.go
│   │   │   ├── pod.go
│   │   │   ├── podtemplate.go
│   │   │   ├── replicationcontroller.go
│   │   │   ├── resourcequota.go
│   │   │   ├── secret.go
│   │   │   ├── service.go
│   │   │   └── serviceaccount.go
│   │   └── interface.go
│   ├── discovery
│   │   ├── v1
│   │   │   ├── endpointslice.go
│   │   │   └── interface.go
│   │   ├── v1beta1
│   │   │   ├── endpointslice.go
│   │   │   └── interface.go
│   │   └── interface.go
│   ├── events
│   │   ├── v1
│   │   │   ├── event.go
│   │   │   └── interface.go
│   │   ├── v1beta1
│   │   │   ├── event.go
│   │   │   └── interface.go
│   │   └── interface.go
│   ├── extensions
│   │   ├── v1beta1
│   │   │   ├── daemonset.go
│   │   │   ├── deployment.go
│   │   │   ├── ingress.go
│   │   │   ├── interface.go
│   │   │   ├── networkpolicy.go
│   │   │   └── replicaset.go
│   │   └── interface.go
│   ├── flowcontrol
│   │   ├── v1
│   │   │   ├── flowschema.go
│   │   │   ├── interface.go
│   │   │   └── prioritylevelconfiguration.go
│   │   ├── v1beta1
│   │   │   ├── flowschema.go
│   │   │   ├── interface.go
│   │   │   └── prioritylevelconfiguration.go
│   │   ├── v1beta2
│   │   │   ├── flowschema.go
│   │   │   ├── interface.go
│   │   │   └── prioritylevelconfiguration.go
│   │   ├── v1beta3
│   │   │   ├── flowschema.go
│   │   │   ├── interface.go
│   │   │   └── prioritylevelconfiguration.go
│   │   └── interface.go
│   ├── internalinterfaces
│   │   └── factory_interfaces.go
│   ├── networking
│   │   ├── v1
│   │   │   ├── ingress.go
│   │   │   ├── ingressclass.go
│   │   │   ├── interface.go
│   │   │   ├── ipaddress.go
│   │   │   ├── networkpolicy.go
│   │   │   └── servicecidr.go
│   │   ├── v1beta1
│   │   │   ├── ingress.go
│   │   │   ├── ingressclass.go
│   │   │   ├── interface.go
│   │   │   ├── ipaddress.go
│   │   │   └── servicecidr.go
│   │   └── interface.go
│   ├── node
│   │   ├── v1
│   │   │   ├── interface.go
│   │   │   └── runtimeclass.go
│   │   ├── v1alpha1
│   │   │   ├── interface.go
│   │   │   └── runtimeclass.go
│   │   ├── v1beta1
│   │   │   ├── interface.go
│   │   │   └── runtimeclass.go
│   │   └── interface.go
│   ├── policy
│   │   ├── v1
│   │   │   ├── interface.go
│   │   │   └── poddisruptionbudget.go
│   │   ├── v1beta1
│   │   │   ├── interface.go
│   │   │   └── poddisruptionbudget.go
│   │   └── interface.go
│   ├── rbac
│   │   ├── v1
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── interface.go
│   │   │   ├── role.go
│   │   │   └── rolebinding.go
│   │   ├── v1alpha1
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── interface.go
│   │   │   ├── role.go
│   │   │   └── rolebinding.go
│   │   ├── v1beta1
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── interface.go
│   │   │   ├── role.go
│   │   │   └── rolebinding.go
│   │   └── interface.go
│   ├── resource
│   │   ├── v1
│   │   │   ├── deviceclass.go
│   │   │   ├── interface.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   └── resourceslice.go
│   │   ├── v1alpha3
│   │   │   ├── devicetaintrule.go
│   │   │   ├── interface.go
│   │   │   └── resourcepoolstatusrequest.go
│   │   ├── v1beta1
│   │   │   ├── deviceclass.go
│   │   │   ├── interface.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   └── resourceslice.go
│   │   ├── v1beta2
│   │   │   ├── deviceclass.go
│   │   │   ├── devicetaintrule.go
│   │   │   ├── interface.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   └── resourceslice.go
│   │   └── interface.go
│   ├── scheduling
│   │   ├── v1
│   │   │   ├── interface.go
│   │   │   └── priorityclass.go
│   │   ├── v1alpha3
│   │   │   ├── interface.go
│   │   │   ├── podgroup.go
│   │   │   └── workload.go
│   │   ├── v1beta1
│   │   │   ├── interface.go
│   │   │   └── priorityclass.go
│   │   └── interface.go
│   ├── storage
│   │   ├── v1
│   │   │   ├── csidriver.go
│   │   │   ├── csinode.go
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── interface.go
│   │   │   ├── storageclass.go
│   │   │   ├── volumeattachment.go
│   │   │   └── volumeattributesclass.go
│   │   ├── v1alpha1
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── interface.go
│   │   │   ├── volumeattachment.go
│   │   │   └── volumeattributesclass.go
│   │   ├── v1beta1
│   │   │   ├── csidriver.go
│   │   │   ├── csinode.go
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── interface.go
│   │   │   ├── storageclass.go
│   │   │   ├── volumeattachment.go
│   │   │   └── volumeattributesclass.go
│   │   └── interface.go
│   ├── storagemigration
│   │   ├── v1beta1
│   │   │   ├── interface.go
│   │   │   └── storageversionmigration.go
│   │   └── interface.go
│   ├── doc.go
│   ├── factory.go
│   └── generic.go
├── kubernetes
│   ├── fake
│   │   ├── clientset_generated.go
│   │   ├── doc.go
│   │   └── register.go
│   ├── scheme
│   │   ├── doc.go
│   │   └── register.go
│   ├── typed
│   │   ├── admissionregistration
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_admissionregistration_client.go
│   │   │   │   │   ├── fake_mutatingadmissionpolicy.go
│   │   │   │   │   ├── fake_mutatingadmissionpolicybinding.go
│   │   │   │   │   ├── fake_mutatingwebhookconfiguration.go
│   │   │   │   │   ├── fake_validatingadmissionpolicy.go
│   │   │   │   │   ├── fake_validatingadmissionpolicybinding.go
│   │   │   │   │   └── fake_validatingwebhookconfiguration.go
│   │   │   │   ├── admissionregistration_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── mutatingadmissionpolicy.go
│   │   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   │   ├── mutatingwebhookconfiguration.go
│   │   │   │   ├── validatingadmissionpolicy.go
│   │   │   │   ├── validatingadmissionpolicybinding.go
│   │   │   │   └── validatingwebhookconfiguration.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_admissionregistration_client.go
│   │   │   │   │   ├── fake_mutatingadmissionpolicy.go
│   │   │   │   │   ├── fake_mutatingadmissionpolicybinding.go
│   │   │   │   │   ├── fake_validatingadmissionpolicy.go
│   │   │   │   │   └── fake_validatingadmissionpolicybinding.go
│   │   │   │   ├── admissionregistration_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── mutatingadmissionpolicy.go
│   │   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   │   ├── validatingadmissionpolicy.go
│   │   │   │   └── validatingadmissionpolicybinding.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_admissionregistration_client.go
│   │   │       │   ├── fake_mutatingadmissionpolicy.go
│   │   │       │   ├── fake_mutatingadmissionpolicybinding.go
│   │   │       │   ├── fake_mutatingwebhookconfiguration.go
│   │   │       │   ├── fake_validatingadmissionpolicy.go
│   │   │       │   ├── fake_validatingadmissionpolicybinding.go
│   │   │       │   └── fake_validatingwebhookconfiguration.go
│   │   │       ├── admissionregistration_client.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── mutatingadmissionpolicy.go
│   │   │       ├── mutatingadmissionpolicybinding.go
│   │   │       ├── mutatingwebhookconfiguration.go
│   │   │       ├── validatingadmissionpolicy.go
│   │   │       ├── validatingadmissionpolicybinding.go
│   │   │       └── validatingwebhookconfiguration.go
│   │   ├── apiserverinternal
│   │   │   └── v1alpha1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_apiserverinternal_client.go
│   │   │       │   └── fake_storageversion.go
│   │   │       ├── apiserverinternal_client.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       └── storageversion.go
│   │   ├── apps
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_apps_client.go
│   │   │   │   │   ├── fake_controllerrevision.go
│   │   │   │   │   ├── fake_daemonset.go
│   │   │   │   │   ├── fake_deployment.go
│   │   │   │   │   ├── fake_replicaset.go
│   │   │   │   │   └── fake_statefulset.go
│   │   │   │   ├── apps_client.go
│   │   │   │   ├── controllerrevision.go
│   │   │   │   ├── daemonset.go
│   │   │   │   ├── deployment.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── replicaset.go
│   │   │   │   └── statefulset.go
│   │   │   ├── v1beta1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_apps_client.go
│   │   │   │   │   ├── fake_controllerrevision.go
│   │   │   │   │   ├── fake_deployment.go
│   │   │   │   │   └── fake_statefulset.go
│   │   │   │   ├── apps_client.go
│   │   │   │   ├── controllerrevision.go
│   │   │   │   ├── deployment.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── statefulset.go
│   │   │   └── v1beta2
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_apps_client.go
│   │   │       │   ├── fake_controllerrevision.go
│   │   │       │   ├── fake_daemonset.go
│   │   │       │   ├── fake_deployment.go
│   │   │       │   ├── fake_replicaset.go
│   │   │       │   └── fake_statefulset.go
│   │   │       ├── apps_client.go
│   │   │       ├── controllerrevision.go
│   │   │       ├── daemonset.go
│   │   │       ├── deployment.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── replicaset.go
│   │   │       └── statefulset.go
│   │   ├── authentication
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_authentication_client.go
│   │   │   │   │   ├── fake_selfsubjectreview.go
│   │   │   │   │   └── fake_tokenreview.go
│   │   │   │   ├── authentication_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── selfsubjectreview.go
│   │   │   │   └── tokenreview.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_authentication_client.go
│   │   │   │   │   └── fake_selfsubjectreview.go
│   │   │   │   ├── authentication_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── selfsubjectreview.go
│   │   │   ├── v1beta1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_authentication_client.go
│   │   │   │   │   ├── fake_selfsubjectreview.go
│   │   │   │   │   └── fake_tokenreview.go
│   │   │   │   ├── authentication_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── selfsubjectreview.go
│   │   │   │   └── tokenreview.go
│   │   │   └── OWNERS
│   │   ├── authorization
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_authorization_client.go
│   │   │   │   │   ├── fake_localsubjectaccessreview.go
│   │   │   │   │   ├── fake_selfsubjectaccessreview.go
│   │   │   │   │   ├── fake_selfsubjectrulesreview.go
│   │   │   │   │   └── fake_subjectaccessreview.go
│   │   │   │   ├── authorization_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── localsubjectaccessreview.go
│   │   │   │   ├── selfsubjectaccessreview.go
│   │   │   │   ├── selfsubjectrulesreview.go
│   │   │   │   └── subjectaccessreview.go
│   │   │   ├── v1beta1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_authorization_client.go
│   │   │   │   │   ├── fake_localsubjectaccessreview.go
│   │   │   │   │   ├── fake_selfsubjectaccessreview.go
│   │   │   │   │   ├── fake_selfsubjectrulesreview.go
│   │   │   │   │   └── fake_subjectaccessreview.go
│   │   │   │   ├── authorization_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── localsubjectaccessreview.go
│   │   │   │   ├── selfsubjectaccessreview.go
│   │   │   │   ├── selfsubjectrulesreview.go
│   │   │   │   └── subjectaccessreview.go
│   │   │   └── OWNERS
│   │   ├── autoscaling
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_autoscaling_client.go
│   │   │   │   │   └── fake_horizontalpodautoscaler.go
│   │   │   │   ├── autoscaling_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── horizontalpodautoscaler.go
│   │   │   └── v2
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_autoscaling_client.go
│   │   │       │   └── fake_horizontalpodautoscaler.go
│   │   │       ├── autoscaling_client.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       └── horizontalpodautoscaler.go
│   │   ├── batch
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_batch_client.go
│   │   │   │   │   ├── fake_cronjob.go
│   │   │   │   │   └── fake_job.go
│   │   │   │   ├── batch_client.go
│   │   │   │   ├── cronjob.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── job.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_batch_client.go
│   │   │       │   └── fake_cronjob.go
│   │   │       ├── batch_client.go
│   │   │       ├── cronjob.go
│   │   │       ├── doc.go
│   │   │       └── generated_expansion.go
│   │   ├── certificates
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_certificates_client.go
│   │   │   │   │   └── fake_certificatesigningrequest.go
│   │   │   │   ├── certificates_client.go
│   │   │   │   ├── certificatesigningrequest.go
│   │   │   │   ├── doc.go
│   │   │   │   └── generated_expansion.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_certificates_client.go
│   │   │   │   │   └── fake_clustertrustbundle.go
│   │   │   │   ├── certificates_client.go
│   │   │   │   ├── clustertrustbundle.go
│   │   │   │   ├── doc.go
│   │   │   │   └── generated_expansion.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_certificates_client.go
│   │   │       │   ├── fake_certificatesigningrequest.go
│   │   │       │   ├── fake_certificatesigningrequest_expansion.go
│   │   │       │   ├── fake_clustertrustbundle.go
│   │   │       │   └── fake_podcertificaterequest.go
│   │   │       ├── certificates_client.go
│   │   │       ├── certificatesigningrequest.go
│   │   │       ├── certificatesigningrequest_expansion.go
│   │   │       ├── clustertrustbundle.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       └── podcertificaterequest.go
│   │   ├── coordination
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_coordination_client.go
│   │   │   │   │   └── fake_lease.go
│   │   │   │   ├── coordination_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── lease.go
│   │   │   ├── v1alpha2
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_coordination_client.go
│   │   │   │   │   └── fake_leasecandidate.go
│   │   │   │   ├── coordination_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── leasecandidate.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_coordination_client.go
│   │   │       │   ├── fake_lease.go
│   │   │       │   └── fake_leasecandidate.go
│   │   │       ├── coordination_client.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── lease.go
│   │   │       └── leasecandidate.go
│   │   ├── core
│   │   │   └── v1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_componentstatus.go
│   │   │       │   ├── fake_configmap.go
│   │   │       │   ├── fake_core_client.go
│   │   │       │   ├── fake_endpoints.go
│   │   │       │   ├── fake_event.go
│   │   │       │   ├── fake_event_expansion.go
│   │   │       │   ├── fake_limitrange.go
│   │   │       │   ├── fake_namespace.go
│   │   │       │   ├── fake_namespace_expansion.go
│   │   │       │   ├── fake_node.go
│   │   │       │   ├── fake_node_expansion.go
│   │   │       │   ├── fake_persistentvolume.go
│   │   │       │   ├── fake_persistentvolumeclaim.go
│   │   │       │   ├── fake_pod.go
│   │   │       │   ├── fake_pod_expansion.go
│   │   │       │   ├── fake_podtemplate.go
│   │   │       │   ├── fake_replicationcontroller.go
│   │   │       │   ├── fake_resourcequota.go
│   │   │       │   ├── fake_secret.go
│   │   │       │   ├── fake_service.go
│   │   │       │   ├── fake_service_expansion.go
│   │   │       │   └── fake_serviceaccount.go
│   │   │       ├── componentstatus.go
│   │   │       ├── configmap.go
│   │   │       ├── core_client.go
│   │   │       ├── doc.go
│   │   │       ├── endpoints.go
│   │   │       ├── event.go
│   │   │       ├── event_expansion.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── limitrange.go
│   │   │       ├── namespace.go
│   │   │       ├── namespace_expansion.go
│   │   │       ├── node.go
│   │   │       ├── node_expansion.go
│   │   │       ├── persistentvolume.go
│   │   │       ├── persistentvolumeclaim.go
│   │   │       ├── pod.go
│   │   │       ├── pod_expansion.go
│   │   │       ├── podtemplate.go
│   │   │       ├── replicationcontroller.go
│   │   │       ├── resourcequota.go
│   │   │       ├── secret.go
│   │   │       ├── service.go
│   │   │       ├── service_expansion.go
│   │   │       └── serviceaccount.go
│   │   ├── discovery
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_discovery_client.go
│   │   │   │   │   └── fake_endpointslice.go
│   │   │   │   ├── discovery_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── endpointslice.go
│   │   │   │   └── generated_expansion.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_discovery_client.go
│   │   │       │   └── fake_endpointslice.go
│   │   │       ├── discovery_client.go
│   │   │       ├── doc.go
│   │   │       ├── endpointslice.go
│   │   │       └── generated_expansion.go
│   │   ├── events
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_event.go
│   │   │   │   │   └── fake_events_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── event.go
│   │   │   │   ├── events_client.go
│   │   │   │   └── generated_expansion.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_event.go
│   │   │       │   ├── fake_event_expansion.go
│   │   │       │   └── fake_events_client.go
│   │   │       ├── doc.go
│   │   │       ├── event.go
│   │   │       ├── event_expansion.go
│   │   │       ├── events_client.go
│   │   │       └── generated_expansion.go
│   │   ├── extensions
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_daemonset.go
│   │   │       │   ├── fake_deployment.go
│   │   │       │   ├── fake_deployment_expansion.go
│   │   │       │   ├── fake_extensions_client.go
│   │   │       │   ├── fake_ingress.go
│   │   │       │   ├── fake_networkpolicy.go
│   │   │       │   └── fake_replicaset.go
│   │   │       ├── daemonset.go
│   │   │       ├── deployment.go
│   │   │       ├── deployment_expansion.go
│   │   │       ├── doc.go
│   │   │       ├── extensions_client.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── ingress.go
│   │   │       ├── networkpolicy.go
│   │   │       └── replicaset.go
│   │   ├── flowcontrol
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_flowcontrol_client.go
│   │   │   │   │   ├── fake_flowschema.go
│   │   │   │   │   └── fake_prioritylevelconfiguration.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── flowcontrol_client.go
│   │   │   │   ├── flowschema.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── prioritylevelconfiguration.go
│   │   │   ├── v1beta1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_flowcontrol_client.go
│   │   │   │   │   ├── fake_flowschema.go
│   │   │   │   │   └── fake_prioritylevelconfiguration.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── flowcontrol_client.go
│   │   │   │   ├── flowschema.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── prioritylevelconfiguration.go
│   │   │   ├── v1beta2
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_flowcontrol_client.go
│   │   │   │   │   ├── fake_flowschema.go
│   │   │   │   │   └── fake_prioritylevelconfiguration.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── flowcontrol_client.go
│   │   │   │   ├── flowschema.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   └── prioritylevelconfiguration.go
│   │   │   └── v1beta3
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_flowcontrol_client.go
│   │   │       │   ├── fake_flowschema.go
│   │   │       │   └── fake_prioritylevelconfiguration.go
│   │   │       ├── doc.go
│   │   │       ├── flowcontrol_client.go
│   │   │       ├── flowschema.go
│   │   │       ├── generated_expansion.go
│   │   │       └── prioritylevelconfiguration.go
│   │   ├── networking
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_ingress.go
│   │   │   │   │   ├── fake_ingressclass.go
│   │   │   │   │   ├── fake_ipaddress.go
│   │   │   │   │   ├── fake_networking_client.go
│   │   │   │   │   ├── fake_networkpolicy.go
│   │   │   │   │   └── fake_servicecidr.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── ingress.go
│   │   │   │   ├── ingressclass.go
│   │   │   │   ├── ipaddress.go
│   │   │   │   ├── networking_client.go
│   │   │   │   ├── networkpolicy.go
│   │   │   │   └── servicecidr.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_ingress.go
│   │   │       │   ├── fake_ingressclass.go
│   │   │       │   ├── fake_ipaddress.go
│   │   │       │   ├── fake_networking_client.go
│   │   │       │   └── fake_servicecidr.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── ingress.go
│   │   │       ├── ingressclass.go
│   │   │       ├── ipaddress.go
│   │   │       ├── networking_client.go
│   │   │       └── servicecidr.go
│   │   ├── node
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_node_client.go
│   │   │   │   │   └── fake_runtimeclass.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── node_client.go
│   │   │   │   └── runtimeclass.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_node_client.go
│   │   │   │   │   └── fake_runtimeclass.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── node_client.go
│   │   │   │   └── runtimeclass.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_node_client.go
│   │   │       │   └── fake_runtimeclass.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── node_client.go
│   │   │       └── runtimeclass.go
│   │   ├── policy
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_eviction.go
│   │   │   │   │   ├── fake_eviction_expansion.go
│   │   │   │   │   ├── fake_poddisruptionbudget.go
│   │   │   │   │   └── fake_policy_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── eviction.go
│   │   │   │   ├── eviction_expansion.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── poddisruptionbudget.go
│   │   │   │   └── policy_client.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_eviction.go
│   │   │       │   ├── fake_eviction_expansion.go
│   │   │       │   ├── fake_poddisruptionbudget.go
│   │   │       │   └── fake_policy_client.go
│   │   │       ├── doc.go
│   │   │       ├── eviction.go
│   │   │       ├── eviction_expansion.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── poddisruptionbudget.go
│   │   │       └── policy_client.go
│   │   ├── rbac
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_clusterrole.go
│   │   │   │   │   ├── fake_clusterrolebinding.go
│   │   │   │   │   ├── fake_rbac_client.go
│   │   │   │   │   ├── fake_role.go
│   │   │   │   │   └── fake_rolebinding.go
│   │   │   │   ├── clusterrole.go
│   │   │   │   ├── clusterrolebinding.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── rbac_client.go
│   │   │   │   ├── role.go
│   │   │   │   └── rolebinding.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_clusterrole.go
│   │   │   │   │   ├── fake_clusterrolebinding.go
│   │   │   │   │   ├── fake_rbac_client.go
│   │   │   │   │   ├── fake_role.go
│   │   │   │   │   └── fake_rolebinding.go
│   │   │   │   ├── clusterrole.go
│   │   │   │   ├── clusterrolebinding.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── rbac_client.go
│   │   │   │   ├── role.go
│   │   │   │   └── rolebinding.go
│   │   │   ├── v1beta1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_clusterrole.go
│   │   │   │   │   ├── fake_clusterrolebinding.go
│   │   │   │   │   ├── fake_rbac_client.go
│   │   │   │   │   ├── fake_role.go
│   │   │   │   │   └── fake_rolebinding.go
│   │   │   │   ├── clusterrole.go
│   │   │   │   ├── clusterrolebinding.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── rbac_client.go
│   │   │   │   ├── role.go
│   │   │   │   └── rolebinding.go
│   │   │   └── OWNERS
│   │   ├── resource
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_deviceclass.go
│   │   │   │   │   ├── fake_resource_client.go
│   │   │   │   │   ├── fake_resourceclaim.go
│   │   │   │   │   ├── fake_resourceclaimtemplate.go
│   │   │   │   │   └── fake_resourceslice.go
│   │   │   │   ├── deviceclass.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── resource_client.go
│   │   │   │   ├── resourceclaim.go
│   │   │   │   ├── resourceclaimtemplate.go
│   │   │   │   └── resourceslice.go
│   │   │   ├── v1alpha3
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_devicetaintrule.go
│   │   │   │   │   ├── fake_resource_client.go
│   │   │   │   │   └── fake_resourcepoolstatusrequest.go
│   │   │   │   ├── devicetaintrule.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── resource_client.go
│   │   │   │   └── resourcepoolstatusrequest.go
│   │   │   ├── v1beta1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_deviceclass.go
│   │   │   │   │   ├── fake_resource_client.go
│   │   │   │   │   ├── fake_resourceclaim.go
│   │   │   │   │   ├── fake_resourceclaimtemplate.go
│   │   │   │   │   └── fake_resourceslice.go
│   │   │   │   ├── deviceclass.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── resource_client.go
│   │   │   │   ├── resourceclaim.go
│   │   │   │   ├── resourceclaimtemplate.go
│   │   │   │   └── resourceslice.go
│   │   │   └── v1beta2
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_deviceclass.go
│   │   │       │   ├── fake_devicetaintrule.go
│   │   │       │   ├── fake_resource_client.go
│   │   │       │   ├── fake_resourceclaim.go
│   │   │       │   ├── fake_resourceclaimtemplate.go
│   │   │       │   └── fake_resourceslice.go
│   │   │       ├── deviceclass.go
│   │   │       ├── devicetaintrule.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── resource_client.go
│   │   │       ├── resourceclaim.go
│   │   │       ├── resourceclaimtemplate.go
│   │   │       └── resourceslice.go
│   │   ├── scheduling
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_priorityclass.go
│   │   │   │   │   └── fake_scheduling_client.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── priorityclass.go
│   │   │   │   └── scheduling_client.go
│   │   │   ├── v1alpha3
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_podgroup.go
│   │   │   │   │   ├── fake_scheduling_client.go
│   │   │   │   │   └── fake_workload.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── podgroup.go
│   │   │   │   ├── scheduling_client.go
│   │   │   │   └── workload.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_priorityclass.go
│   │   │       │   └── fake_scheduling_client.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── priorityclass.go
│   │   │       └── scheduling_client.go
│   │   ├── storage
│   │   │   ├── v1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_csidriver.go
│   │   │   │   │   ├── fake_csinode.go
│   │   │   │   │   ├── fake_csistoragecapacity.go
│   │   │   │   │   ├── fake_storage_client.go
│   │   │   │   │   ├── fake_storageclass.go
│   │   │   │   │   ├── fake_volumeattachment.go
│   │   │   │   │   └── fake_volumeattributesclass.go
│   │   │   │   ├── csidriver.go
│   │   │   │   ├── csinode.go
│   │   │   │   ├── csistoragecapacity.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── storage_client.go
│   │   │   │   ├── storageclass.go
│   │   │   │   ├── volumeattachment.go
│   │   │   │   └── volumeattributesclass.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── fake
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fake_csistoragecapacity.go
│   │   │   │   │   ├── fake_storage_client.go
│   │   │   │   │   ├── fake_volumeattachment.go
│   │   │   │   │   └── fake_volumeattributesclass.go
│   │   │   │   ├── csistoragecapacity.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated_expansion.go
│   │   │   │   ├── storage_client.go
│   │   │   │   ├── volumeattachment.go
│   │   │   │   └── volumeattributesclass.go
│   │   │   └── v1beta1
│   │   │       ├── fake
│   │   │       │   ├── doc.go
│   │   │       │   ├── fake_csidriver.go
│   │   │       │   ├── fake_csinode.go
│   │   │       │   ├── fake_csistoragecapacity.go
│   │   │       │   ├── fake_storage_client.go
│   │   │       │   ├── fake_storageclass.go
│   │   │       │   ├── fake_volumeattachment.go
│   │   │       │   └── fake_volumeattributesclass.go
│   │   │       ├── csidriver.go
│   │   │       ├── csinode.go
│   │   │       ├── csistoragecapacity.go
│   │   │       ├── doc.go
│   │   │       ├── generated_expansion.go
│   │   │       ├── storage_client.go
│   │   │       ├── storageclass.go
│   │   │       ├── volumeattachment.go
│   │   │       └── volumeattributesclass.go
│   │   └── storagemigration
│   │       └── v1beta1
│   │           ├── fake
│   │           │   ├── doc.go
│   │           │   ├── fake_storagemigration_client.go
│   │           │   └── fake_storageversionmigration.go
│   │           ├── doc.go
│   │           ├── generated_expansion.go
│   │           ├── storagemigration_client.go
│   │           └── storageversionmigration.go
│   ├── clientset.go
│   ├── doc.go
│   └── import.go
├── kubernetes_test
├── listers
│   ├── admissionregistration
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── mutatingwebhookconfiguration.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   ├── validatingadmissionpolicybinding.go
│   │   │   └── validatingwebhookconfiguration.go
│   │   ├── v1alpha1
│   │   │   ├── expansion_generated.go
│   │   │   ├── mutatingadmissionpolicy.go
│   │   │   ├── mutatingadmissionpolicybinding.go
│   │   │   ├── validatingadmissionpolicy.go
│   │   │   └── validatingadmissionpolicybinding.go
│   │   └── v1beta1
│   │       ├── expansion_generated.go
│   │       ├── mutatingadmissionpolicy.go
│   │       ├── mutatingadmissionpolicybinding.go
│   │       ├── mutatingwebhookconfiguration.go
│   │       ├── validatingadmissionpolicy.go
│   │       ├── validatingadmissionpolicybinding.go
│   │       └── validatingwebhookconfiguration.go
│   ├── apiserverinternal
│   │   └── v1alpha1
│   │       ├── expansion_generated.go
│   │       └── storageversion.go
│   ├── apps
│   │   ├── v1
│   │   │   ├── controllerrevision.go
│   │   │   ├── daemonset.go
│   │   │   ├── daemonset_expansion.go
│   │   │   ├── deployment.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── replicaset.go
│   │   │   ├── replicaset_expansion.go
│   │   │   ├── statefulset.go
│   │   │   └── statefulset_expansion.go
│   │   ├── v1beta1
│   │   │   ├── controllerrevision.go
│   │   │   ├── deployment.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── statefulset.go
│   │   │   └── statefulset_expansion.go
│   │   └── v1beta2
│   │       ├── controllerrevision.go
│   │       ├── daemonset.go
│   │       ├── daemonset_expansion.go
│   │       ├── deployment.go
│   │       ├── expansion_generated.go
│   │       ├── replicaset.go
│   │       ├── replicaset_expansion.go
│   │       ├── statefulset.go
│   │       └── statefulset_expansion.go
│   ├── autoscaling
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   └── horizontalpodautoscaler.go
│   │   └── v2
│   │       ├── expansion_generated.go
│   │       └── horizontalpodautoscaler.go
│   ├── batch
│   │   ├── v1
│   │   │   ├── cronjob.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── job.go
│   │   │   └── job_expansion.go
│   │   └── v1beta1
│   │       ├── cronjob.go
│   │       └── expansion_generated.go
│   ├── certificates
│   │   ├── v1
│   │   │   ├── certificatesigningrequest.go
│   │   │   └── expansion_generated.go
│   │   ├── v1alpha1
│   │   │   ├── clustertrustbundle.go
│   │   │   └── expansion_generated.go
│   │   └── v1beta1
│   │       ├── certificatesigningrequest.go
│   │       ├── clustertrustbundle.go
│   │       ├── expansion_generated.go
│   │       └── podcertificaterequest.go
│   ├── coordination
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   └── lease.go
│   │   ├── v1alpha2
│   │   │   ├── expansion_generated.go
│   │   │   └── leasecandidate.go
│   │   └── v1beta1
│   │       ├── expansion_generated.go
│   │       ├── lease.go
│   │       └── leasecandidate.go
│   ├── core
│   │   └── v1
│   │       ├── componentstatus.go
│   │       ├── configmap.go
│   │       ├── endpoints.go
│   │       ├── event.go
│   │       ├── expansion_generated.go
│   │       ├── limitrange.go
│   │       ├── namespace.go
│   │       ├── node.go
│   │       ├── persistentvolume.go
│   │       ├── persistentvolumeclaim.go
│   │       ├── pod.go
│   │       ├── podtemplate.go
│   │       ├── replicationcontroller.go
│   │       ├── replicationcontroller_expansion.go
│   │       ├── resourcequota.go
│   │       ├── secret.go
│   │       ├── service.go
│   │       └── serviceaccount.go
│   ├── discovery
│   │   ├── v1
│   │   │   ├── endpointslice.go
│   │   │   └── expansion_generated.go
│   │   └── v1beta1
│   │       ├── endpointslice.go
│   │       └── expansion_generated.go
│   ├── events
│   │   ├── v1
│   │   │   ├── event.go
│   │   │   └── expansion_generated.go
│   │   └── v1beta1
│   │       ├── event.go
│   │       └── expansion_generated.go
│   ├── extensions
│   │   └── v1beta1
│   │       ├── daemonset.go
│   │       ├── daemonset_expansion.go
│   │       ├── deployment.go
│   │       ├── expansion_generated.go
│   │       ├── ingress.go
│   │       ├── networkpolicy.go
│   │       ├── replicaset.go
│   │       └── replicaset_expansion.go
│   ├── flowcontrol
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   ├── flowschema.go
│   │   │   └── prioritylevelconfiguration.go
│   │   ├── v1beta1
│   │   │   ├── expansion_generated.go
│   │   │   ├── flowschema.go
│   │   │   └── prioritylevelconfiguration.go
│   │   ├── v1beta2
│   │   │   ├── expansion_generated.go
│   │   │   ├── flowschema.go
│   │   │   └── prioritylevelconfiguration.go
│   │   └── v1beta3
│   │       ├── expansion_generated.go
│   │       ├── flowschema.go
│   │       └── prioritylevelconfiguration.go
│   ├── imagepolicy
│   │   └── v1alpha1
│   │       ├── expansion_generated.go
│   │       └── imagereview.go
│   ├── networking
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   ├── ingress.go
│   │   │   ├── ingressclass.go
│   │   │   ├── ipaddress.go
│   │   │   ├── networkpolicy.go
│   │   │   └── servicecidr.go
│   │   └── v1beta1
│   │       ├── expansion_generated.go
│   │       ├── ingress.go
│   │       ├── ingressclass.go
│   │       ├── ipaddress.go
│   │       └── servicecidr.go
│   ├── node
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   └── runtimeclass.go
│   │   ├── v1alpha1
│   │   │   ├── expansion_generated.go
│   │   │   └── runtimeclass.go
│   │   └── v1beta1
│   │       ├── expansion_generated.go
│   │       └── runtimeclass.go
│   ├── policy
│   │   ├── v1
│   │   │   ├── eviction.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── poddisruptionbudget.go
│   │   │   └── poddisruptionbudget_expansion.go
│   │   └── v1beta1
│   │       ├── eviction.go
│   │       ├── expansion_generated.go
│   │       ├── poddisruptionbudget.go
│   │       └── poddisruptionbudget_expansion.go
│   ├── rbac
│   │   ├── v1
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── role.go
│   │   │   └── rolebinding.go
│   │   ├── v1alpha1
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── role.go
│   │   │   └── rolebinding.go
│   │   ├── v1beta1
│   │   │   ├── clusterrole.go
│   │   │   ├── clusterrolebinding.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── role.go
│   │   │   └── rolebinding.go
│   │   └── OWNERS
│   ├── resource
│   │   ├── v1
│   │   │   ├── deviceclass.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   └── resourceslice.go
│   │   ├── v1alpha3
│   │   │   ├── devicetaintrule.go
│   │   │   ├── expansion_generated.go
│   │   │   └── resourcepoolstatusrequest.go
│   │   ├── v1beta1
│   │   │   ├── deviceclass.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── resourceclaim.go
│   │   │   ├── resourceclaimtemplate.go
│   │   │   └── resourceslice.go
│   │   └── v1beta2
│   │       ├── deviceclass.go
│   │       ├── devicetaintrule.go
│   │       ├── expansion_generated.go
│   │       ├── resourceclaim.go
│   │       ├── resourceclaimtemplate.go
│   │       └── resourceslice.go
│   ├── scheduling
│   │   ├── v1
│   │   │   ├── expansion_generated.go
│   │   │   └── priorityclass.go
│   │   ├── v1alpha3
│   │   │   ├── expansion_generated.go
│   │   │   ├── podgroup.go
│   │   │   └── workload.go
│   │   └── v1beta1
│   │       ├── expansion_generated.go
│   │       └── priorityclass.go
│   ├── storage
│   │   ├── v1
│   │   │   ├── csidriver.go
│   │   │   ├── csinode.go
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── storageclass.go
│   │   │   ├── volumeattachment.go
│   │   │   └── volumeattributesclass.go
│   │   ├── v1alpha1
│   │   │   ├── csistoragecapacity.go
│   │   │   ├── expansion_generated.go
│   │   │   ├── volumeattachment.go
│   │   │   └── volumeattributesclass.go
│   │   └── v1beta1
│   │       ├── csidriver.go
│   │       ├── csinode.go
│   │       ├── csistoragecapacity.go
│   │       ├── expansion_generated.go
│   │       ├── storageclass.go
│   │       ├── volumeattachment.go
│   │       └── volumeattributesclass.go
│   ├── storagemigration
│   │   └── v1beta1
│   │       ├── expansion_generated.go
│   │       └── storageversionmigration.go
│   ├── doc.go
│   └── generic_helpers.go
├── metadata
│   ├── fake
│   │   └── simple.go
│   ├── metadatainformer
│   │   ├── informer.go
│   │   └── interface.go
│   ├── metadatalister
│   │   ├── interface.go
│   │   ├── lister.go
│   │   └── shim.go
│   ├── interface.go
│   └── metadata.go
├── openapi
│   ├── cached
│   │   ├── client.go
│   │   └── groupversion.go
│   ├── openapitest
│   │   ├── fakeclient.go
│   │   └── fileclient.go
│   ├── OWNERS
│   ├── client.go
│   ├── groupversion.go
│   └── typeconverter.go
├── openapi3
│   └── root.go
├── pkg
│   ├── apis
│   │   └── clientauthentication
│   │       ├── install
│   │       │   └── install.go
│   │       ├── v1
│   │       │   ├── doc.go
│   │       │   ├── register.go
│   │       │   ├── types.go
│   │       │   ├── zz_generated.conversion.go
│   │       │   ├── zz_generated.deepcopy.go
│   │       │   ├── zz_generated.defaults.go
│   │       │   └── zz_generated.model_name.go
│   │       ├── v1beta1
│   │       │   ├── doc.go
│   │       │   ├── register.go
│   │       │   ├── types.go
│   │       │   ├── zz_generated.conversion.go
│   │       │   ├── zz_generated.deepcopy.go
│   │       │   ├── zz_generated.defaults.go
│   │       │   └── zz_generated.model_name.go
│   │       ├── OWNERS
│   │       ├── doc.go
│   │       ├── register.go
│   │       ├── types.go
│   │       └── zz_generated.deepcopy.go
│   └── version
│       ├── .gitattributes
│       ├── base.go
│       ├── doc.go
│       └── version.go
├── plugin
│   └── pkg
│       └── client
│           └── auth
│               ├── azure
│               │   └── azure_stub.go
│               ├── exec
│               │   ├── exec.go
│               │   └── metrics.go
│               ├── gcp
│               │   └── gcp_stub.go
│               ├── oidc
│               │   └── oidc.go
│               ├── OWNERS
│               ├── plugins.go
│               └── plugins_providers.go
├── rest
│   ├── fake
│   │   └── fake.go
│   ├── watch
│   │   ├── decoder.go
│   │   └── encoder.go
│   ├── OWNERS
│   ├── client.go
│   ├── config.go
│   ├── exec.go
│   ├── plugin.go
│   ├── request.go
│   ├── transport.go
│   ├── url_utils.go
│   ├── urlbackoff.go
│   ├── warnings.go
│   ├── with_retry.go
│   └── zz_generated.deepcopy.go
├── restmapper
│   ├── category_expansion.go
│   ├── discovery.go
│   └── shortcut.go
├── scale
│   ├── fake
│   │   └── client.go
│   ├── scheme
│   │   ├── appsint
│   │   │   ├── doc.go
│   │   │   └── register.go
│   │   ├── appsv1beta1
│   │   │   ├── conversion.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   └── zz_generated.conversion.go
│   │   ├── appsv1beta2
│   │   │   ├── conversion.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   └── zz_generated.conversion.go
│   │   ├── autoscalingv1
│   │   │   ├── conversion.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   └── zz_generated.conversion.go
│   │   ├── extensionsint
│   │   │   ├── doc.go
│   │   │   └── register.go
│   │   ├── extensionsv1beta1
│   │   │   ├── conversion.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   └── zz_generated.conversion.go
│   │   ├── doc.go
│   │   ├── register.go
│   │   ├── types.go
│   │   └── zz_generated.deepcopy.go
│   ├── client.go
│   ├── doc.go
│   ├── interfaces.go
│   └── util.go
├── testing
│   ├── internal
│   ├── actions.go
│   ├── doc.go
│   ├── fake.go
│   ├── fixture.go
│   └── interface.go
├── tools
│   ├── auth
│   │   ├── exec
│   │   │   └── exec.go
│   │   ├── OWNERS
│   │   └── clientauth.go
│   ├── cache
│   │   ├── synctrack
│   │   │   ├── lazy.go
│   │   │   └── synctrack.go
│   │   ├── testing
│   │   │   └── fake_controller_source.go
│   │   ├── OWNERS
│   │   ├── controller.go
│   │   ├── delta_fifo.go
│   │   ├── doc.go
│   │   ├── event_handler_name.go
│   │   ├── expiration_cache.go
│   │   ├── expiration_cache_fakes.go
│   │   ├── fake_custom_store.go
│   │   ├── fifo.go
│   │   ├── heap.go
│   │   ├── identity.go
│   │   ├── index.go
│   │   ├── informer_metrics.go
│   │   ├── listers.go
│   │   ├── listwatch.go
│   │   ├── mutation_cache.go
│   │   ├── mutation_detector.go
│   │   ├── object-names.go
│   │   ├── reflector.go
│   │   ├── reflector_data_consistency_detector.go
│   │   ├── reflector_metrics.go
│   │   ├── retry_with_deadline.go
│   │   ├── shared_informer.go
│   │   ├── store.go
│   │   ├── the_real_fifo.go
│   │   ├── thread_safe_store.go
│   │   └── undelta_store.go
│   ├── clientcmd
│   │   ├── api
│   │   │   ├── latest
│   │   │   │   └── latest.go
│   │   │   ├── v1
│   │   │   │   ├── conversion.go
│   │   │   │   ├── defaults.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   └── zz_generated.defaults.go
│   │   │   ├── doc.go
│   │   │   ├── helpers.go
│   │   │   ├── register.go
│   │   │   ├── types.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── auth_loaders.go
│   │   ├── client_config.go
│   │   ├── config.go
│   │   ├── doc.go
│   │   ├── flag.go
│   │   ├── helpers.go
│   │   ├── loader.go
│   │   ├── merge.go
│   │   ├── merged_client_builder.go
│   │   ├── overrides.go
│   │   └── validation.go
│   ├── events
│   │   ├── OWNERS
│   │   ├── doc.go
│   │   ├── event_broadcaster.go
│   │   ├── event_recorder.go
│   │   ├── fake.go
│   │   ├── helper.go
│   │   └── interfaces.go
│   ├── internal
│   │   └── events
│   │       └── interfaces.go
│   ├── leaderelection
│   │   ├── resourcelock
│   │   │   ├── interface.go
│   │   │   ├── leaselock.go
│   │   │   └── multilock.go
│   │   ├── OWNERS
│   │   ├── healthzadaptor.go
│   │   ├── leaderelection.go
│   │   ├── leasecandidate.go
│   │   └── metrics.go
│   ├── metrics
│   │   ├── OWNERS
│   │   └── metrics.go
│   ├── pager
│   │   └── pager.go
│   ├── portforward
│   │   ├── OWNERS
│   │   ├── doc.go
│   │   ├── fallback_dialer.go
│   │   ├── portforward.go
│   │   ├── tunneling_connection.go
│   │   └── tunneling_dialer.go
│   ├── record
│   │   ├── util
│   │   │   └── util.go
│   │   ├── OWNERS
│   │   ├── doc.go
│   │   ├── event.go
│   │   ├── events_cache.go
│   │   └── fake.go
│   ├── reference
│   │   └── ref.go
│   ├── remotecommand
│   │   ├── OWNERS
│   │   ├── doc.go
│   │   ├── errorstream.go
│   │   ├── fallback.go
│   │   ├── reader.go
│   │   ├── remotecommand.go
│   │   ├── resize.go
│   │   ├── spdy.go
│   │   ├── v1.go
│   │   ├── v2.go
│   │   ├── v3.go
│   │   ├── v4.go
│   │   ├── v5.go
│   │   └── websocket.go
│   └── watch
│       ├── informerwatcher.go
│       ├── retrywatcher.go
│       └── until.go
├── transport
│   ├── spdy
│   │   └── spdy.go
│   ├── websocket
│   │   └── roundtripper.go
│   ├── OWNERS
│   ├── ca_rotation.go
│   ├── cache.go
│   ├── cache_go118.go
│   ├── cert_rotation.go
│   ├── config.go
│   ├── round_trippers.go
│   ├── token_source.go
│   └── transport.go
├── util
│   ├── apply
│   │   └── apply.go
│   ├── cert
│   │   ├── OWNERS
│   │   ├── cert.go
│   │   ├── csr.go
│   │   ├── io.go
│   │   ├── pem.go
│   │   └── server_inspection.go
│   ├── certificate
│   │   ├── csr
│   │   │   └── csr.go
│   │   ├── OWNERS
│   │   ├── certificate_manager.go
│   │   └── certificate_store.go
│   ├── connrotation
│   │   └── connrotation.go
│   ├── consistencydetector
│   │   └── data_consistency_detector.go
│   ├── csaupgrade
│   │   ├── OWNERS
│   │   ├── options.go
│   │   └── upgrade.go
│   ├── exec
│   │   └── exec.go
│   ├── flowcontrol
│   │   ├── backoff.go
│   │   └── throttle.go
│   ├── homedir
│   │   └── homedir.go
│   ├── jsonpath
│   │   ├── doc.go
│   │   ├── jsonpath.go
│   │   ├── node.go
│   │   └── parser.go
│   ├── keyutil
│   │   ├── OWNERS
│   │   └── key.go
│   ├── retry
│   │   └── util.go
│   ├── testing
│   │   ├── fake_handler.go
│   │   ├── fake_openapi_handler.go
│   │   ├── remove_file.go
│   │   └── tmpdir.go
│   ├── watchlist
│   │   └── watch_list.go
│   └── workqueue
│       ├── default_rate_limiters.go
│       ├── delaying_queue.go
│       ├── doc.go
│       ├── metrics.go
│       ├── parallelizer.go
│       ├── queue.go
│       └── rate_limiting_queue.go
├── ARCHITECTURE.md
├── CONTRIBUTING.md
├── INSTALL.md
├── LICENSE
├── OWNERS
├── README.md
├── SECURITY_CONTACTS
├── code-of-conduct.md
├── doc.go
├── go.mod
└── go.sum
```

## Source Stream Aggregation

// === FILE: references!/kubernetes/staging/src/k8s.io/client-go/tools/cache/delta_fifo.go ===
```go
/*
Copyright 2014 The Kubernetes Authors.

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

package cache

import (
	"errors"
	"fmt"
	"sync"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/klog/v2"
	utiltrace "k8s.io/utils/trace"
)

// DeltaFIFOOptions is the configuration parameters for DeltaFIFO. All are
// optional.
type DeltaFIFOOptions struct {
	// If set, log output will go to this logger instead of klog.Background().
	// The name of the fifo gets added automatically.
	Logger *klog.Logger

	// Name can be used to override the default "DeltaFIFO" name for the new instance.
	Name string

	// KeyFunction is used to figure out what key an object should have. (It's
	// exposed in the returned DeltaFIFO's KeyOf() method, with additional
	// handling around deleted objects and queue state).
	// Optional, the default is MetaNamespaceKeyFunc.
	KeyFunction KeyFunc

	// KnownObjects is expected to return a list of keys that the consumer of
	// this queue "knows about". It is used to decide which items are missing
	// when Replace() is called; 'Deleted' deltas are produced for the missing items.
	// KnownObjects may be nil if you can tolerate missing deletions on Replace().
	KnownObjects KeyListerGetter

	// EmitDeltaTypeReplaced indicates that the queue consumer
	// understands the Replaced DeltaType. Before the `Replaced` event type was
	// added, calls to Replace() were handled the same as Sync(). For
	// backwards-compatibility purposes, this is false by default.
	// When true, `Replaced` events will be sent for items passed to a Replace() call.
	// When false, `Sync` events will be sent instead.
	EmitDeltaTypeReplaced bool

	// If set, will be called for objects before enqueueing them. Please
	// see the comment on TransformFunc for details.
	Transformer TransformFunc
}

// DeltaFIFO is like FIFO, but differs in two ways.  One is that the
// accumulator associated with a given object's key is not that object
// but rather a Deltas, which is a slice of Delta values for that
// object.  Applying an object to a Deltas means to append a Delta
// except when the potentially appended Delta is a Deleted and the
// Deltas already ends with a Deleted.  In that case the Deltas does
// not grow, although the terminal Deleted will be replaced by the new
// Deleted if the older Deleted's object is a
// DeletedFinalStateUnknown.
//
// The other difference is that DeltaFIFO has two additional ways that
// an object can be applied to an accumulator: Replaced and Sync.
// If EmitDeltaTypeReplaced is not set to true, Sync will be used in
// replace events for backwards compatibility.  Sync is used for periodic
// resync events.
//
// DeltaFIFO is a producer-consumer queue, where a Reflector is
// intended to be the producer, and the consumer is whatever calls
// the Pop() method.
//
// DeltaFIFO solves this use case:
//   - You want to process every object change (delta) at most once.
//   - When you process an object, you want to see everything
//     that's happened to it since you last processed it.
//   - You want to process the deletion of some of the objects.
//   - You might want to periodically reprocess objects.
//
// DeltaFIFO's Pop(), Get(), and GetByKey() methods return
// interface{} to satisfy the Store/Queue interfaces, but they
// will always return an object of type Deltas. List() returns
// the newest object from each accumulator in the FIFO.
//
// A DeltaFIFO's knownObjects KeyListerGetter provides the abilities
// to list Store keys and to get objects by Store key.  The objects in
// question are called "known objects" and this set of objects
// modifies the behavior of the Delete, Replace, and Resync methods
// (each in a different way).
//
// A note on threading: If you call Pop() in parallel from multiple
// threads, you could end up with multiple threads processing slightly
// different versions of the same object.
type DeltaFIFO struct {
	// logger is a per-instance logger. This gets chosen when constructing
	// the instance, with klog.Background() as default.
	logger klog.Logger

	// name is the name of the fifo. It is included in the logger.
	name string

	// lock/cond protects access to 'items' and 'queue'.
	lock sync.RWMutex
	cond sync.Cond

	// `items` maps a key to a Deltas.
	// Each such Deltas has at least one Delta.
	items map[string]Deltas

	// `queue` maintains FIFO order of keys for consumption in Pop().
	// There are no duplicates in `queue`.
	// A key is in `queue` if and only if it is in `items`.
	queue []string

	// synced is initially an open channel. It gets closed (once!) by checkSynced_locked
	// as soon as the initial sync is considered complete.
	synced       chan struct{}
	syncedClosed bool

	// populated is true if the first batch of items inserted by Replace() has been populated
	// or Delete/Add/Update/AddIfNotPresent was called first.
	populated bool
	// initialPopulationCount is the number of items inserted by the first call of Replace()
	initialPopulationCount int

	// keyFunc is used to make the key used for queued item
	// insertion and retrieval, and should be deterministic.
	keyFunc KeyFunc

	// knownObjects list keys that are "known" --- affecting Delete(),
	// Replace(), and Resync()
	knownObjects KeyListerGetter

	// Used to indicate a queue is closed so a control loop can exit when a queue is empty.
	// Currently, not used to gate any of CRUD operations.
	closed bool

	// emitDeltaTypeReplaced is whether to emit the Replaced or Sync
	// DeltaType when Replace() is called (to preserve backwards compat).
	emitDeltaTypeReplaced bool

	// Called with every object if non-nil.
	transformer TransformFunc
}

// TransformFunc allows for transforming an object before it will be processed.
//
// The most common usage pattern is to clean-up some parts of the object to
// reduce component memory usage if a given component doesn't care about them.
//
// New in v1.27: TransformFunc sees the object before any other actor, and it
// is now safe to mutate the object in place instead of making a copy.
//
// It's recommended for the TransformFunc to be idempotent.
// It MUST be idempotent if objects already present in the cache are passed to
// the Replace() to avoid re-mutating them. Default informers do not pass
// existing objects to Replace though.
//
// Note that TransformFunc is called while inserting objects into the
// notification queue and is therefore extremely performance sensitive; please
// do not do anything that will take a long time.
type TransformFunc func(interface{}) (interface{}, error)

// DeltaType is the type of a change (addition, deletion, etc)
type DeltaType string

// Change type definition
const (
	Added   DeltaType = "Added"
	Updated DeltaType = "Updated"
	Deleted DeltaType = "Deleted"
	// Replaced is emitted when we encountered watch errors and had to do a
	// relist, or on initial listing of objects. We don't know if the replaced
	// object has changed.
	//
	// NOTE: Previous versions of DeltaFIFO would use Sync for Replace events
	// as well. Hence, Replaced is only emitted when the option
	// EmitDeltaTypeReplaced is true.
	Replaced DeltaType = "Replaced"
	// ReplacedAll is emitted when we encountered watch errors and had to do
	// a relist, or on initial listing of objects. This is the same reason as
	// Replaced but will be emitted instead when the FIFO supports atomic
	// replacement. This event will return the full list of replaced items
	// instead of a single object.
	ReplacedAll DeltaType = "ReplacedAll"
	// Sync is for synthetic events during a periodic resync.
	Sync DeltaType = "Sync"
	// SyncAll indicates all known objects should be reprocessed.
	// This event contains an object of type SyncAllInfo.
	SyncAll DeltaType = "SyncAll"
	// Bookmark is emitted on Bookmark calls and Replace calls to pass resource
	// version information to the consumer.
	Bookmark DeltaType = "Bookmark"
)

// Delta is a member of Deltas (a list of Delta objects) which
// in its turn is the type stored by a DeltaFIFO. It tells you what
// change happened, and the object's state after* that change.
//
// [*] Unless the change is a deletion, and then you'll get the final
// state of the object before it was deleted.
type Delta struct {
	Type   DeltaType
	Object interface{}
}

// Deltas is a list of one or more 'Delta's to an individual object.
// The oldest delta is at index 0, the newest delta is the last one.
type Deltas []Delta

// NewDeltaFIFO returns a Queue which can be used to process changes to items.
//
// keyFunc is used to figure out what key an object should have. (It is
// exposed in the returned DeltaFIFO's KeyOf() method, with additional handling
// around deleted objects and queue state).
//
// 'knownObjects' may be supplied to modify the behavior of Delete,
// Replace, and Resync.  It may be nil if you do not need those
// modifications.
//
// TODO: consider merging keyLister with this object, tracking a list of
// "known" keys when Pop() is called. Have to think about how that
// affects error retrying.
//
//	NOTE: It is possible to misuse this and cause a race when using an
//	external known object source.
//	Whether there is a potential race depends on how the consumer
//	modifies knownObjects. In Pop(), process function is called under
//	lock, so it is safe to update data structures in it that need to be
//	in sync with the queue (e.g. knownObjects).
//
//	Example:
//	In case of sharedIndexInformer being a consumer
//	(https://github.com/kubernetes/kubernetes/blob/0cdd940f/staging/src/k8s.io/client-go/tools/cache/shared_informer.go#L192),
//	there is no race as knownObjects (s.indexer) is modified safely
//	under DeltaFIFO's lock. The only exceptions are GetStore() and
//	GetIndexer() methods, which expose ways to modify the underlying
//	storage. Currently these two methods are used for creating Lister
//	and internal tests.
//
// Also see the comment on DeltaFIFO.
//
// Warning: This constructs a DeltaFIFO that does not differentiate between
// events caused by a call to Replace (e.g., from a relist, which may
// contain object updates), and synthetic events caused by a periodic resync
// (which just emit the existing object). See https://issue.k8s.io/86015 for details.
//
// Use `NewDeltaFIFOWithOptions(DeltaFIFOOptions{..., EmitDeltaTypeReplaced: true})`
// instead to receive a `Replaced` event depending on the type.
//
// Deprecated: Equivalent to NewDeltaFIFOWithOptions(DeltaFIFOOptions{KeyFunction: keyFunc, KnownObjects: knownObjects})
func NewDeltaFIFO(keyFunc KeyFunc, knownObjects KeyListerGetter) *DeltaFIFO {
	return NewDeltaFIFOWithOptions(DeltaFIFOOptions{
		KeyFunction:  keyFunc,
		KnownObjects: knownObjects,
	})
}

// NewDeltaFIFOWithOptions returns a Queue which can be used to process changes to
// items. See also the comment on DeltaFIFO.
func NewDeltaFIFOWithOptions(opts DeltaFIFOOptions) *DeltaFIFO {
	if opts.KeyFunction == nil {
		opts.KeyFunction = MetaNamespaceKeyFunc
	}

	f := &DeltaFIFO{
		logger:       klog.Background(),
		name:         "DeltaFIFO",
		synced:       make(chan struct{}),
		items:        map[string]Deltas{},
		queue:        []string{},
		keyFunc:      opts.KeyFunction,
		knownObjects: opts.KnownObjects,

		emitDeltaTypeReplaced: opts.EmitDeltaTypeReplaced,
		transformer:           opts.Transformer,
	}
	if opts.Logger != nil {
		f.logger = *opts.Logger
	}
	if name := opts.Name; name != "" {
		f.name = name
	}
	f.logger = klog.LoggerWithName(f.logger, f.name)
	f.cond.L = &f.lock
	return f
}

var (
	_ = Queue(&DeltaFIFO{})             // DeltaFIFO is a Queue
	_ = TransformingStore(&DeltaFIFO{}) // DeltaFIFO implements TransformingStore to allow memory optimizations
	_ = DoneChecker(&DeltaFIFO{})       // DeltaFIFO implements DoneChecker.
)

var (
	// ErrZeroLengthDeltasObject is returned in a KeyError if a Deltas
	// object with zero length is encountered (should be impossible,
	// but included for completeness).
	ErrZeroLengthDeltasObject = errors.New("0 length Deltas object; can't get key")
)

// Close the queue.
func (f *DeltaFIFO) Close() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.closed = true
	f.cond.Broadcast()
}

// KeyOf exposes f's keyFunc, but also detects the key of a Deltas object or
// DeletedFinalStateUnknown objects.
func (f *DeltaFIFO) KeyOf(obj interface{}) (string, error) {
	if d, ok := obj.(Deltas); ok {
		if len(d) == 0 {
			return "", KeyError{obj, ErrZeroLengthDeltasObject}
		}
		obj = d.Newest().Object
	}
	if d, ok := obj.(DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return f.keyFunc(obj)
}

// Transformer implements the TransformingStore interface.
func (f *DeltaFIFO) Transformer() TransformFunc {
	return f.transformer
}

// HasSynced returns true if an Add/Update/Delete/AddIfNotPresent are called first,
// or the first batch of items inserted by Replace() has been popped.
func (f *DeltaFIFO) HasSynced() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.hasSynced_locked()
}

// HasSyncedChecker is done if an Add/Update/Delete/AddIfNotPresent are called first,
// or the first batch of items inserted by Replace() has been popped.
func (f *DeltaFIFO) HasSyncedChecker() DoneChecker {
	return f
}

// Name implements [DoneChecker.Name]
func (f *DeltaFIFO) Name() string {
	return f.name
}

// Done implements [DoneChecker.Done]
func (f *DeltaFIFO) Done() <-chan struct{} {
	return f.synced
}

// hasSynced_locked returns the result of a prior checkSynced_locked call.
func (f *DeltaFIFO) hasSynced_locked() bool {
	return f.syncedClosed
}

// checkSynced_locked checks whether the initial is completed.
// It must be called whenever populated or initialPopulationCount change.
func (f *DeltaFIFO) checkSynced_locked() {
	synced := f.populated && f.initialPopulationCount == 0
	if synced && !f.syncedClosed {
		// Initial sync is complete.
		f.syncedClosed = true
		close(f.synced)
	}
}

// Add inserts an item, and puts it in the queue. The item is only enqueued
// if it doesn't already exist in the set.
func (f *DeltaFIFO) Add(obj interface{}) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true
	f.checkSynced_locked()
	return f.queueActionLocked(Added, obj)
}

// Update is just like Add, but makes an Updated Delta.
func (f *DeltaFIFO) Update(obj interface{}) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true
	f.checkSynced_locked()
	return f.queueActionLocked(Updated, obj)
}

// Delete is just like Add, but makes a Deleted Delta. If the given
// object does not already exist, it will be ignored. (It may have
// already been deleted by a Replace (re-list), for example.)  In this
// method `f.knownObjects`, if not nil, provides (via GetByKey)
// _additional_ objects that are considered to already exist.
func (f *DeltaFIFO) Delete(obj interface{}) error {
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true
	f.checkSynced_locked()
	if f.knownObjects == nil {
		if _, exists := f.items[id]; !exists {
			// Presumably, this was deleted when a relist happened.
			// Don't provide a second report of the same deletion.
			return nil
		}
	} else {
		// We only want to skip the "deletion" action if the object doesn't
		// exist in knownObjects and it doesn't have corresponding item in items.
		// Note that even if there is a "deletion" action in items, we can ignore it,
		// because it will be deduped automatically in "queueActionLocked"
		_, exists, err := f.knownObjects.GetByKey(id)
		_, itemsExist := f.items[id]
		if err == nil && !exists && !itemsExist {
			// Presumably, this was deleted when a relist happened.
			// Don't provide a second report of the same deletion.
			return nil
		}
	}

	// exist in items and/or KnownObjects
	return f.queueActionLocked(Deleted, obj)
}

// re-listing and watching can deliver the same update multiple times in any
// order. This will combine the most recent two deltas if they are the same.
func dedupDeltas(deltas Deltas) Deltas {
	n := len(deltas)
	if n < 2 {
		return deltas
	}
	a := &deltas[n-1]
	b := &deltas[n-2]
	if out := isDup(a, b); out != nil {
		deltas[n-2] = *out
		return deltas[:n-1]
	}
	return deltas
}

// If a & b represent the same event, returns the delta that ought to be kept.
// Otherwise, returns nil.
// TODO: is there anything other than deletions that need deduping?
func isDup(a, b *Delta) *Delta {
	if out := isDeletionDup(a, b); out != nil {
		return out
	}
	// TODO: Detect other duplicate situations? Are there any?
	return nil
}

// keep the one with the most information if both are deletions.
func isDeletionDup(a, b *Delta) *Delta {
	if b.Type != Deleted || a.Type != Deleted {
		return nil
	}
	// Do more sophisticated checks, or is this sufficient?
	if _, ok := b.Object.(DeletedFinalStateUnknown); ok {
		return a
	}
	return b
}

// queueActionLocked appends to the delta list for the object.
// Caller must lock first.
func (f *DeltaFIFO) queueActionLocked(actionType DeltaType, obj interface{}) error {
	return f.queueActionInternalLocked(actionType, actionType, obj)
}

// queueActionInternalLocked appends to the delta list for the object.
// The actionType is emitted and must honor emitDeltaTypeReplaced.
// The internalActionType is only used within this function and must
// ignore emitDeltaTypeReplaced.
// Caller must lock first.
func (f *DeltaFIFO) queueActionInternalLocked(actionType, internalActionType DeltaType, obj interface{}) error {
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}

	// Every object comes through this code path once, so this is a good
	// place to call the transform func.
	//
	// If obj is a DeletedFinalStateUnknown tombstone or the action is a Sync,
	// then the object have already gone through the transformer.
	//
	// If the objects already present in the cache are passed to Replace(),
	// the transformer must be idempotent to avoid re-mutating them,
	// or coordinate with all readers from the cache to avoid data races.
	// Default informers do not pass existing objects to Replace.
	if f.transformer != nil {
		_, isTombstone := obj.(DeletedFinalStateUnknown)
		if !isTombstone && internalActionType != Sync {
			var err error
			obj, err = f.transformer(obj)
			if err != nil {
				return err
			}
		}
	}

	oldDeltas := f.items[id]
	newDeltas := append(oldDeltas, Delta{actionType, obj})
	newDeltas = dedupDeltas(newDeltas)

	if len(newDeltas) > 0 {
		if _, exists := f.items[id]; !exists {
			f.queue = append(f.queue, id)
		}
		f.items[id] = newDeltas
		f.cond.Broadcast()
	} else {
		// This never happens, because dedupDeltas never returns an empty list
		// when given a non-empty list (as it is here).
		// If somehow it happens anyway, deal with it but complain.
		if oldDeltas == nil {
			utilruntime.HandleErrorWithLogger(f.logger, nil, "Impossible dedupDeltas, ignoring", "id", id, "oldDeltas", oldDeltas, "obj", obj)
			return nil
		}
		utilruntime.HandleErrorWithLogger(f.logger, nil, "Impossible dedupDeltas, breaking invariant by storing empty Deltas", "id", id, "oldDeltas", oldDeltas, "obj", obj)
		f.items[id] = newDeltas
		return fmt.Errorf("Impossible dedupDeltas for id=%q: oldDeltas=%#+v, obj=%#+v; broke DeltaFIFO invariant by storing empty Deltas", id, oldDeltas, obj)
	}
	return nil
}

// IsClosed checks if the queue is closed
func (f *DeltaFIFO) IsClosed() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.closed
}

// Pop blocks until the queue has some items, and then returns one.  If
// multiple items are ready, they are returned in the order in which they were
// added/updated. The item is removed from the queue (and the store) before it
// is returned, so if you don't successfully process it, you need to add it back
// with AddIfNotPresent().
// process function is called under lock, so it is safe to update data structures
// in it that need to be in sync with the queue (e.g. knownKeys).
// process should avoid expensive I/O operation so that other queue operations, i.e.
// Add() and Get(), won't be blocked for too long.
//
// Pop returns a 'Deltas', which has a complete list of all the things
// that happened to the object (deltas) while it was sitting in the queue.
func (f *DeltaFIFO) Pop(process PopProcessFunc) (interface{}, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	for {
		for len(f.queue) == 0 {
			// When the queue is empty, invocation of Pop() is blocked until new item is enqueued.
			// When Close() is called, the f.closed is set and the condition is broadcasted.
			// Which causes this loop to continue and return from the Pop().
			if f.closed {
				return nil, ErrFIFOClosed
			}

			f.cond.Wait()
		}
		isInInitialList := !f.hasSynced_locked()
		id := f.queue[0]
		f.queue = f.queue[1:]
		depth := len(f.queue)
		if f.initialPopulationCount > 0 {
			f.initialPopulationCount--
			f.checkSynced_locked()
		}
		item, ok := f.items[id]
		if !ok {
			// This should never happen
			utilruntime.HandleErrorWithLogger(f.logger, nil, "Inconceivable! Item was in f.queue but not f.items; ignoring", "id", id)
			continue
		}
		delete(f.items, id)
		// Only log traces if the queue depth is greater than 10 and it takes more than
		// 100 milliseconds to process one item from the queue.
		// Queue depth never goes high because processing an item is locking the queue,
		// and new items can't be added until processing finish.
		// https://github.com/kubernetes/kubernetes/issues/103789
		if depth > 10 {
			trace := utiltrace.New("DeltaFIFO Pop Process",
				utiltrace.Field{Key: "ID", Value: id},
				utiltrace.Field{Key: "Depth", Value: depth},
				utiltrace.Field{Key: "Reason", Value: "slow event handlers blocking the queue"})
			defer trace.LogIfLong(100 * time.Millisecond)
		}
		err := process(item, isInInitialList)
		// Don't need to copyDeltas here, because we're transferring
		// ownership to the caller.
		return item, err
	}
}

// Replace atomically does two things: (1) it adds the given objects
// using the Sync or Replace DeltaType and then (2) it does some deletions.
// In particular: for every pre-existing key K that is not the key of
// an object in `list` there is the effect of
// `Delete(DeletedFinalStateUnknown{K, O})` where O is the latest known
// object of K. The pre-existing keys are those in the union set of the keys in
// `f.items` and `f.knownObjects` (if not nil). The last known object for key K is
// the one present in the last delta in `f.items`. If there is no delta for K
// in `f.items`, it is the object in `f.knownObjects`
func (f *DeltaFIFO) Replace(list []interface{}, _ string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	keys := make(sets.Set[string], len(list))

	// keep backwards compat for old clients
	action := Sync
	if f.emitDeltaTypeReplaced {
		action = Replaced
	}

	// Add Sync/Replaced action for each new item.
	for _, item := range list {
		key, err := f.KeyOf(item)
		if err != nil {
			return KeyError{item, err}
		}
		keys.Insert(key)
		if err := f.queueActionInternalLocked(action, Replaced, item); err != nil {
			return fmt.Errorf("couldn't enqueue object: %v", err)
		}
	}

	// Do deletion detection against objects in the queue
	queuedDeletions := 0
	for k, oldItem := range f.items {
		if keys.Has(k) {
			continue
		}
		// Delete pre-existing items not in the new list.
		// This could happen if watch deletion event was missed while
		// disconnected from apiserver.
		var deletedObj interface{}
		if n := oldItem.Newest(); n != nil {
			deletedObj = n.Object

			// if the previous object is a DeletedFinalStateUnknown, we have to extract the actual Object
			if d, ok := deletedObj.(DeletedFinalStateUnknown); ok {
				deletedObj = d.Obj
			}
		}
		queuedDeletions++
		if err := f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}); err != nil {
			return err
		}
	}

	if f.knownObjects != nil {
		// Detect deletions for objects not present in the queue, but present in KnownObjects
		knownKeys := f.knownObjects.ListKeys()
		for _, k := range knownKeys {
			if keys.Has(k) {
				continue
			}
			if len(f.items[k]) > 0 {
				continue
			}

			deletedObj, exists, err := f.knownObjects.GetByKey(k)
			if err != nil {
				deletedObj = nil
				utilruntime.HandleErrorWithLogger(f.logger, err, "Unexpected error during lookup, placing DeleteFinalStateUnknown marker without object", "key", k)
			} else if !exists {
				deletedObj = nil
				f.logger.Info("Key does not exist in known objects store, placing DeleteFinalStateUnknown marker without object", "key", k)
			}
			queuedDeletions++
			if err := f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}); err != nil {
				return err
			}
		}
	}

	if !f.populated {
		f.populated = true
		f.initialPopulationCount = keys.Len() + queuedDeletions
		f.checkSynced_locked()
	}

	return nil
}

// Resync adds, with a Sync type of Delta, every object listed by
// `f.knownObjects` whose key is not already queued for processing.
// If `f.knownObjects` is `nil` then Resync does nothing.
func (f *DeltaFIFO) Resync() error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.knownObjects == nil {
		return nil
	}

	keys := f.knownObjects.ListKeys()
	for _, k := range keys {
		if err := f.syncKeyLocked(k); err != nil {
			return err
		}
	}
	return nil
}

func (f *DeltaFIFO) syncKeyLocked(key string) error {
	obj, exists, err := f.knownObjects.GetByKey(key)
	if err != nil {
		utilruntime.HandleErrorWithLogger(f.logger, err, "Unexpected error during lookup, unable to queue object for sync", "key", key)
		return nil
	} else if !exists {
		f.logger.Info("Key does not exist in known objects store, unable to queue object for sync", "key", key)
		return nil
	}

	// If we are doing Resync() and there is already an event queued for that object,
	// we ignore the Resync for it. This is to avoid the race, in which the resync
	// comes with the previous value of object (since queueing an event for the object
	// doesn't trigger changing the underlying store <knownObjects>.
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	if len(f.items[id]) > 0 {
		return nil
	}

	if err := f.queueActionLocked(Sync, obj); err != nil {
		return fmt.Errorf("couldn't queue object: %v", err)
	}
	return nil
}

// A KeyListerGetter is anything that knows how to list its keys and look up by key.
type KeyListerGetter interface {
	KeyLister
	KeyGetter
}

// A KeyLister is anything that knows how to list its keys.
type KeyLister interface {
	ListKeys() []string
}

// A KeyGetter is anything that knows how to get the value stored under a given key.
type KeyGetter interface {
	// GetByKey returns the value associated with the key, or sets exists=false.
	GetByKey(key string) (value interface{}, exists bool, err error)
}

// Oldest is a convenience function that returns the oldest delta, or
// nil if there are no deltas.
func (d Deltas) Oldest() *Delta {
	if len(d) > 0 {
		return &d[0]
	}
	return nil
}

// Newest is a convenience function that returns the newest delta, or
// nil if there are no deltas.
func (d Deltas) Newest() *Delta {
	if n := len(d); n > 0 {
		return &d[n-1]
	}
	return nil
}

// copyDeltas returns a shallow copy of d; that is, it copies the slice but not
// the objects in the slice. This allows Get/List to return an object that we
// know won't be clobbered by a subsequent modifications.
func copyDeltas(d Deltas) Deltas {
	d2 := make(Deltas, len(d))
	copy(d2, d)
	return d2
}

// DeletedFinalStateUnknown is placed into a DeltaFIFO in the case where an object
// was deleted but the watch deletion event was missed while disconnected from
// apiserver. In this case we don't know the final "resting" state of the object, so
// there's a chance the included `Obj` is stale.
type DeletedFinalStateUnknown struct {
	Key string
	Obj interface{}
}

```

// === FILE: references!/kubernetes/staging/src/k8s.io/client-go/tools/cache/reflector.go ===
```go
/*
Copyright 2014 The Kubernetes Authors.

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

package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/naming"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientfeatures "k8s.io/client-go/features"
	"k8s.io/client-go/tools/pager"
	"k8s.io/client-go/util/watchlist"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"
)

const defaultExpectedTypeName = "<unspecified>"

var (
	// We try to spread the load on apiserver by setting timeouts for
	// watch requests - it is random in [minWatchTimeout, 2*minWatchTimeout].
	defaultMinWatchTimeout = 5 * time.Minute
	defaultMaxWatchTimeout = 2 * defaultMinWatchTimeout
	// We used to make the call every 1sec (1 QPS), the goal here is to achieve ~98% traffic reduction when
	// API server is not healthy. With these parameters, backoff will stop at [30,60) sec interval which is
	// 0.22 QPS.
	defaultBackoffInit = 800 * time.Millisecond
	defaultBackoffMax  = 30 * time.Second
	// If we don't backoff for 2min, assume API server is healthy and we reset the backoff.
	defaultBackoffReset  = 2 * time.Minute
	defaultBackoffFactor = 2.0
	defaultBackoffJitter = 1.0
)

// ReflectorStore is the subset of cache.Store that the reflector uses
type ReflectorStore interface {
	// Add adds the given object to the accumulator associated with the given object's key
	Add(obj interface{}) error

	// Update updates the given object in the accumulator associated with the given object's key
	Update(obj interface{}) error

	// Delete deletes the given object from the accumulator associated with the given object's key
	Delete(obj interface{}) error

	// Replace will delete the contents of the store, using instead the
	// given list. Store takes ownership of the list, you should not reference
	// it after calling this function.
	Replace([]interface{}, string) error

	// Resync is meaningless in the terms appearing here but has
	// meaning in some implementations that have non-trivial
	// additional behavior (e.g., DeltaFIFO).
	Resync() error
}

// ReflectorBookmarkStore is an optional interface that allows a store
// to be informed of bookmark events received by the reflector.
type ReflectorBookmarkStore interface {
	Bookmark(resourceVersion string) error
}

// TransformingStore is an optional interface that can be implemented by the provided store.
// If implemented on the provided store reflector will use the same transformer in its internal stores.
type TransformingStore interface {
	ReflectorStore
	Transformer() TransformFunc
}

// Reflector watches a specified resource and causes all changes to be reflected in the given store.
type Reflector struct {
	logger klog.Logger
	// name identifies this reflector. By default, it will be a file:line if possible.
	name string
	// The name of the type we expect to place in the store. The name
	// will be the stringification of expectedGVK if provided, and the
	// stringification of expectedType otherwise. It is for display
	// only, and should not be used for parsing or comparison.
	typeDescription string
	// An example object of the type we expect to place in the store.
	// Only the type needs to be right, except that when that is
	// `unstructured.Unstructured` the object's `"apiVersion"` and
	// `"kind"` must also be right.
	expectedType reflect.Type
	// The GVK of the object we expect to place in the store if unstructured.
	expectedGVK *schema.GroupVersionKind
	// The destination to sync up with the watch source
	store ReflectorStore
	// listerWatcher is used to perform lists and watches.
	listerWatcher ListerWatcherWithContext
	// delay returns the next backoff interval for retries.
	resyncPeriod time.Duration
	delayHandler wait.DelayFunc
	// minWatchTimeout defines the minimum timeout for watch requests.
	minWatchTimeout time.Duration
	// maxWatchTimeout defines the maximum timeout for watch requests.
	// Actual timeout is random in [minWatchTimeout, maxWatchTimeout].
	maxWatchTimeout time.Duration
	// clock allows tests to manipulate time
	clock clock.Clock
	// paginatedResult defines whether pagination should be forced for list calls.
	// It is set based on the result of the initial list call.
	paginatedResult bool
	// lastSyncResourceVersion is the resource version token last
	// observed when doing a sync with the underlying store
	// it is thread safe, but not synchronized with the underlying store
	lastSyncResourceVersion string
	// isLastSyncResourceVersionUnavailable is true if the previous list or watch request with
	// lastSyncResourceVersion failed with an "expired" or "too large resource version" error.
	isLastSyncResourceVersionUnavailable bool
	// lastSyncResourceVersionMutex guards read/write access to lastSyncResourceVersion
	lastSyncResourceVersionMutex sync.RWMutex
	// Called whenever the ListAndWatch drops the connection with an error.
	watchErrorHandler WatchErrorHandlerWithContext
	// WatchListPageSize is the requested chunk size of initial and resync watch lists.
	// If unset, for consistent reads (RV="") or reads that opt-into arbitrarily old data
	// (RV="0") it will default to pager.PageSize, for the rest (RV != "" && RV != "0")
	// it will turn off pagination to allow serving them from watch cache.
	// NOTE: It should be used carefully as paginated lists are always served directly from
	// etcd, which is significantly less efficient and may lead to serious performance and
	// scalability problems.
	WatchListPageSize int64
	// ShouldResync is invoked periodically and whenever it returns `true` the Store's Resync operation is invoked
	ShouldResync func() bool
	// MaxInternalErrorRetryDuration defines how long we should retry internal errors returned by watch.
	MaxInternalErrorRetryDuration time.Duration
	// useWatchList if turned on instructs the reflector to open a stream to bring data from the API server.
	// Streaming has the primary advantage of using fewer server's resources to fetch data.
	//
	// The old behaviour establishes a LIST request which gets data in chunks.
	// Paginated list is less efficient and depending on the actual size of objects
	// might result in an increased memory consumption of the APIServer.
	//
	// See https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/3157-watch-list#design-details
	useWatchList bool
}

func (r *Reflector) Name() string {
	return r.name
}

func (r *Reflector) TypeDescription() string {
	return r.typeDescription
}

// ResourceVersionUpdater is an interface that allows store implementation to
// track the current resource version of the reflector. This is especially
// important if storage bookmarks are enabled.
type ResourceVersionUpdater interface {
	// UpdateResourceVersion is called each time current resource version of the reflector
	// is updated.
	UpdateResourceVersion(resourceVersion string)
}

// The WatchErrorHandler is called whenever ListAndWatch drops the
// connection with an error. After calling this handler, the informer
// will backoff and retry.
//
// The default implementation looks at the error type and tries to log
// the error message at an appropriate level.
//
// Implementations of this handler may display the error message in other
// ways. Implementations should return quickly - any expensive processing
// should be offloaded.
type WatchErrorHandler func(r *Reflector, err error)

// The WatchErrorHandler is called whenever ListAndWatch drops the
// connection with an error. After calling this handler, the informer
// will backoff and retry.
//
// The default implementation looks at the error type and tries to log
// the error message at an appropriate level.
//
// Implementations of this handler may display the error message in other
// ways. Implementations should return quickly - any expensive processing
// should be offloaded.
type WatchErrorHandlerWithContext func(ctx context.Context, r *Reflector, err error)

// DefaultWatchErrorHandler is the default implementation of WatchErrorHandlerWithContext.
func DefaultWatchErrorHandler(ctx context.Context, r *Reflector, err error) {
	switch {
	case isExpiredError(err):
		// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
		// has a semantic that it returns data at least as fresh as provided RV.
		// So first try to LIST with setting RV to resource version of last observed object.
		klog.FromContext(ctx).V(4).Info("Watch closed", "reflector", r.name, "type", r.typeDescription, "err", err)
	case err == io.EOF:
		// watch closed normally
	case err == io.ErrUnexpectedEOF:
		klog.FromContext(ctx).V(1).Info("Watch closed with unexpected EOF", "reflector", r.name, "type", r.typeDescription, "err", err)
	default:
		utilruntime.HandleErrorWithContext(ctx, err, "Failed to watch", "reflector", r.name, "type", r.typeDescription)
	}
}

// NewNamespaceKeyedIndexerAndReflector creates an Indexer and a Reflector
// The indexer is configured to key on namespace
func NewNamespaceKeyedIndexerAndReflector(lw ListerWatcher, expectedType interface{}, resyncPeriod time.Duration) (indexer Indexer, reflector *Reflector) {
	indexer = NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
	reflector = NewReflector(lw, expectedType, indexer, resyncPeriod)
	return indexer, reflector
}

// NewReflector creates a new Reflector with its name defaulted to the closest source_file.go:line in the call stack
// that is outside this package. See NewReflectorWithOptions for further information.
func NewReflector(lw ListerWatcher, expectedType interface{}, store ReflectorStore, resyncPeriod time.Duration) *Reflector {
	return NewReflectorWithOptions(lw, expectedType, store, ReflectorOptions{ResyncPeriod: resyncPeriod})
}

// NewNamedReflector creates a new Reflector with the specified name. See NewReflectorWithOptions for further
// information.
func NewNamedReflector(name string, lw ListerWatcher, expectedType interface{}, store ReflectorStore, resyncPeriod time.Duration) *Reflector {
	return NewReflectorWithOptions(lw, expectedType, store, ReflectorOptions{Name: name, ResyncPeriod: resyncPeriod})
}

// ReflectorOptions configures a Reflector.
type ReflectorOptions struct {
	// Logger, if not nil, is used instead of klog.Background() for logging.
	// The name of the reflector gets added automatically.
	Logger *klog.Logger

	// Name is the Reflector's name. If unset/unspecified, the name defaults to the closest source_file.go:line
	// in the call stack that is outside this package.
	Name string

	// TypeDescription is the Reflector's type description. If unset/unspecified, the type description is defaulted
	// using the following rules: if the expectedType passed to NewReflectorWithOptions was nil, the type description is
	// "<unspecified>". If the expectedType is an instance of *unstructured.Unstructured and its apiVersion and kind fields
	// are set, the type description is the string encoding of those. Otherwise, the type description is set to the
	// go type of expectedType..
	TypeDescription string

	// ResyncPeriod is the Reflector's resync period. If unset/unspecified, the resync period defaults to 0
	// (do not resync).
	ResyncPeriod time.Duration

	// MinWatchTimeout, if non-zero, defines the minimum timeout for watch requests send to kube-apiserver.
	// However, values lower than 5m will not be honored to avoid negative performance impact on controlplane.
	MinWatchTimeout time.Duration

	// Clock allows tests to control time. If unset defaults to clock.RealClock{}
	Clock clock.Clock

	// Backoff is an optional custom backoff configuration.
	// If set, it will be used instead of the default exponential backoff.
	// DelayWithReset(clock, resetDuration) will be called on it to create the delay function.
	// TODO(#136943): Expose this configuration through SharedInformerFactory.
	Backoff *wait.Backoff
}

// NewReflectorWithOptions creates a new Reflector object which will keep the
// given store up to date with the server's contents for the given
// resource. Reflector promises to only put things in the store that
// have the type of expectedType, unless expectedType is nil. If
// resyncPeriod is non-zero, then the reflector will periodically
// consult its ShouldResync function to determine whether to invoke
// the Store's Resync operation; `ShouldResync==nil` means always
// "yes".  This enables you to use reflectors to periodically process
// everything as well as incrementally processing the things that
// change.
func NewReflectorWithOptions(lw ListerWatcher, expectedType interface{}, store ReflectorStore, options ReflectorOptions) *Reflector {
	reflectorClock := options.Clock
	if reflectorClock == nil {
		reflectorClock = clock.RealClock{}
	}

	minWatchTimeout := defaultMinWatchTimeout
	maxWatchTimeout := defaultMaxWatchTimeout
	if options.MinWatchTimeout > defaultMinWatchTimeout {
		minWatchTimeout = options.MinWatchTimeout
		maxWatchTimeout = 2 * minWatchTimeout
	}
	if maxWatchTimeout < minWatchTimeout {
		klog.TODO().V(3).Info(
			"maxWatchTimeout was less than minWatchTimeout, overriding to minWatchTimeout. Watch timeout randomization is disabled.",
			"minWatchTimeout", minWatchTimeout,
			"maxWatchTimeout", maxWatchTimeout,
		)
		maxWatchTimeout = minWatchTimeout
	}

	backoff := options.Backoff
	if backoff == nil {
		backoff = &wait.Backoff{
			Duration: defaultBackoffInit,
			Cap:      defaultBackoffMax,
			Steps:    int(math.Ceil(float64(defaultBackoffMax) / float64(defaultBackoffInit))),
			Factor:   defaultBackoffFactor,
			Jitter:   defaultBackoffJitter,
		}
	}

	r := &Reflector{
		name:              options.Name,
		resyncPeriod:      options.ResyncPeriod,
		minWatchTimeout:   minWatchTimeout,
		maxWatchTimeout:   maxWatchTimeout,
		typeDescription:   options.TypeDescription,
		listerWatcher:     ToListerWatcherWithContext(lw),
		store:             store,
		delayHandler:      backoff.DelayWithReset(reflectorClock, defaultBackoffReset),
		clock:             reflectorClock,
		watchErrorHandler: WatchErrorHandlerWithContext(DefaultWatchErrorHandler),
		expectedType:      reflect.TypeOf(expectedType),
	}

	if r.name == "" {
		r.name = naming.GetNameFromCallsite(internalPackages...)
	}

	logger := klog.Background()
	if options.Logger != nil {
		logger = *options.Logger
	}
	logger = klog.LoggerWithName(logger, r.name)
	r.logger = logger

	if r.typeDescription == "" {
		r.typeDescription = getTypeDescriptionFromObject(expectedType)
	}

	if r.expectedGVK == nil {
		r.expectedGVK = getExpectedGVKFromObject(expectedType)
	}

	r.useWatchList = clientfeatures.FeatureGates().Enabled(clientfeatures.WatchListClient)
	if r.useWatchList && watchlist.DoesClientNotSupportWatchListSemantics(lw) {
		r.logger.V(2).Info(
			"The client used to build this informer/reflector doesn't support WatchList semantics. The feature will be disabled. This is expected in unit tests but not in production. For details, see the documentation of watchlist.DoesClientNotSupportWatchListSemantics().",
			"feature", clientfeatures.WatchListClient,
		)
		r.useWatchList = false
	}

	return r
}

func getTypeDescriptionFromObject(expectedType interface{}) string {
	if expectedType == nil {
		return defaultExpectedTypeName
	}

	reflectDescription := reflect.TypeOf(expectedType).String()

	obj, ok := expectedType.(*unstructured.Unstructured)
	if !ok {
		return reflectDescription
	}

	gvk := obj.GroupVersionKind()
	if gvk.Empty() {
		return reflectDescription
	}

	return gvk.String()
}

func getExpectedGVKFromObject(expectedType interface{}) *schema.GroupVersionKind {
	obj, ok := expectedType.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	gvk := obj.GroupVersionKind()
	if gvk.Empty() {
		return nil
	}

	return &gvk
}

// internalPackages are packages that ignored when creating a default reflector name. These packages are in the common
// call chains to NewReflector, so they'd be low entropy names for reflectors
var internalPackages = []string{"client-go/tools/cache/"}

// Run repeatedly uses the reflector's ListAndWatch to fetch all the
// objects and subsequent deltas.
// Run will exit when stopCh is closed.
//
// Contextual logging: RunWithContext should be used instead of Run in code which supports contextual logging.
func (r *Reflector) Run(stopCh <-chan struct{}) {
	r.RunWithContext(wait.ContextForChannel(stopCh))
}

// RunWithContext repeatedly uses the reflector's ListAndWatch to fetch all the
// objects and subsequent deltas.
// Run will exit when the context is canceled.
func (r *Reflector) RunWithContext(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Starting reflector", "type", r.typeDescription, "resyncPeriod", r.resyncPeriod, "reflector", r.name)
	// Until runs the loop immediately (immediate=true) and resets the backoff timer after each
	// successful iteration (sliding=true). See backoff constants at top of file for generalized QPS targets (~0.22 QPS).
	_ = r.delayHandler.Until(ctx, true, true, func(ctx context.Context) (bool, error) {
		if err := r.ListAndWatchWithContext(ctx); err != nil {
			r.watchErrorHandler(ctx, r, err)
		}
		return false, nil
	})
	logger.V(3).Info("Stopping reflector", "type", r.typeDescription, "resyncPeriod", r.resyncPeriod, "reflector", r.name)
}

var (
	// Used to indicate that watching stopped because of a signal from the stop
	// channel passed in from a client of the reflector.
	errorStopRequested = errors.New("stop requested")
)

// resyncChan returns a channel which will receive something when a resync is
// required, and a cleanup function.
func (r *Reflector) resyncChan() (<-chan time.Time, func() bool) {
	if r.resyncPeriod == 0 {
		// nothing will ever be sent down this channel
		return nil, func() bool { return false }
	}
	// The cleanup function is required: imagine the scenario where watches
	// always fail so we end up listing frequently. Then, if we don't
	// manually stop the timer, we could end up with many timers active
	// concurrently.
	t := r.clock.NewTimer(r.resyncPeriod)
	return t.C(), t.Stop
}

// ListAndWatch first lists all items and get the resource version at the moment of call,
// and then use the resource version to watch.
// It returns error if ListAndWatch didn't even try to initialize watch.
//
// Contextual logging: ListAndWatchWithContext should be used instead of ListAndWatch in code which supports contextual logging.
func (r *Reflector) ListAndWatch(stopCh <-chan struct{}) error {
	return r.ListAndWatchWithContext(wait.ContextForChannel(stopCh))
}

// ListAndWatchWithContext first lists all items and get the resource version at the moment of call,
// and then use the resource version to watch.
// It returns error if ListAndWatchWithContext didn't even try to initialize watch.
func (r *Reflector) ListAndWatchWithContext(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Listing and watching", "type", r.typeDescription, "reflector", r.name)
	var err error
	var w watch.Interface
	fallbackToList := !r.useWatchList

	defer func() {
		if w != nil {
			w.Stop()
		}
	}()

	if r.useWatchList {
		w, err = r.watchList(ctx)
		if w == nil && err == nil {
			// stopCh was closed
			return nil
		}
		if err != nil {
			logger.V(4).Info(
				"Data couldn't be fetched in watchlist mode. Falling back to regular list. This is expected if watchlist is not supported or disabled in kube-apiserver.",
				"err", err,
			)
			fallbackToList = true
			// ensure that we won't accidentally pass some garbage down the watch.
			w = nil
		}
	}

	if fallbackToList {
		err = r.list(ctx)
		if err != nil {
			return err
		}
	}

	logger.V(2).Info("Caches populated", "type", r.typeDescription, "reflector", r.name)
	return r.watchWithResync(ctx, w)
}

// startResync periodically calls r.store.Resync() method.
// Note that this method is blocking and should be
// called in a separate goroutine.
func (r *Reflector) startResync(ctx context.Context, resyncerrc chan error) {
	logger := klog.FromContext(ctx)
	resyncCh, cleanup := r.resyncChan()
	defer func() {
		cleanup() // Call the last one written into cleanup
	}()
	for {
		select {
		case <-resyncCh:
		case <-ctx.Done():
			return
		}
		if r.ShouldResync == nil || r.ShouldResync() {
			logger.V(4).Info("Forcing resync", "reflector", r.name)
			if err := r.store.Resync(); err != nil {
				resyncerrc <- err
				return
			}
		}
		cleanup()
		resyncCh, cleanup = r.resyncChan()
	}
}

// watchWithResync runs watch with startResync in the background.
func (r *Reflector) watchWithResync(ctx context.Context, w watch.Interface) error {
	resyncerrc := make(chan error, 1)
	cancelCtx, cancel := context.WithCancel(ctx)
	// Waiting for completion of the goroutine is relevant for race detector.
	// Without this, there is a race between "this function returns + code
	// waiting for it" and "goroutine does something".
	var wg wait.Group
	defer func() {
		cancel()
		wg.Wait()
	}()
	wg.Start(func() {
		r.startResync(cancelCtx, resyncerrc)
	})
	return r.watch(ctx, w, resyncerrc)
}

// watch starts a watch request with the server, consumes watch events, and
// restarts the watch until an exit scenario is reached.
//
// If a watch is provided, it will be used, otherwise another will be started.
// If the watcher has started, it will always be stopped before returning.
func (r *Reflector) watch(ctx context.Context, w watch.Interface, resyncerrc chan error) error {
	stopCh := ctx.Done()
	logger := klog.FromContext(ctx)
	var err error
	retry := NewRetryWithDeadline(r.MaxInternalErrorRetryDuration, time.Minute, apierrors.IsInternalError, r.clock)
	defer func() {
		if w != nil {
			w.Stop()
		}
	}()

	for {
		// give the stopCh a chance to stop the loop, even in case of continue statements further down on errors
		select {
		case <-stopCh:
			// we can only end up here when the stopCh
			// was closed after a successful watchlist or list request
			return nil
		default:
		}

		// start the clock before sending the request, since some proxies won't flush headers until after the first watch event is sent
		start := r.clock.Now()

		// if w is already initialized, it must be past any synthetic non-rv-ordered added events
		propagateRVFromStart := true
		if w == nil {
			timeoutSeconds := int64(r.minWatchTimeout.Seconds() + rand.Float64()*(r.maxWatchTimeout.Seconds()-r.minWatchTimeout.Seconds()))
			options := metav1.ListOptions{
				ResourceVersion: r.LastSyncResourceVersion(),
				// We want to avoid situations of hanging watchers. Stop any watchers that do not
				// receive any events within the timeout window.
				TimeoutSeconds: &timeoutSeconds,
				// To reduce load on kube-apiserver on watch restarts, you may enable watch bookmarks.
				// Reflector doesn't assume bookmarks are returned at all (if the server do not support
				// watch bookmarks, it will ignore this field).
				AllowWatchBookmarks: true,
			}
			if options.ResourceVersion == "" || options.ResourceVersion == "0" {
				// if we're starting the watch at a resource version that will get synthetic ADDED events in non-rv order,
				// wait until we're through that set of events before propagating the RV
				propagateRVFromStart = false
			}

			w, err = r.listerWatcher.WatchWithContext(ctx, options)
			if err != nil {
				if canRetry := isWatchErrorRetriable(err); canRetry {
					logger.V(4).Info("Watch failed - backing off", "reflector", r.name, "type", r.typeDescription, "err", err)
					select {
					case <-stopCh:
						return nil
					case <-r.clock.After(r.delayHandler()):
						continue
					}
				}
				return err
			}
		}

		err = handleWatch(ctx, start, w, r.store, r.expectedType, r.expectedGVK, r.name, r.typeDescription,
			func(rv string, eventReceivedBesidesAdded bool) {
				// We update the resource version in the store only if we have received at least one event that is
				// not an added event, or if the resource version has been set previously. This is because we can
				// encounter 2 scenarios:
				// 1. The watch is started from a resource version specified by the LastSyncResourceVersion field.
				//    In this case, we can update the resource version in the store without worrying about it being
				//    out of order since we will not receive any synthetic added events for resources that may be
				//    out of order.
				// 2. The watch is started when the LastSyncResourceVersion field is empty. In this case, we may not
				//    update the LastSyncResourceVersion until we receive at least one event that is not an added
				//    event, since that is the only way to ensure that the watch has exited the initial list phase.
				if propagateRVFromStart || eventReceivedBesidesAdded {
					r.setLastSyncResourceVersion(rv)
					if rvu, ok := r.store.(ResourceVersionUpdater); ok {
						rvu.UpdateResourceVersion(rv)
					}
				}
			},
			r.clock, resyncerrc)
		// handleWatch always stops the watcher. So we don't need to here.
		// Just set it to nil to trigger a retry on the next loop.
		w = nil
		retry.After(err)
		if err != nil {
			if !errors.Is(err, errorStopRequested) {
				switch {
				case isExpiredError(err):
					// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
					// has a semantic that it returns data at least as fresh as provided RV.
					// So first try to LIST with setting RV to resource version of last observed object.
					logger.V(4).Info("Watch closed", "reflector", r.name, "type", r.typeDescription, "err", err)
				case apierrors.IsTooManyRequests(err):
					logger.V(2).Info("Watch returned 429 - backing off", "reflector", r.name, "type", r.typeDescription)
					select {
					case <-stopCh:
						return nil
					case <-r.clock.After(r.delayHandler()):
						continue
					}
				case apierrors.IsInternalError(err) && retry.ShouldRetry():
					logger.V(2).Info("Retrying watch after internal error", "reflector", r.name, "type", r.typeDescription, "err", err)
					continue
				default:
					logger.Info("Warning: watch ended with error", "reflector", r.name, "type", r.typeDescription, "err", err)
				}
			}
			return nil
		}
	}
}

// list simply lists all items and records a resource version obtained from the server at the moment of the call.
// the resource version can be used for further progress notification (aka. watch).
func (r *Reflector) list(ctx context.Context) error {
	var resourceVersion string
	options := metav1.ListOptions{ResourceVersion: r.relistResourceVersion()}

	initTrace := trace.New("Reflector ListAndWatch", trace.Field{Key: "name", Value: r.name})
	defer initTrace.LogIfLong(10 * time.Second)
	var list runtime.Object
	var paginatedResult bool
	var err error
	listCh := make(chan struct{}, 1)
	panicCh := make(chan interface{}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicCh <- r
			}
		}()
		// Attempt to gather list in chunks, if supported by listerWatcher, if not, the first
		// list request will return the full response.
		pager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
			return r.listerWatcher.ListWithContext(ctx, opts)
		}))
		switch {
		case r.WatchListPageSize != 0:
			pager.PageSize = r.WatchListPageSize
		case r.paginatedResult:
			// We got a paginated result initially. Assume this resource and server honor
			// paging requests (i.e. watch cache is probably disabled) and leave the default
			// pager size set.
		case options.ResourceVersion != "" && options.ResourceVersion != "0":
			// User didn't explicitly request pagination.
			//
			// With ResourceVersion != "", we have a possibility to list from watch cache,
			// but we do that (for ResourceVersion != "0") only if Limit is unset.
			// To avoid thundering herd on etcd (e.g. on master upgrades), we explicitly
			// switch off pagination to force listing from watch cache (if enabled).
			// With the existing semantic of RV (result is at least as fresh as provided RV),
			// this is correct and doesn't lead to going back in time.
			//
			// We also don't turn off pagination for ResourceVersion="0", since watch cache
			// is ignoring Limit in that case anyway, and if watch cache is not enabled
			// we don't introduce regression.
			pager.PageSize = 0
		}

		list, paginatedResult, err = pager.ListWithAlloc(context.Background(), options)
		if isExpiredError(err) || isTooLargeResourceVersionError(err) {
			r.setIsLastSyncResourceVersionUnavailable(true)
			// Retry immediately if the resource version used to list is unavailable.
			// The pager already falls back to full list if paginated list calls fail due to an "Expired" error on
			// continuation pages, but the pager might not be enabled, the full list might fail because the
			// resource version it is listing at is expired or the cache may not yet be synced to the provided
			// resource version. So we need to fallback to resourceVersion="" in all to recover and ensure
			// the reflector makes forward progress.
			list, paginatedResult, err = pager.ListWithAlloc(context.Background(), metav1.ListOptions{ResourceVersion: r.relistResourceVersion()})
		}
		if err == nil {
			if unsupportedList, unsupportedListGVK := isUnsupportedTableListObject(list); unsupportedList {
				err = fmt.Errorf("unsupported list gvk: %v, type: %v", unsupportedListGVK, r.typeDescription)
			}
		}
		close(listCh)
	}()
	select {
	case <-ctx.Done():
		return nil
	case r := <-panicCh:
		panic(r)
	case <-listCh:
	}

	initTrace.Step("Objects listed", trace.Field{Key: "error", Value: err}, trace.Field{Key: "count", Value: meta.LenList(list)})
	if err != nil {
		return fmt.Errorf("failed to list %v: %w", r.typeDescription, err)
	}

	// We check if the list was paginated and if so set the paginatedResult based on that.
	// However, we want to do that only for the initial list (which is the only case
	// when we set ResourceVersion="0"). The reasoning behind it is that later, in some
	// situations we may force listing directly from etcd (by setting ResourceVersion="")
	// which will return paginated result, even if watch cache is enabled. However, in
	// that case, we still want to prefer sending requests to watch cache if possible.
	//
	// Paginated result returned for request with ResourceVersion="0" mean that watch
	// cache is disabled and there are a lot of objects of a given type. In such case,
	// there is no need to prefer listing from watch cache.
	if options.ResourceVersion == "0" && paginatedResult {
		r.paginatedResult = true
	}

	r.setIsLastSyncResourceVersionUnavailable(false) // list was successful
	listMetaInterface, err := meta.ListAccessor(list)
	if err != nil {
		return fmt.Errorf("unable to understand list result %#v: %v", list, err)
	}
	resourceVersion = listMetaInterface.GetResourceVersion()
	initTrace.Step("Resource version extracted")
	items, err := meta.ExtractListWithAlloc(list)
	if err != nil {
		return fmt.Errorf("unable to understand list result %#v (%v)", list, err)
	}
	initTrace.Step("Objects extracted")
	if err := r.syncWith(items, resourceVersion); err != nil {
		return fmt.Errorf("unable to sync list result: %v", err)
	}
	initTrace.Step("SyncWith done")
	r.setLastSyncResourceVersion(resourceVersion)
	initTrace.Step("Resource version updated")
	return nil
}

// watchList establishes a stream to get a consistent snapshot of data
// from the server as described in https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/3157-watch-list#proposal
//
// case 1: start at Most Recent (RV="", ResourceVersionMatch=ResourceVersionMatchNotOlderThan)
// Establishes a consistent stream with the server.
// That means the returned data is consistent, as if, served directly from etcd via a quorum read.
// It begins with synthetic "Added" events of all resources up to the most recent ResourceVersion.
// It ends with a synthetic "Bookmark" event containing the most recent ResourceVersion.
// After receiving a "Bookmark" event the reflector is considered to be synchronized.
// It replaces its internal store with the collected items and
// reuses the current watch requests for getting further events.
//
// case 2: start at Exact (RV>"0", ResourceVersionMatch=ResourceVersionMatchNotOlderThan)
// Establishes a stream with the server at the provided resource version.
// To establish the initial state the server begins with synthetic "Added" events.
// It ends with a synthetic "Bookmark" event containing the provided or newer resource version.
// After receiving a "Bookmark" event the reflector is considered to be synchronized.
// It replaces its internal store with the collected items and
// reuses the current watch requests for getting further events.
func (r *Reflector) watchList(ctx context.Context) (watch.Interface, error) {
	stopCh := ctx.Done()
	logger := klog.FromContext(ctx)
	var w watch.Interface
	var err error
	var temporaryStore Store
	var resourceVersion string
	// TODO(#115478): see if this function could be turned
	//  into a method and see if error handling
	//  could be unified with the r.watch method
	isErrorRetriableWithSideEffectsFn := func(err error) bool {
		if canRetry := isWatchErrorRetriable(err); canRetry {
			logger.V(2).Info("watch-list failed - backing off", "reflector", r.name, "type", r.typeDescription, "err", err)
			<-r.clock.After(r.delayHandler())
			return true
		}
		if isExpiredError(err) || isTooLargeResourceVersionError(err) {
			// we tried to re-establish a watch request but the provided RV
			// has either expired or it is greater than the server knows about.
			// In that case we reset the RV and
			// try to get a consistent snapshot from the watch cache (case 1)
			r.setIsLastSyncResourceVersionUnavailable(true)
			return true
		}
		return false
	}

	var transformer TransformFunc
	storeOpts := []StoreOption{}
	if tr, ok := r.store.(TransformingStore); ok && tr.Transformer() != nil {
		transformer = tr.Transformer()
		storeOpts = append(storeOpts, WithTransformer(transformer))
	}

	initTrace := trace.New("Reflector WatchList", trace.Field{Key: "name", Value: r.name})
	defer initTrace.LogIfLong(10 * time.Second)
	for {
		select {
		case <-stopCh:
			return nil, nil
		default:
		}

		resourceVersion = ""
		lastKnownRV := r.rewatchResourceVersion()
		temporaryStore = NewStore(DeletionHandlingMetaNamespaceKeyFunc, storeOpts...)
		// TODO(#115478): large "list", slow clients, slow network, p&f
		//  might slow down streaming and eventually fail.
		//  maybe in such a case we should retry with an increased timeout?
		timeoutSeconds := int64(r.minWatchTimeout.Seconds() + rand.Float64()*(r.maxWatchTimeout.Seconds()-r.minWatchTimeout.Seconds()))
		options := metav1.ListOptions{
			ResourceVersion:      lastKnownRV,
			AllowWatchBookmarks:  true,
			SendInitialEvents:    ptr.To(true),
			ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
			TimeoutSeconds:       &timeoutSeconds,
		}
		start := r.clock.Now()

		w, err = r.listerWatcher.WatchWithContext(ctx, options)
		if err != nil {
			if isErrorRetriableWithSideEffectsFn(err) {
				continue
			}
			return nil, err
		}
		watchListBookmarkReceived, err := handleListWatch(ctx, start, w, temporaryStore, r.expectedType, r.expectedGVK, r.name, r.typeDescription,
			func(rv string, eventReceivedBesidesAdded bool) {
				if eventReceivedBesidesAdded {
					resourceVersion = rv
				}
			},
			r.clock, make(chan error))
		if err != nil {
			w.Stop() // stop and retry with clean state
			if errors.Is(err, errorStopRequested) {
				return nil, nil
			}
			if isErrorRetriableWithSideEffectsFn(err) {
				continue
			}
			return nil, err
		}
		if watchListBookmarkReceived {
			break
		}
	}
	// We successfully got initial state from watch-list confirmed by the
	// "k8s.io/initial-events-end" bookmark.
	initTrace.Step("Objects streamed", trace.Field{Key: "count", Value: len(temporaryStore.List())})
	r.setIsLastSyncResourceVersionUnavailable(false)

	// we utilize the temporaryStore to ensure independence from the current store implementation.
	// as of today, the store is implemented as a queue and will be drained by the higher-level
	// component as soon as it finishes replacing the content.
	checkWatchListDataConsistencyIfRequested(ctx, r.name, resourceVersion, r.listerWatcher.ListWithContext, transformer, temporaryStore.List)

	if err := r.store.Replace(temporaryStore.List(), resourceVersion); err != nil {
		return nil, fmt.Errorf("unable to sync watch-list result: %w", err)
	}
	initTrace.Step("SyncWith done")
	r.setLastSyncResourceVersion(resourceVersion)

	return w, nil
}

// syncWith replaces the store's items with the given list.
func (r *Reflector) syncWith(items []runtime.Object, resourceVersion string) error {
	found := make([]interface{}, 0, len(items))
	for _, item := range items {
		found = append(found, item)
	}
	return r.store.Replace(found, resourceVersion)
}

// handleListWatch consumes events from w, updates the Store, and records the
// last seen ResourceVersion, to allow continuing from that ResourceVersion on
// retry. If successful, the watcher will be left open after receiving the
// initial set of objects, to allow watching for future events.
func handleListWatch(
	ctx context.Context,
	start time.Time,
	w watch.Interface,
	store Store,
	expectedType reflect.Type,
	expectedGVK *schema.GroupVersionKind,
	name string,
	expectedTypeName string,
	setLastSyncResourceVersion func(string, bool),
	clock clock.Clock,
	errCh chan error,
) (bool, error) {
	exitOnWatchListBookmarkReceived := true
	return handleAnyWatch(ctx, start, w, store, expectedType, expectedGVK, name, expectedTypeName,
		setLastSyncResourceVersion, exitOnWatchListBookmarkReceived, clock, errCh)
}

// handleListWatch consumes events from w, updates the Store, and records the
// last seen ResourceVersion, to allow continuing from that ResourceVersion on
// retry. The watcher will always be stopped on exit.
func handleWatch(
	ctx context.Context,
	start time.Time,
	w watch.Interface,
	store ReflectorStore,
	expectedType reflect.Type,
	expectedGVK *schema.GroupVersionKind,
	name string,
	expectedTypeName string,
	setLastSyncResourceVersion func(string, bool),
	clock clock.Clock,
	errCh chan error,
) error {
	exitOnWatchListBookmarkReceived := false
	_, err := handleAnyWatch(ctx, start, w, store, expectedType, expectedGVK, name, expectedTypeName,
		setLastSyncResourceVersion, exitOnWatchListBookmarkReceived, clock, errCh)
	return err
}

// handleAnyWatch consumes events from w, updates the Store, and records the last
// seen ResourceVersion, to allow continuing from that ResourceVersion on retry.
// If exitOnWatchListBookmarkReceived is true, the watch events will be consumed
// until a bookmark event is received with the WatchList annotation present.
// Returns true (watchListBookmarkReceived) if the WatchList bookmark was
// received, even if exitOnWatchListBookmarkReceived is false.
// The watcher will always be stopped, unless exitOnWatchListBookmarkReceived is
// true and watchListBookmarkReceived is true. This allows the same watch stream
// to be re-used by the caller to continue watching for new events.
func handleAnyWatch(
	ctx context.Context,
	start time.Time,
	w watch.Interface,
	store ReflectorStore,
	expectedType reflect.Type,
	expectedGVK *schema.GroupVersionKind,
	name string,
	expectedTypeName string,
	setLastSyncResourceVersion func(string, bool),
	exitOnWatchListBookmarkReceived bool,
	clock clock.Clock,
	errCh chan error,
) (bool, error) {
	watchListBookmarkReceived := false
	eventReceivedBesidesAdded := false
	eventCount := 0
	logger := klog.FromContext(ctx)
	initialEventsEndBookmarkWarningTicker := newInitialEventsEndBookmarkTicker(logger, name, clock, start, exitOnWatchListBookmarkReceived)
	defer initialEventsEndBookmarkWarningTicker.Stop()
	stopWatcher := true
	defer func() {
		if stopWatcher {
			w.Stop()
		}
	}()

loop:
	for {
		select {
		case <-ctx.Done():
			return watchListBookmarkReceived, errorStopRequested
		case err := <-errCh:
			return watchListBookmarkReceived, err
		case event, ok := <-w.ResultChan():
			if !ok {
				break loop
			}
			if event.Type == watch.Error {
				return watchListBookmarkReceived, apierrors.FromObject(event.Object)
			}
			if expectedType != nil {
				if e, a := expectedType, reflect.TypeOf(event.Object); e != a {
					utilruntime.HandleErrorWithContext(ctx, nil, "Unexpected watch event object type", "reflector", name, "expectedType", e, "actualType", a)
					continue
				}
			}
			if expectedGVK != nil {
				if e, a := *expectedGVK, event.Object.GetObjectKind().GroupVersionKind(); e != a {
					utilruntime.HandleErrorWithContext(ctx, nil, "Unexpected watch event object gvk", "reflector", name, "expectedGVK", e, "actualGVK", a)
					continue
				}
			}
			// we don't support receiving resources in Table format
			// see #132926 for more info
			if unsupportedGVK := isUnsupportedTableObject(event.Object); unsupportedGVK {
				utilruntime.HandleErrorWithContext(ctx, nil, "Unsupported watch event object gvk", "reflector", name, "actualGVK", event.Object.GetObjectKind().GroupVersionKind())
				continue
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				utilruntime.HandleErrorWithContext(ctx, err, "Unable to understand watch event", "reflector", name, "event", event)
				continue
			}
			resourceVersion := meta.GetResourceVersion()
			switch event.Type {
			case watch.Added:
				err := store.Add(event.Object)
				if err != nil {
					utilruntime.HandleErrorWithContext(ctx, err, "Unable to add watch event object to store", "reflector", name, "object", event.Object)
				}
			case watch.Modified:
				eventReceivedBesidesAdded = true
				err := store.Update(event.Object)
				if err != nil {
					utilruntime.HandleErrorWithContext(ctx, err, "Unable to update watch event object to store", "reflector", name, "object", event.Object)
				}
			case watch.Deleted:
				// TODO: Will any consumers need access to the "last known
				// state", which is passed in event.Object? If so, may need
				// to change this.
				eventReceivedBesidesAdded = true
				err := store.Delete(event.Object)
				if err != nil {
					utilruntime.HandleErrorWithContext(ctx, err, "Unable to delete watch event object from store", "reflector", name, "object", event.Object)
				}
			case watch.Bookmark:
				// A `Bookmark` means watch has synced here, just update the resourceVersion
				eventReceivedBesidesAdded = true
				if meta.GetAnnotations()[metav1.InitialEventsAnnotationKey] == "true" {
					watchListBookmarkReceived = true
				}
				// Propagate the resource version from the bookmark event to stores which indicate they want it
				if bookmarkStore, ok := store.(ReflectorBookmarkStore); ok {
					err := bookmarkStore.Bookmark(resourceVersion)
					if err != nil {
						utilruntime.HandleErrorWithContext(ctx, err, "Unable to send bookmark event to store", "reflector", name, "object", event.Object)
					}
				}
			default:
				utilruntime.HandleErrorWithContext(ctx, err, "Unknown watch event", "reflector", name, "event", event)
			}
			// when eventReceivedBesidesAdded is true, that indicates we are definitely past any initial synthetic Added events
			setLastSyncResourceVersion(resourceVersion, eventReceivedBesidesAdded)
			eventCount++
			if exitOnWatchListBookmarkReceived && watchListBookmarkReceived {
				stopWatcher = false
				watchDuration := clock.Since(start)
				klog.FromContext(ctx).V(4).Info("Exiting watch because received the bookmark that marks the end of initial events stream", "reflector", name, "totalItems", eventCount, "duration", watchDuration)
				return watchListBookmarkReceived, nil
			}
			initialEventsEndBookmarkWarningTicker.observeLastEventTimeStamp(clock.Now())
		case <-initialEventsEndBookmarkWarningTicker.C():
			initialEventsEndBookmarkWarningTicker.warnIfExpired()
		}
	}

	watchDuration := clock.Since(start)
	if watchDuration < 1*time.Second && eventCount == 0 {
		return watchListBookmarkReceived, &VeryShortWatchError{Name: name}
	}
	klog.FromContext(ctx).V(4).Info("Watch close", "reflector", name, "type", expectedTypeName, "totalItems", eventCount)
	return watchListBookmarkReceived, nil
}

// LastSyncResourceVersion is the resource version observed when last sync with the underlying store
// The value returned is not synchronized with access to the underlying store and is not thread-safe
func (r *Reflector) LastSyncResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()
	return r.lastSyncResourceVersion
}

func (r *Reflector) setLastSyncResourceVersion(v string) {
	r.lastSyncResourceVersionMutex.Lock()
	defer r.lastSyncResourceVersionMutex.Unlock()
	r.lastSyncResourceVersion = v
}

// relistResourceVersion determines the resource version the reflector should list or relist from.
// Returns either the lastSyncResourceVersion so that this reflector will relist with a resource
// versions no older than has already been observed in relist results or watch events, or, if the last relist resulted
// in an HTTP 410 (Gone) status code, returns "" so that the relist will use the latest resource version available in
// etcd via a quorum read.
func (r *Reflector) relistResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()

	if r.isLastSyncResourceVersionUnavailable {
		// Since this reflector makes paginated list requests, and all paginated list requests skip the watch cache
		// if the lastSyncResourceVersion is unavailable, we set ResourceVersion="" and list again to re-establish reflector
		// to the latest available ResourceVersion, using a consistent read from etcd.
		return ""
	}
	if r.lastSyncResourceVersion == "" {
		// For performance reasons, initial list performed by reflector uses "0" as resource version to allow it to
		// be served from the watch cache if it is enabled.
		return "0"
	}
	return r.lastSyncResourceVersion
}

// rewatchResourceVersion determines the resource version the reflector should start streaming from.
func (r *Reflector) rewatchResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()
	if r.isLastSyncResourceVersionUnavailable {
		// initial stream should return data at the most recent resource version.
		// the returned data must be consistent i.e. as if served from etcd via a quorum read
		return ""
	}
	return r.lastSyncResourceVersion
}

// setIsLastSyncResourceVersionUnavailable sets if the last list or watch request with lastSyncResourceVersion returned
// "expired" or "too large resource version" error.
func (r *Reflector) setIsLastSyncResourceVersionUnavailable(isUnavailable bool) {
	r.lastSyncResourceVersionMutex.Lock()
	defer r.lastSyncResourceVersionMutex.Unlock()
	r.isLastSyncResourceVersionUnavailable = isUnavailable
}

func isExpiredError(err error) bool {
	// In Kubernetes 1.17 and earlier, the api server returns both apierrors.StatusReasonExpired and
	// apierrors.StatusReasonGone for HTTP 410 (Gone) status code responses. In 1.18 the kube server is more consistent
	// and always returns apierrors.StatusReasonExpired. For backward compatibility we can only remove the apierrors.IsGone
	// check when we fully drop support for Kubernetes 1.17 servers from reflectors.
	return apierrors.IsResourceExpired(err) || apierrors.IsGone(err)
}

func isTooLargeResourceVersionError(err error) bool {
	if apierrors.HasStatusCause(err, metav1.CauseTypeResourceVersionTooLarge) {
		return true
	}
	// In Kubernetes 1.17.0-1.18.5, the api server doesn't set the error status cause to
	// metav1.CauseTypeResourceVersionTooLarge to indicate that the requested minimum resource
	// version is larger than the largest currently available resource version. To ensure backward
	// compatibility with these server versions we also need to detect the error based on the content
	// of the error message field.
	if !apierrors.IsTimeout(err) {
		return false
	}
	apierr, ok := err.(apierrors.APIStatus)
	if !ok || apierr == nil || apierr.Status().Details == nil {
		return false
	}
	for _, cause := range apierr.Status().Details.Causes {
		// Matches the message returned by api server 1.17.0-1.18.5 for this error condition
		if cause.Message == "Too large resource version" {
			return true
		}
	}

	// Matches the message returned by api server before 1.17.0
	if strings.Contains(apierr.Status().Message, "Too large resource version") {
		return true
	}

	return false
}

// isWatchErrorRetriable determines if it is safe to retry
// a watch error retrieved from the server.
func isWatchErrorRetriable(err error) bool {
	// If this is "connection refused" error, it means that most likely apiserver is not responsive.
	// It doesn't make sense to re-list all objects because most likely we will be able to restart
	// watch where we ended.
	// If that's the case begin exponentially backing off and resend watch request.
	// Do the same for "429" errors.
	if utilnet.IsConnectionRefused(err) || apierrors.IsTooManyRequests(err) {
		return true
	}
	return false
}

// initialEventsEndBookmarkTicker a ticker that produces a warning if the bookmark event
// which marks the end of the watch stream, has not been received within the defined tick interval.
//
// Note:
// The methods exposed by this type are not thread-safe.
type initialEventsEndBookmarkTicker struct {
	clock.Ticker
	clock  clock.Clock
	name   string
	logger klog.Logger

	watchStart           time.Time
	tickInterval         time.Duration
	lastEventObserveTime time.Time
}

// newInitialEventsEndBookmarkTicker returns a noop ticker if exitOnInitialEventsEndBookmarkRequested is false.
// Otherwise, it returns a ticker that exposes a method producing a warning if the bookmark event,
// which marks the end of the watch stream, has not been received within the defined tick interval.
//
// Note that the caller controls whether to call t.C() and t.Stop().
//
// In practice, the reflector exits the watchHandler as soon as the bookmark event is received and calls the t.C() method.
func newInitialEventsEndBookmarkTicker(logger klog.Logger, name string, c clock.Clock, watchStart time.Time, exitOnWatchListBookmarkReceived bool) *initialEventsEndBookmarkTicker {
	return newInitialEventsEndBookmarkTickerInternal(logger, name, c, watchStart, 10*time.Second, exitOnWatchListBookmarkReceived)
}

func newInitialEventsEndBookmarkTickerInternal(logger klog.Logger, name string, c clock.Clock, watchStart time.Time, tickInterval time.Duration, exitOnWatchListBookmarkReceived bool) *initialEventsEndBookmarkTicker {
	clockWithTicker, ok := c.(clock.WithTicker)
	if !ok || !exitOnWatchListBookmarkReceived {
		if exitOnWatchListBookmarkReceived {
			logger.Info("Warning: clock does not support WithTicker interface but exitOnInitialEventsEndBookmark was requested")
		}
		return &initialEventsEndBookmarkTicker{
			Ticker: &noopTicker{},
		}
	}

	return &initialEventsEndBookmarkTicker{
		Ticker:       clockWithTicker.NewTicker(tickInterval),
		clock:        c,
		name:         name,
		logger:       logger,
		watchStart:   watchStart,
		tickInterval: tickInterval,
	}
}

func (t *initialEventsEndBookmarkTicker) observeLastEventTimeStamp(lastEventObserveTime time.Time) {
	t.lastEventObserveTime = lastEventObserveTime
}

func (t *initialEventsEndBookmarkTicker) warnIfExpired() {
	if err := t.produceWarningIfExpired(); err != nil {
		t.logger.Info("Warning: event bookmark expired", "err", err)
	}
}

// produceWarningIfExpired returns an error that represents a warning when
// the time elapsed since the last received event exceeds the tickInterval.
//
// Note that this method should be called when t.C() yields a value.
func (t *initialEventsEndBookmarkTicker) produceWarningIfExpired() error {
	if _, ok := t.Ticker.(*noopTicker); ok {
		return nil /*noop ticker*/
	}
	if t.lastEventObserveTime.IsZero() {
		return fmt.Errorf("%s: awaiting required bookmark event for initial events stream, no events received for %v", t.name, t.clock.Since(t.watchStart))
	}
	elapsedTime := t.clock.Now().Sub(t.lastEventObserveTime)
	hasBookmarkTimerExpired := elapsedTime >= t.tickInterval

	if !hasBookmarkTimerExpired {
		return nil
	}
	return fmt.Errorf("%s: hasn't received required bookmark event marking the end of initial events stream, received last event %v ago", t.name, elapsedTime)
}

var _ clock.Ticker = &noopTicker{}

// TODO(#115478): move to k8s/utils repo
type noopTicker struct{}

func (t *noopTicker) C() <-chan time.Time { return nil }

func (t *noopTicker) Stop() {}

// VeryShortWatchError is returned when the watch result channel is closed
// within one second, without having sent any events.
type VeryShortWatchError struct {
	// Name of the Reflector
	Name string
}

// Error implements the error interface
func (e *VeryShortWatchError) Error() string {
	return fmt.Sprintf("very short watch: %s: Unexpected watch close - "+
		"watch lasted less than a second and no items received", e.Name)
}

var unsupportedTableGVK = map[schema.GroupVersionKind]bool{
	metav1beta1.SchemeGroupVersion.WithKind("Table"): true,
	metav1.SchemeGroupVersion.WithKind("Table"):      true,
}

// isUnsupportedTableObject checks whether the given runtime.Object
// is a "Table" object that belongs to a set of well-known unsupported GroupVersionKinds.
func isUnsupportedTableObject(rawObject runtime.Object) bool {
	unstructuredObj, ok := rawObject.(*unstructured.Unstructured)
	if !ok {
		return false
	}
	if unstructuredObj.GetKind() != "Table" {
		return false
	}

	return unsupportedTableGVK[rawObject.GetObjectKind().GroupVersionKind()]
}

func isUnsupportedTableListObject(rawObject runtime.Object) (bool, schema.GroupVersionKind) {
	unstructuredObj, ok := rawObject.(*unstructured.UnstructuredList)
	if !ok {
		return false, schema.GroupVersionKind{}
	}

	return unsupportedTableGVK[unstructuredObj.GetObjectKind().GroupVersionKind()], unstructuredObj.GetObjectKind().GroupVersionKind()
}

```

// === FILE: references!/kubernetes/staging/src/k8s.io/client-go/util/workqueue/queue.go ===
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

package workqueue

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
)

// Deprecated: Interface is deprecated, use TypedInterface instead.
type Interface TypedInterface[any]

type TypedInterface[T comparable] interface {
	Add(item T)
	Len() int
	Get() (item T, shutdown bool)
	Done(item T)
	ShutDown()
	ShutDownWithDrain()
	ShuttingDown() bool
}

// Queue is the underlying storage for items. The functions below are always
// called from the same goroutine.
type Queue[T comparable] interface {
	// Touch can be hooked when an existing item is added again. This may be
	// useful if the implementation allows priority change for the given item.
	Touch(item T)
	// Push adds a new item.
	Push(item T)
	// Len tells the total number of items.
	Len() int
	// Pop retrieves an item.
	Pop() (item T)
}

// DefaultQueue is a slice based FIFO queue.
func DefaultQueue[T comparable]() Queue[T] {
	return new(queue[T])
}

// queue is a slice which implements Queue.
type queue[T comparable] []T

func (q *queue[T]) Touch(item T) {}

func (q *queue[T]) Push(item T) {
	*q = append(*q, item)
}

func (q *queue[T]) Len() int {
	return len(*q)
}

func (q *queue[T]) Pop() (item T) {
	item = (*q)[0]

	// The underlying array still exists and reference this object, so the object will not be garbage collected.
	(*q)[0] = *new(T)
	*q = (*q)[1:]

	return item
}

// QueueConfig specifies optional configurations to customize an Interface.
// Deprecated: use TypedQueueConfig instead.
type QueueConfig = TypedQueueConfig[any]

type TypedQueueConfig[T comparable] struct {
	// Name for the queue. If unnamed, the metrics will not be registered.
	Name string

	// MetricsProvider optionally allows specifying a metrics provider to use for the queue
	// instead of the global provider.
	MetricsProvider MetricsProvider

	// Clock ability to inject real or fake clock for testing purposes.
	Clock clock.WithTicker

	// Queue provides the underlying queue to use. It is optional and defaults to slice based FIFO queue.
	Queue Queue[T]
}

// New constructs a new work queue (see the package comment).
//
// Deprecated: use NewTyped instead.
func New() *Type {
	return NewWithConfig(QueueConfig{
		Name: "",
	})
}

// NewTyped constructs a new work queue (see the package comment).
func NewTyped[T comparable]() *Typed[T] {
	return NewTypedWithConfig(TypedQueueConfig[T]{
		Name: "",
	})
}

// NewWithConfig constructs a new workqueue with ability to
// customize different properties.
//
// Deprecated: use NewTypedWithConfig instead.
func NewWithConfig(config QueueConfig) *Type {
	return NewTypedWithConfig(config)
}

// NewTypedWithConfig constructs a new workqueue with ability to
// customize different properties.
func NewTypedWithConfig[T comparable](config TypedQueueConfig[T]) *Typed[T] {
	return newQueueWithConfig(config, defaultUnfinishedWorkUpdatePeriod)
}

// NewNamed creates a new named queue.
// Deprecated: Use NewWithConfig instead.
func NewNamed(name string) *Type {
	return NewWithConfig(QueueConfig{
		Name: name,
	})
}

// newQueueWithConfig constructs a new named workqueue
// with the ability to customize different properties for testing purposes
func newQueueWithConfig[T comparable](config TypedQueueConfig[T], updatePeriod time.Duration) *Typed[T] {
	metricsProvider := globalMetricsProvider
	if config.MetricsProvider != nil {
		metricsProvider = config.MetricsProvider
	}

	if config.Clock == nil {
		config.Clock = clock.RealClock{}
	}

	if config.Queue == nil {
		config.Queue = DefaultQueue[T]()
	}

	return newQueue(
		config.Clock,
		config.Queue,
		newQueueMetrics[T](metricsProvider, config.Name, config.Clock),
		updatePeriod,
	)
}

func newQueue[T comparable](c clock.WithTicker, queue Queue[T], metrics queueMetrics[T], updatePeriod time.Duration) *Typed[T] {
	t := &Typed[T]{
		clock:                      c,
		queue:                      queue,
		dirty:                      sets.Set[T]{},
		processing:                 sets.Set[T]{},
		cond:                       sync.NewCond(&sync.Mutex{}),
		metrics:                    metrics,
		unfinishedWorkUpdatePeriod: updatePeriod,
		stopCh:                     make(chan struct{}),
	}

	// Don't start the goroutine for a type of noMetrics so we don't consume
	// resources unnecessarily
	if _, ok := metrics.(noMetrics[T]); !ok {
		t.wg.Go(t.updateUnfinishedWorkLoop)
	}

	return t
}

const defaultUnfinishedWorkUpdatePeriod = 500 * time.Millisecond

// Type is a work queue (see the package comment).
// Deprecated: Use Typed instead.
type Type = Typed[any]

type Typed[t comparable] struct {
	// queue defines the order in which we will work on items. Every
	// element of queue should be in the dirty set and not in the
	// processing set.
	queue Queue[t]

	// dirty defines all of the items that need to be processed.
	dirty sets.Set[t]

	// Things that are currently being processed are in the processing set.
	// These things may be simultaneously in the dirty set. When we finish
	// processing something and remove it from this set, we'll check if
	// it's in the dirty set, and if so, add it to the queue.
	processing sets.Set[t]

	cond *sync.Cond

	shuttingDown bool
	drain        bool

	metrics queueMetrics[t]

	unfinishedWorkUpdatePeriod time.Duration
	clock                      clock.WithTicker

	// wg manages goroutines started by the queue to allow graceful shutdown
	// ShutDown() will wait for goroutines to exit before returning.
	wg sync.WaitGroup

	stopCh chan struct{}
	// stopOnce guarantees we only signal shutdown a single time
	stopOnce sync.Once
}

// Add marks item as needing processing. When the queue is shutdown new
// items will silently be ignored and not queued or marked as dirty for
// reprocessing.
func (q *Typed[T]) Add(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.shuttingDown {
		return
	}
	if q.dirty.Has(item) {
		// the same item is added again before it is processed, call the Touch
		// function if the queue cares about it (for e.g, reset its priority)
		if !q.processing.Has(item) {
			q.queue.Touch(item)
		}
		return
	}

	q.metrics.add(item)

	q.dirty.Insert(item)
	if q.processing.Has(item) {
		return
	}

	q.queue.Push(item)
	q.cond.Signal()
}

// Len returns the current queue length, for informational purposes only. You
// shouldn't e.g. gate a call to Add() or Get() on Len() being a particular
// value, that can't be synchronized properly.
func (q *Typed[T]) Len() int {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	return q.queue.Len()
}

// Get blocks until it can return an item to be processed. If shutdown = true,
// the caller should end their goroutine. You must call Done with item when you
// have finished processing it.
func (q *Typed[T]) Get() (item T, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for q.queue.Len() == 0 && !q.shuttingDown {
		q.cond.Wait()
	}
	if q.queue.Len() == 0 {
		// We must be shutting down.
		return *new(T), true
	}

	item = q.queue.Pop()

	q.metrics.get(item)

	q.processing.Insert(item)
	q.dirty.Delete(item)

	return item, false
}

// Done marks item as done processing, and if it has been marked as dirty again
// while it was being processed, it will be re-added to the queue for
// re-processing.
func (q *Typed[T]) Done(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.metrics.done(item)

	q.processing.Delete(item)
	if q.dirty.Has(item) {
		q.queue.Push(item)
		q.cond.Signal()
	} else if q.processing.Len() == 0 {
		q.cond.Signal()
	}
}

// ShutDown will cause q to ignore all new items added to it. Worker
// goroutines will continue processing items in the queue until it is
// empty and then receive the shutdown signal.
func (q *Typed[T]) ShutDown() {
	defer q.wg.Wait()
	q.stopOnce.Do(func() {
		defer close(q.stopCh)
	})

	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.drain = false
	q.shuttingDown = true
	q.cond.Broadcast()
}

// ShutDownWithDrain is equivalent to ShutDown but waits until all items
// in the queue have been processed.
// ShutDown can be called after ShutDownWithDrain to force
// ShutDownWithDrain to stop waiting.
// Workers must call Done on an item after processing it, otherwise
// ShutDownWithDrain will block indefinitely.
func (q *Typed[T]) ShutDownWithDrain() {
	defer q.wg.Wait()
	q.stopOnce.Do(func() {
		defer close(q.stopCh)
	})
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.drain = true
	q.shuttingDown = true
	q.cond.Broadcast()

	for q.processing.Len() != 0 && q.drain {
		q.cond.Wait()
	}
}

func (q *Typed[T]) ShuttingDown() bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	return q.shuttingDown
}

func (q *Typed[T]) updateUnfinishedWork() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if !q.shuttingDown {
		q.metrics.updateUnfinishedWork()
	}
}

func (q *Typed[T]) updateUnfinishedWorkLoop() {
	t := q.clock.NewTicker(q.unfinishedWorkUpdatePeriod)
	defer t.Stop()
	for {
		select {
		case <-t.C():
			q.updateUnfinishedWork()
		case <-q.stopCh:
			return
		}
	}
}

```

