# Domain Architecture: staging/src/k8s.io/apiserver

## Layout Topology
```text
staging/src/k8s.io/apiserver/
├── pkg
│   ├── admission
│   │   ├── configuration
│   │   │   ├── configuration_manager.go
│   │   │   ├── mutating_webhook_manager.go
│   │   │   └── validating_webhook_manager.go
│   │   ├── initializer
│   │   │   ├── apiserver_id.go
│   │   │   ├── initializer.go
│   │   │   └── interfaces.go
│   │   ├── metrics
│   │   │   └── metrics.go
│   │   ├── plugin
│   │   │   ├── authorizer
│   │   │   │   └── caching_authorizer.go
│   │   │   ├── cel
│   │   │   │   ├── OWNERS
│   │   │   │   ├── activation.go
│   │   │   │   ├── compile.go
│   │   │   │   ├── composition.go
│   │   │   │   ├── condition.go
│   │   │   │   ├── interface.go
│   │   │   │   └── mutation.go
│   │   │   ├── manifest
│   │   │   │   ├── metrics
│   │   │   │   │   └── metrics.go
│   │   │   │   ├── loader.go
│   │   │   │   └── validation.go
│   │   │   ├── namespace
│   │   │   │   └── lifecycle
│   │   │   │       └── admission.go
│   │   │   ├── policy
│   │   │   │   ├── config
│   │   │   │   │   ├── apis
│   │   │   │   │   │   └── policyconfig
│   │   │   │   │   │       ├── install
│   │   │   │   │   │       │   └── install.go
│   │   │   │   │   │       ├── v1
│   │   │   │   │   │       │   ├── doc.go
│   │   │   │   │   │       │   ├── register.go
│   │   │   │   │   │       │   ├── types.go
│   │   │   │   │   │       │   ├── zz_generated.conversion.go
│   │   │   │   │   │       │   ├── zz_generated.deepcopy.go
│   │   │   │   │   │       │   └── zz_generated.model_name.go
│   │   │   │   │   │       ├── doc.go
│   │   │   │   │   │       ├── register.go
│   │   │   │   │   │       ├── types.go
│   │   │   │   │   │       └── zz_generated.deepcopy.go
│   │   │   │   │   └── config.go
│   │   │   │   ├── generic
│   │   │   │   │   ├── accessor.go
│   │   │   │   │   ├── composite_policy_source.go
│   │   │   │   │   ├── interfaces.go
│   │   │   │   │   ├── plugin.go
│   │   │   │   │   ├── policy_dispatcher.go
│   │   │   │   │   ├── policy_matcher.go
│   │   │   │   │   ├── policy_source.go
│   │   │   │   │   └── policy_test_context.go
│   │   │   │   ├── internal
│   │   │   │   │   └── generic
│   │   │   │   │       ├── controller.go
│   │   │   │   │       ├── doc.go
│   │   │   │   │       ├── informer.go
│   │   │   │   │       ├── interface.go
│   │   │   │   │       └── lister.go
│   │   │   │   ├── manifest
│   │   │   │   │   ├── loader
│   │   │   │   │   │   └── loader.go
│   │   │   │   │   └── source
│   │   │   │   │       └── source.go
│   │   │   │   ├── matching
│   │   │   │   │   └── matching.go
│   │   │   │   ├── mutating
│   │   │   │   │   ├── metrics
│   │   │   │   │   │   ├── errors.go
│   │   │   │   │   │   └── metrics.go
│   │   │   │   │   ├── patch
│   │   │   │   │   │   ├── interface.go
│   │   │   │   │   │   ├── json_patch.go
│   │   │   │   │   │   ├── smd.go
│   │   │   │   │   │   └── typeconverter.go
│   │   │   │   │   ├── accessor.go
│   │   │   │   │   ├── compilation.go
│   │   │   │   │   ├── dispatcher.go
│   │   │   │   │   ├── errors.go
│   │   │   │   │   ├── plugin.go
│   │   │   │   │   └── reinvocationcontext.go
│   │   │   │   ├── validating
│   │   │   │   │   ├── metrics
│   │   │   │   │   │   ├── errors.go
│   │   │   │   │   │   └── metrics.go
│   │   │   │   │   ├── accessor.go
│   │   │   │   │   ├── dispatcher.go
│   │   │   │   │   ├── errors.go
│   │   │   │   │   ├── initializer.go
│   │   │   │   │   ├── interface.go
│   │   │   │   │   ├── message.go
│   │   │   │   │   ├── plugin.go
│   │   │   │   │   ├── policy_decision.go
│   │   │   │   │   ├── typechecking.go
│   │   │   │   │   └── validator.go
│   │   │   │   └── OWNERS
│   │   │   ├── resourcequota
│   │   │   │   ├── apis
│   │   │   │   │   └── resourcequota
│   │   │   │   │       ├── install
│   │   │   │   │       │   └── install.go
│   │   │   │   │       ├── v1
│   │   │   │   │       │   ├── defaults.go
│   │   │   │   │       │   ├── doc.go
│   │   │   │   │       │   ├── register.go
│   │   │   │   │       │   ├── types.go
│   │   │   │   │       │   ├── zz_generated.conversion.go
│   │   │   │   │       │   ├── zz_generated.deepcopy.go
│   │   │   │   │       │   ├── zz_generated.defaults.go
│   │   │   │   │       │   └── zz_generated.model_name.go
│   │   │   │   │       ├── v1alpha1
│   │   │   │   │       │   ├── defaults.go
│   │   │   │   │       │   ├── doc.go
│   │   │   │   │       │   ├── register.go
│   │   │   │   │       │   ├── types.go
│   │   │   │   │       │   ├── zz_generated.conversion.go
│   │   │   │   │       │   ├── zz_generated.deepcopy.go
│   │   │   │   │       │   ├── zz_generated.defaults.go
│   │   │   │   │       │   └── zz_generated.model_name.go
│   │   │   │   │       ├── v1beta1
│   │   │   │   │       │   ├── defaults.go
│   │   │   │   │       │   ├── doc.go
│   │   │   │   │       │   ├── register.go
│   │   │   │   │       │   ├── types.go
│   │   │   │   │       │   ├── zz_generated.conversion.go
│   │   │   │   │       │   ├── zz_generated.deepcopy.go
│   │   │   │   │       │   ├── zz_generated.defaults.go
│   │   │   │   │       │   └── zz_generated.model_name.go
│   │   │   │   │       ├── validation
│   │   │   │   │       │   └── validation.go
│   │   │   │   │       ├── OWNERS
│   │   │   │   │       ├── doc.go
│   │   │   │   │       ├── register.go
│   │   │   │   │       ├── types.go
│   │   │   │   │       └── zz_generated.deepcopy.go
│   │   │   │   ├── admission.go
│   │   │   │   ├── config.go
│   │   │   │   ├── controller.go
│   │   │   │   ├── doc.go
│   │   │   │   └── resource_access.go
│   │   │   └── webhook
│   │   │       ├── config
│   │   │       │   ├── apis
│   │   │       │   │   └── webhookadmission
│   │   │       │   │       ├── install
│   │   │       │   │       │   └── install.go
│   │   │       │   │       ├── v1
│   │   │       │   │       │   ├── doc.go
│   │   │       │   │       │   ├── register.go
│   │   │       │   │       │   ├── types.go
│   │   │       │   │       │   ├── zz_generated.conversion.go
│   │   │       │   │       │   ├── zz_generated.deepcopy.go
│   │   │       │   │       │   ├── zz_generated.defaults.go
│   │   │       │   │       │   └── zz_generated.model_name.go
│   │   │       │   │       ├── doc.go
│   │   │       │   │       ├── register.go
│   │   │       │   │       ├── types.go
│   │   │       │   │       └── zz_generated.deepcopy.go
│   │   │       │   └── kubeconfig.go
│   │   │       ├── errors
│   │   │       │   ├── doc.go
│   │   │       │   └── statuserror.go
│   │   │       ├── generic
│   │   │       │   ├── composite_webhook_source.go
│   │   │       │   ├── interfaces.go
│   │   │       │   └── webhook.go
│   │   │       ├── initializer
│   │   │       │   └── initializer.go
│   │   │       ├── manifest
│   │   │       │   ├── loader
│   │   │       │   │   └── loader.go
│   │   │       │   └── source
│   │   │       │       └── source.go
│   │   │       ├── matchconditions
│   │   │       │   ├── interface.go
│   │   │       │   └── matcher.go
│   │   │       ├── mutating
│   │   │       │   ├── dispatcher.go
│   │   │       │   ├── doc.go
│   │   │       │   ├── plugin.go
│   │   │       │   └── reinvocationcontext.go
│   │   │       ├── predicates
│   │   │       │   ├── namespace
│   │   │       │   │   ├── doc.go
│   │   │       │   │   └── matcher.go
│   │   │       │   ├── object
│   │   │       │   │   ├── doc.go
│   │   │       │   │   └── matcher.go
│   │   │       │   └── rules
│   │   │       │       └── rules.go
│   │   │       ├── request
│   │   │       │   ├── admissionreview.go
│   │   │       │   └── doc.go
│   │   │       ├── testcerts
│   │   │       │   ├── certs.go
│   │   │       │   ├── doc.go
│   │   │       │   └── gencerts.sh
│   │   │       ├── testing
│   │   │       │   ├── main
│   │   │       │   │   └── main.go
│   │   │       │   ├── authentication_info_resolver.go
│   │   │       │   ├── service_resolver.go
│   │   │       │   ├── testcase.go
│   │   │       │   └── webhook_server.go
│   │   │       ├── util
│   │   │       │   └── client_config.go
│   │   │       ├── validating
│   │   │       │   ├── dispatcher.go
│   │   │       │   ├── doc.go
│   │   │       │   └── plugin.go
│   │   │       └── accessors.go
│   │   ├── testing
│   │   │   └── helpers.go
│   │   ├── attributes.go
│   │   ├── audit.go
│   │   ├── chain.go
│   │   ├── config.go
│   │   ├── conversion.go
│   │   ├── decorator.go
│   │   ├── errors.go
│   │   ├── handler.go
│   │   ├── interfaces.go
│   │   ├── plugins.go
│   │   ├── reinvocation.go
│   │   └── util.go
│   ├── apis
│   │   ├── apidiscovery
│   │   │   ├── v2
│   │   │   │   ├── conversion.go
│   │   │   │   ├── doc.go
│   │   │   │   └── register.go
│   │   │   └── v2beta1
│   │   │       ├── doc.go
│   │   │       └── register.go
│   │   ├── apiserver
│   │   │   ├── install
│   │   │   │   └── install.go
│   │   │   ├── load
│   │   │   │   └── load.go
│   │   │   ├── v1
│   │   │   │   ├── defaults.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── types_encryption.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   └── zz_generated.defaults.go
│   │   │   ├── v1alpha1
│   │   │   │   ├── conversion.go
│   │   │   │   ├── defaults.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   └── zz_generated.defaults.go
│   │   │   ├── v1beta1
│   │   │   │   ├── conversion.go
│   │   │   │   ├── defaults.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   └── zz_generated.defaults.go
│   │   │   ├── validation
│   │   │   │   ├── validation.go
│   │   │   │   └── validation_encryption.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   ├── types.go
│   │   │   ├── types_encryption.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── audit
│   │   │   ├── fuzzer
│   │   │   │   └── fuzzer.go
│   │   │   ├── install
│   │   │   │   └── install.go
│   │   │   ├── v1
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated.pb.go
│   │   │   │   ├── generated.proto
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   ├── zz_generated.defaults.go
│   │   │   │   └── zz_generated.model_name.go
│   │   │   ├── validation
│   │   │   │   └── validation.go
│   │   │   ├── OWNERS
│   │   │   ├── doc.go
│   │   │   ├── helpers.go
│   │   │   ├── register.go
│   │   │   ├── types.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── cel
│   │   │   └── config.go
│   │   ├── example
│   │   │   ├── fuzzer
│   │   │   │   └── fuzzer.go
│   │   │   ├── install
│   │   │   │   └── install.go
│   │   │   ├── v1
│   │   │   │   ├── conversion.go
│   │   │   │   ├── defaults.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated.pb.go
│   │   │   │   ├── generated.proto
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   ├── zz_generated.defaults.go
│   │   │   │   └── zz_generated.model_name.go
│   │   │   ├── doc.go
│   │   │   ├── register.go
│   │   │   ├── types.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── example2
│   │   │   ├── install
│   │   │   │   └── install.go
│   │   │   ├── v1
│   │   │   │   ├── conversion.go
│   │   │   │   ├── defaults.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── generated.pb.go
│   │   │   │   ├── generated.proto
│   │   │   │   ├── register.go
│   │   │   │   ├── types.go
│   │   │   │   ├── zz_generated.conversion.go
│   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   ├── zz_generated.defaults.go
│   │   │   │   └── zz_generated.model_name.go
│   │   │   ├── doc.go
│   │   │   └── register.go
│   │   ├── flowcontrol
│   │   │   └── bootstrap
│   │   │       └── default.go
│   │   └── OWNERS
│   ├── audit
│   │   ├── policy
│   │   │   ├── checker.go
│   │   │   ├── reader.go
│   │   │   └── util.go
│   │   ├── OWNERS
│   │   ├── context.go
│   │   ├── evaluator.go
│   │   ├── format.go
│   │   ├── metrics.go
│   │   ├── request.go
│   │   ├── scheme.go
│   │   ├── types.go
│   │   └── union.go
│   ├── authentication
│   │   ├── authenticator
│   │   │   ├── audagnostic.go
│   │   │   ├── audiences.go
│   │   │   └── interfaces.go
│   │   ├── authenticatorfactory
│   │   │   ├── delegating.go
│   │   │   ├── loopback.go
│   │   │   ├── metrics.go
│   │   │   └── requestheader.go
│   │   ├── cel
│   │   │   ├── compile.go
│   │   │   ├── interface.go
│   │   │   └── mapper.go
│   │   ├── group
│   │   │   ├── authenticated_group_adder.go
│   │   │   ├── group_adder.go
│   │   │   └── token_group_adder.go
│   │   ├── request
│   │   │   ├── anonymous
│   │   │   │   └── anonymous.go
│   │   │   ├── bearertoken
│   │   │   │   └── bearertoken.go
│   │   │   ├── headerrequest
│   │   │   │   ├── requestheader.go
│   │   │   │   └── requestheader_controller.go
│   │   │   ├── union
│   │   │   │   └── union.go
│   │   │   ├── websocket
│   │   │   │   └── protocol.go
│   │   │   └── x509
│   │   │       ├── OWNERS
│   │   │       ├── doc.go
│   │   │       ├── verify_options.go
│   │   │       └── x509.go
│   │   ├── serviceaccount
│   │   │   └── util.go
│   │   ├── token
│   │   │   ├── cache
│   │   │   │   ├── cache_simple.go
│   │   │   │   ├── cache_striped.go
│   │   │   │   ├── cached_token_authenticator.go
│   │   │   │   └── stats.go
│   │   │   ├── jwt
│   │   │   │   └── jwt.go
│   │   │   ├── tokenfile
│   │   │   │   └── tokenfile.go
│   │   │   └── union
│   │   │       └── union.go
│   │   ├── user
│   │   │   ├── doc.go
│   │   │   └── user.go
│   │   └── OWNERS
│   ├── authorization
│   │   ├── authorizer
│   │   │   ├── conditions.go
│   │   │   ├── interfaces.go
│   │   │   └── rule.go
│   │   ├── authorizerfactory
│   │   │   ├── builtin.go
│   │   │   ├── delegating.go
│   │   │   └── metrics.go
│   │   ├── cel
│   │   │   ├── compile.go
│   │   │   ├── interface.go
│   │   │   ├── matcher.go
│   │   │   └── metrics.go
│   │   ├── metrics
│   │   │   └── metrics.go
│   │   ├── path
│   │   │   ├── doc.go
│   │   │   └── path.go
│   │   ├── union
│   │   │   └── union.go
│   │   └── OWNERS
│   ├── cel
│   │   ├── common
│   │   │   ├── adaptor.go
│   │   │   ├── equality.go
│   │   │   ├── maplist.go
│   │   │   ├── schemas.go
│   │   │   ├── typeprovider.go
│   │   │   ├── valuesreflect.go
│   │   │   ├── valuesschemalesstyped.go
│   │   │   └── valuesunstructured.go
│   │   ├── environment
│   │   │   ├── base.go
│   │   │   └── environment.go
│   │   ├── lazy
│   │   │   └── lazy.go
│   │   ├── library
│   │   │   ├── authz.go
│   │   │   ├── cidr.go
│   │   │   ├── cost.go
│   │   │   ├── format.go
│   │   │   ├── ip.go
│   │   │   ├── jsonpatch.go
│   │   │   ├── libraries.go
│   │   │   ├── lists.go
│   │   │   ├── quantity.go
│   │   │   ├── regex.go
│   │   │   ├── semverlib.go
│   │   │   ├── test.go
│   │   │   └── urls.go
│   │   ├── metrics
│   │   │   └── metrics.go
│   │   ├── mutation
│   │   │   ├── dynamic
│   │   │   │   └── objects.go
│   │   │   ├── jsonpatch.go
│   │   │   └── typeresolver.go
│   │   ├── openapi
│   │   │   ├── resolver
│   │   │   │   ├── combined.go
│   │   │   │   ├── definitions.go
│   │   │   │   ├── discovery.go
│   │   │   │   ├── refs.go
│   │   │   │   └── resolver.go
│   │   │   ├── adaptor.go
│   │   │   └── extensions.go
│   │   ├── OWNERS
│   │   ├── cidr.go
│   │   ├── errors.go
│   │   ├── escaping.go
│   │   ├── format.go
│   │   ├── ip.go
│   │   ├── limits.go
│   │   ├── quantity.go
│   │   ├── semver.go
│   │   ├── types.go
│   │   ├── url.go
│   │   └── value.go
│   ├── endpoints
│   │   ├── deprecation
│   │   │   └── deprecation.go
│   │   ├── discovery
│   │   │   ├── aggregated
│   │   │   │   ├── etag.go
│   │   │   │   ├── fake.go
│   │   │   │   ├── handler.go
│   │   │   │   ├── metrics.go
│   │   │   │   ├── negotiation.go
│   │   │   │   ├── peer_aggregated_handler.go
│   │   │   │   └── wrapper.go
│   │   │   ├── OWNERS
│   │   │   ├── addresses.go
│   │   │   ├── group.go
│   │   │   ├── legacy.go
│   │   │   ├── root.go
│   │   │   ├── storageversionhash.go
│   │   │   ├── util.go
│   │   │   └── version.go
│   │   ├── filterlatency
│   │   │   └── filterlatency.go
│   │   ├── filters
│   │   │   ├── impersonation
│   │   │   │   ├── metrics
│   │   │   │   │   └── metrics.go
│   │   │   │   ├── OWNERS
│   │   │   │   ├── cache.go
│   │   │   │   ├── constrained_impersonation.go
│   │   │   │   ├── impersonation.go
│   │   │   │   └── mode.go
│   │   │   ├── OWNERS
│   │   │   ├── audit.go
│   │   │   ├── audit_init.go
│   │   │   ├── authentication.go
│   │   │   ├── authn_audit.go
│   │   │   ├── authorization.go
│   │   │   ├── cachecontrol.go
│   │   │   ├── doc.go
│   │   │   ├── metrics.go
│   │   │   ├── mux_discovery_complete.go
│   │   │   ├── request_deadline.go
│   │   │   ├── request_received_time.go
│   │   │   ├── requestinfo.go
│   │   │   ├── storageversion.go
│   │   │   ├── traces.go
│   │   │   ├── warning.go
│   │   │   └── webhook_duration.go
│   │   ├── handlers
│   │   │   ├── fieldmanager
│   │   │   │   ├── OWNERS
│   │   │   │   ├── admission.go
│   │   │   │   └── equality.go
│   │   │   ├── finisher
│   │   │   │   └── finisher.go
│   │   │   ├── metrics
│   │   │   │   ├── OWNERS
│   │   │   │   └── metrics.go
│   │   │   ├── negotiation
│   │   │   │   ├── doc.go
│   │   │   │   ├── errors.go
│   │   │   │   └── negotiate.go
│   │   │   ├── responsewriters
│   │   │   │   ├── compression.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── errors.go
│   │   │   │   ├── status.go
│   │   │   │   └── writers.go
│   │   │   ├── create.go
│   │   │   ├── delete.go
│   │   │   ├── doc.go
│   │   │   ├── get.go
│   │   │   ├── helpers.go
│   │   │   ├── namer.go
│   │   │   ├── patch.go
│   │   │   ├── response.go
│   │   │   ├── rest.go
│   │   │   ├── trace_util.go
│   │   │   ├── update.go
│   │   │   └── watch.go
│   │   ├── metrics
│   │   │   ├── OWNERS
│   │   │   └── metrics.go
│   │   ├── openapi
│   │   │   ├── testing
│   │   │   │   ├── types.go
│   │   │   │   └── zz_generated.deepcopy.go
│   │   │   └── openapi.go
│   │   ├── request
│   │   │   ├── OWNERS
│   │   │   ├── context.go
│   │   │   ├── doc.go
│   │   │   ├── methods.go
│   │   │   ├── received_time.go
│   │   │   ├── requestinfo.go
│   │   │   ├── server_shutdown_signal.go
│   │   │   └── webhook_duration.go
│   │   ├── responsewriter
│   │   │   ├── fake.go
│   │   │   └── wrapper.go
│   │   ├── testing
│   │   │   ├── OWNERS
│   │   │   ├── conversion.go
│   │   │   ├── doc.go
│   │   │   ├── types.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── warning
│   │   │   └── warning.go
│   │   ├── OWNERS
│   │   ├── doc.go
│   │   ├── groupversion.go
│   │   └── installer.go
│   ├── features
│   │   ├── OWNERS
│   │   └── kube_features.go
│   ├── quota
│   │   └── v1
│   │       ├── generic
│   │       │   ├── OWNERS
│   │       │   ├── configuration.go
│   │       │   ├── evaluator.go
│   │       │   └── registry.go
│   │       ├── OWNERS
│   │       ├── interfaces.go
│   │       └── resources.go
│   ├── reconcilers
│   │   └── peer_endpoint_lease.go
│   ├── registry
│   │   ├── generic
│   │   │   ├── registry
│   │   │   │   ├── corrupt_obj_deleter.go
│   │   │   │   ├── decorated_watcher.go
│   │   │   │   ├── doc.go
│   │   │   │   ├── dryrun.go
│   │   │   │   ├── storage_factory.go
│   │   │   │   └── store.go
│   │   │   ├── rest
│   │   │   │   ├── doc.go
│   │   │   │   ├── response_checker.go
│   │   │   │   └── streamer.go
│   │   │   ├── testing
│   │   │   │   └── tester.go
│   │   │   ├── OWNERS
│   │   │   ├── doc.go
│   │   │   ├── matcher.go
│   │   │   ├── options.go
│   │   │   └── storage_decorator.go
│   │   ├── rest
│   │   │   ├── resttest
│   │   │   │   └── resttest.go
│   │   │   ├── OWNERS
│   │   │   ├── create.go
│   │   │   ├── create_update.go
│   │   │   ├── delete.go
│   │   │   ├── doc.go
│   │   │   ├── meta.go
│   │   │   ├── rest.go
│   │   │   ├── table.go
│   │   │   ├── update.go
│   │   │   └── validate.go
│   │   └── doc.go
│   ├── server
│   │   ├── dynamiccertificates
│   │   │   ├── cert_key.go
│   │   │   ├── client_ca.go
│   │   │   ├── configmap_cafile_content.go
│   │   │   ├── dynamic_cafile_content.go
│   │   │   ├── dynamic_serving_content.go
│   │   │   ├── dynamic_sni_content.go
│   │   │   ├── interfaces.go
│   │   │   ├── named_certificates.go
│   │   │   ├── static_content.go
│   │   │   ├── tlsconfig.go
│   │   │   ├── union_content.go
│   │   │   └── util.go
│   │   ├── egressselector
│   │   │   ├── metrics
│   │   │   │   └── metrics.go
│   │   │   ├── config.go
│   │   │   └── egress_selector.go
│   │   ├── filters
│   │   │   ├── OWNERS
│   │   │   ├── content_type.go
│   │   │   ├── cors.go
│   │   │   ├── doc.go
│   │   │   ├── goaway.go
│   │   │   ├── hsts.go
│   │   │   ├── longrunning.go
│   │   │   ├── maxinflight.go
│   │   │   ├── priority-and-fairness.go
│   │   │   ├── timeout.go
│   │   │   ├── waitgroup.go
│   │   │   ├── watch_termination.go
│   │   │   ├── with_retry_after.go
│   │   │   └── wrap.go
│   │   ├── flagz
│   │   │   ├── api
│   │   │   │   ├── v1alpha1
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── register.go
│   │   │   │   │   ├── types.go
│   │   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   │   └── zz_generated.model_name.go
│   │   │   │   ├── v1beta1
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── register.go
│   │   │   │   │   ├── types.go
│   │   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   │   └── zz_generated.model_name.go
│   │   │   │   └── OWNERS
│   │   │   ├── negotiate
│   │   │   │   └── negotiation.go
│   │   │   ├── testing
│   │   │   │   └── testing.go
│   │   │   ├── OWNERS
│   │   │   ├── flagreader.go
│   │   │   ├── flagz.go
│   │   │   ├── registry.go
│   │   │   └── textserializer.go
│   │   ├── healthz
│   │   │   ├── doc.go
│   │   │   └── healthz.go
│   │   ├── httplog
│   │   │   ├── doc.go
│   │   │   └── httplog.go
│   │   ├── mux
│   │   │   ├── OWNERS
│   │   │   ├── doc.go
│   │   │   └── pathrecorder.go
│   │   ├── options
│   │   │   ├── authenticationconfig
│   │   │   │   └── metrics
│   │   │   │       └── metrics.go
│   │   │   ├── authorizationconfig
│   │   │   │   └── metrics
│   │   │   │       └── metrics.go
│   │   │   ├── encryptionconfig
│   │   │   │   ├── controller
│   │   │   │   │   └── controller.go
│   │   │   │   ├── metrics
│   │   │   │   │   └── metrics.go
│   │   │   │   ├── OWNERS
│   │   │   │   └── config.go
│   │   │   ├── OWNERS
│   │   │   ├── admission.go
│   │   │   ├── api_enablement.go
│   │   │   ├── audit.go
│   │   │   ├── authentication.go
│   │   │   ├── authentication_dynamic_request_header.go
│   │   │   ├── authorization.go
│   │   │   ├── coreapi.go
│   │   │   ├── doc.go
│   │   │   ├── egress_selector.go
│   │   │   ├── etcd.go
│   │   │   ├── feature.go
│   │   │   ├── recommended.go
│   │   │   ├── server_run_options.go
│   │   │   ├── serving.go
│   │   │   ├── serving_unix.go
│   │   │   ├── serving_windows.go
│   │   │   ├── serving_with_loopback.go
│   │   │   └── tracing.go
│   │   ├── resourceconfig
│   │   │   ├── doc.go
│   │   │   └── helpers.go
│   │   ├── routes
│   │   │   ├── OWNERS
│   │   │   ├── debugsocket.go
│   │   │   ├── doc.go
│   │   │   ├── flags.go
│   │   │   ├── index.go
│   │   │   ├── metrics.go
│   │   │   ├── openapi.go
│   │   │   ├── profiling.go
│   │   │   └── version.go
│   │   ├── routine
│   │   │   └── routine.go
│   │   ├── statusz
│   │   │   ├── api
│   │   │   │   ├── v1alpha1
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── register.go
│   │   │   │   │   ├── types.go
│   │   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   │   └── zz_generated.model_name.go
│   │   │   │   ├── v1beta1
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── register.go
│   │   │   │   │   ├── types.go
│   │   │   │   │   ├── zz_generated.deepcopy.go
│   │   │   │   │   └── zz_generated.model_name.go
│   │   │   │   └── OWNERS
│   │   │   ├── negotiate
│   │   │   │   └── negotiation.go
│   │   │   ├── testing
│   │   │   │   └── testing.go
│   │   │   ├── OWNERS
│   │   │   ├── registry.go
│   │   │   ├── statusz.go
│   │   │   └── textserializer.go
│   │   ├── storage
│   │   │   ├── doc.go
│   │   │   ├── resource_config.go
│   │   │   ├── resource_encoding_config.go
│   │   │   ├── storage_codec.go
│   │   │   └── storage_factory.go
│   │   ├── config.go
│   │   ├── config_selfclient.go
│   │   ├── deleted_kinds.go
│   │   ├── deprecated_insecure_serving.go
│   │   ├── doc.go
│   │   ├── genericapiserver.go
│   │   ├── handler.go
│   │   ├── healthz.go
│   │   ├── hooks.go
│   │   ├── lifecycle_signals.go
│   │   ├── plugins.go
│   │   ├── secure_serving.go
│   │   ├── signal.go
│   │   ├── signal_posix.go
│   │   ├── signal_windows.go
│   │   └── storage_readiness_hook.go
│   ├── sharding
│   │   └── parser.go
│   ├── storage
│   │   ├── cacher
│   │   │   ├── consistency
│   │   │   │   └── checker.go
│   │   │   ├── delegator
│   │   │   │   └── interface.go
│   │   │   ├── key
│   │   │   │   └── key.go
│   │   │   ├── metrics
│   │   │   │   ├── OWNERS
│   │   │   │   └── metrics.go
│   │   │   ├── progress
│   │   │   │   └── watch_progress.go
│   │   │   ├── store
│   │   │   │   ├── store.go
│   │   │   │   ├── store_btree.go
│   │   │   │   └── watch_cache_storage.go
│   │   │   ├── testing
│   │   │   │   └── mock.go
│   │   │   ├── cache_watcher.go
│   │   │   ├── cacher.go
│   │   │   ├── caching_object.go
│   │   │   ├── compactor.go
│   │   │   ├── delegator.go
│   │   │   ├── lister_watcher.go
│   │   │   ├── ready.go
│   │   │   ├── time_budget.go
│   │   │   ├── util.go
│   │   │   ├── watch_cache.go
│   │   │   ├── watch_cache_history.go
│   │   │   └── watch_cache_interval.go
│   │   ├── errors
│   │   │   ├── doc.go
│   │   │   └── storage.go
│   │   ├── etcd3
│   │   │   ├── metrics
│   │   │   │   ├── OWNERS
│   │   │   │   └── metrics.go
│   │   │   ├── preflight
│   │   │   │   └── checks.go
│   │   │   ├── testing
│   │   │   │   ├── testingcert
│   │   │   │   │   └── certificates.go
│   │   │   │   ├── test_server.go
│   │   │   │   └── utils.go
│   │   │   ├── testserver
│   │   │   │   └── test_server.go
│   │   │   ├── OWNERS
│   │   │   ├── block_logger.go
│   │   │   ├── compact.go
│   │   │   ├── corrupt_obj_deleter.go
│   │   │   ├── decoder.go
│   │   │   ├── errors.go
│   │   │   ├── event.go
│   │   │   ├── healthcheck.go
│   │   │   ├── latency_tracker.go
│   │   │   ├── lease_manager.go
│   │   │   ├── logger.go
│   │   │   ├── stats.go
│   │   │   ├── store.go
│   │   │   └── watcher.go
│   │   ├── feature
│   │   │   └── feature_support_checker.go
│   │   ├── metrics
│   │   │   ├── OWNERS
│   │   │   └── metrics.go
│   │   ├── names
│   │   │   └── generate.go
│   │   ├── storagebackend
│   │   │   ├── factory
│   │   │   │   ├── etcd3.go
│   │   │   │   └── factory.go
│   │   │   ├── OWNERS
│   │   │   └── config.go
│   │   ├── testing
│   │   │   ├── OWNERS
│   │   │   ├── recorder.go
│   │   │   ├── store_benchmarks.go
│   │   │   ├── store_tests.go
│   │   │   ├── utils.go
│   │   │   └── watcher_tests.go
│   │   ├── testresource
│   │   │   ├── doc.go
│   │   │   ├── types.go
│   │   │   └── zz_generated.deepcopy.go
│   │   ├── value
│   │   │   ├── encrypt
│   │   │   │   ├── aes
│   │   │   │   │   ├── aes.go
│   │   │   │   │   ├── aes_extended_nonce.go
│   │   │   │   │   └── cache.go
│   │   │   │   ├── envelope
│   │   │   │   │   ├── kmsv2
│   │   │   │   │   │   ├── v2
│   │   │   │   │   │   │   ├── OWNERS
│   │   │   │   │   │   │   ├── api.pb.go
│   │   │   │   │   │   │   ├── api.proto
│   │   │   │   │   │   │   └── v2.go
│   │   │   │   │   │   ├── cache.go
│   │   │   │   │   │   ├── envelope.go
│   │   │   │   │   │   └── grpc_service.go
│   │   │   │   │   ├── metrics
│   │   │   │   │   │   └── metrics.go
│   │   │   │   │   ├── testing
│   │   │   │   │   │   ├── v1beta1
│   │   │   │   │   │   │   └── kms_plugin_mock.go
│   │   │   │   │   │   └── v2
│   │   │   │   │   │       └── kms_plugin_mock.go
│   │   │   │   │   ├── envelope.go
│   │   │   │   │   └── grpc_service.go
│   │   │   │   ├── identity
│   │   │   │   │   └── identity.go
│   │   │   │   └── secretbox
│   │   │   │       └── secretbox.go
│   │   │   ├── OWNERS
│   │   │   ├── metrics.go
│   │   │   └── transformer.go
│   │   ├── OWNERS
│   │   ├── api_object_versioner.go
│   │   ├── continue.go
│   │   ├── doc.go
│   │   ├── errors.go
│   │   ├── interfaces.go
│   │   ├── selection_predicate.go
│   │   └── util.go
│   ├── storageversion
│   │   ├── OWNERS
│   │   ├── manager.go
│   │   └── updater.go
│   ├── util
│   │   ├── apihelpers
│   │   │   └── helpers.go
│   │   ├── compatibility
│   │   │   ├── registry.go
│   │   │   └── version.go
│   │   ├── configmetrics
│   │   │   └── info_collector.go
│   │   ├── dryrun
│   │   │   └── dryrun.go
│   │   ├── feature
│   │   │   └── feature_gate.go
│   │   ├── filesystem
│   │   │   └── watchuntil.go
│   │   ├── flowcontrol
│   │   │   ├── counter
│   │   │   │   └── interface.go
│   │   │   ├── debug
│   │   │   │   └── dump.go
│   │   │   ├── fairqueuing
│   │   │   │   ├── eventclock
│   │   │   │   │   ├── interface.go
│   │   │   │   │   └── real.go
│   │   │   │   ├── promise
│   │   │   │   │   ├── interface.go
│   │   │   │   │   └── promise.go
│   │   │   │   ├── queueset
│   │   │   │   │   ├── doc.go
│   │   │   │   │   ├── fifo_list.go
│   │   │   │   │   ├── queueset.go
│   │   │   │   │   └── types.go
│   │   │   │   ├── testing
│   │   │   │   │   ├── eventclock
│   │   │   │   │   │   └── fake.go
│   │   │   │   │   ├── promise
│   │   │   │   │   │   └── counting.go
│   │   │   │   │   └── no-restraint.go
│   │   │   │   ├── integrator.go
│   │   │   │   └── interface.go
│   │   │   ├── format
│   │   │   │   └── formatting.go
│   │   │   ├── metrics
│   │   │   │   ├── interface.go
│   │   │   │   ├── metrics.go
│   │   │   │   ├── timing_ratio_histogram.go
│   │   │   │   ├── union_gauge.go
│   │   │   │   └── vec_element_pair.go
│   │   │   ├── request
│   │   │   │   ├── config.go
│   │   │   │   ├── list_work_estimator.go
│   │   │   │   ├── mutating_work_estimator.go
│   │   │   │   ├── object_count_tracker.go
│   │   │   │   ├── seat_seconds.go
│   │   │   │   └── width.go
│   │   │   ├── OWNERS
│   │   │   ├── apf_context.go
│   │   │   ├── apf_controller.go
│   │   │   ├── apf_controller_debug.go
│   │   │   ├── apf_filter.go
│   │   │   ├── conc_alloc.go
│   │   │   ├── dropped_requests_tracker.go
│   │   │   ├── formatting.go
│   │   │   ├── max_seats.go
│   │   │   ├── rule.go
│   │   │   └── watch_tracker.go
│   │   ├── flushwriter
│   │   │   ├── doc.go
│   │   │   └── writer.go
│   │   ├── notfoundhandler
│   │   │   └── not_found_handler.go
│   │   ├── openapi
│   │   │   ├── enablement.go
│   │   │   └── proto.go
│   │   ├── peerproxy
│   │   │   ├── metrics
│   │   │   │   └── metrics.go
│   │   │   ├── gv_exclusion_manager.go
│   │   │   ├── local_discovery.go
│   │   │   ├── peer_discovery.go
│   │   │   ├── peerproxy.go
│   │   │   └── peerproxy_handler.go
│   │   ├── proxy
│   │   │   ├── metrics
│   │   │   │   └── metrics.go
│   │   │   ├── doc.go
│   │   │   ├── endpointslice.go
│   │   │   ├── proxy.go
│   │   │   ├── streamtranslator.go
│   │   │   ├── streamtunnel.go
│   │   │   ├── translatinghandler.go
│   │   │   └── websocket.go
│   │   ├── responsewriter
│   │   │   └── inmemoryresponsewriter.go
│   │   ├── shufflesharding
│   │   │   └── shufflesharding.go
│   │   ├── webhook
│   │   │   ├── authentication.go
│   │   │   ├── client.go
│   │   │   ├── error.go
│   │   │   ├── gencerts.sh
│   │   │   ├── metrics.go
│   │   │   ├── serviceresolver.go
│   │   │   ├── validation.go
│   │   │   └── webhook.go
│   │   └── x509metrics
│   │       └── server_cert_deprecations.go
│   ├── validation
│   │   └── metrics.go
│   └── warning
│       └── context.go
├── plugin
│   └── pkg
│       ├── audit
│       │   ├── buffered
│       │   │   ├── buffered.go
│       │   │   └── doc.go
│       │   ├── fake
│       │   │   ├── doc.go
│       │   │   └── fake.go
│       │   ├── log
│       │   │   └── backend.go
│       │   ├── truncate
│       │   │   ├── doc.go
│       │   │   └── truncate.go
│       │   ├── webhook
│       │   │   └── webhook.go
│       │   ├── OWNERS
│       │   └── doc.go
│       ├── authenticator
│       │   ├── token
│       │   │   ├── oidc
│       │   │   │   ├── metrics.go
│       │   │   │   └── oidc.go
│       │   │   ├── tokentest
│       │   │   │   └── tokentest.go
│       │   │   └── webhook
│       │   │       ├── metrics.go
│       │   │       └── webhook.go
│       │   ├── OWNERS
│       │   └── doc.go
│       └── authorizer
│           ├── webhook
│           │   ├── metrics
│           │   │   └── metrics.go
│           │   ├── gencerts.sh
│           │   └── webhook.go
│           └── OWNERS
├── .import-restrictions
├── ARCHITECTURE.md
├── CONTRIBUTING.md
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

// === FILE: references!/kubernetes/staging/src/k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/admission.go ===
```go
/*
Copyright 2021 The Kubernetes Authors.

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

package fieldmanager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/warning"
)

// InvalidManagedFieldsAfterMutatingAdmissionWarningFormat is the warning that a client receives
// when a create/update/patch request results in invalid managedFields after going through the admission chain.
const InvalidManagedFieldsAfterMutatingAdmissionWarningFormat = ".metadata.managedFields was in an invalid state after admission; this could be caused by an outdated mutating admission controller; please fix your requests: %v"

// NewManagedFieldsValidatingAdmissionController validates the managedFields after calling
// the provided admission and resets them to their original state if they got changed to an invalid value
func NewManagedFieldsValidatingAdmissionController(wrap admission.Interface) admission.Interface {
	if wrap == nil {
		return nil
	}
	return &managedFieldsValidatingAdmissionController{wrap: wrap}
}

type managedFieldsValidatingAdmissionController struct {
	wrap admission.Interface
}

var _ admission.Interface = &managedFieldsValidatingAdmissionController{}
var _ admission.MutationInterface = &managedFieldsValidatingAdmissionController{}
var _ admission.ValidationInterface = &managedFieldsValidatingAdmissionController{}

// Handles calls the wrapped admission.Interface if applicable
func (admit *managedFieldsValidatingAdmissionController) Handles(operation admission.Operation) bool {
	return admit.wrap.Handles(operation)
}

// Admit calls the wrapped admission.Interface if applicable and resets the managedFields to their state before admission if they
// got modified in an invalid way
func (admit *managedFieldsValidatingAdmissionController) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) (err error) {
	mutationInterface, isMutationInterface := admit.wrap.(admission.MutationInterface)
	if !isMutationInterface {
		return nil
	}
	objectMeta, err := meta.Accessor(a.GetObject())
	if err != nil {
		// the object we are dealing with doesn't have object metadata defined
		// in that case we don't have to keep track of the managedField
		// just call the wrapped admission
		return mutationInterface.Admit(ctx, a, o)
	}
	managedFieldsBeforeAdmission := objectMeta.GetManagedFields()
	if err := mutationInterface.Admit(ctx, a, o); err != nil {
		return err
	}
	managedFieldsAfterAdmission := objectMeta.GetManagedFields()
	if err := managedfields.ValidateManagedFields(managedFieldsAfterAdmission); err != nil {
		objectMeta.SetManagedFields(managedFieldsBeforeAdmission)
		warning.AddWarning(ctx, "",
			fmt.Sprintf(InvalidManagedFieldsAfterMutatingAdmissionWarningFormat,
				err.Error()),
		)
	}
	return nil
}

// Validate calls the wrapped admission.Interface if aplicable
func (admit *managedFieldsValidatingAdmissionController) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) (err error) {
	if validationInterface, isValidationInterface := admit.wrap.(admission.ValidationInterface); isValidationInterface {
		return validationInterface.Validate(ctx, a, o)
	}
	return nil
}

```

// === FILE: references!/kubernetes/staging/src/k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/equality.go ===
```go
/*
Copyright 2021 The Kubernetes Authors.

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

package fieldmanager

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/klog/v2"
)

var (
	avoidTimestampEqualities     conversion.Equalities
	initAvoidTimestampEqualities sync.Once
)

func getAvoidTimestampEqualities() conversion.Equalities {
	initAvoidTimestampEqualities.Do(func() {
		if avoidNoopTimestampUpdatesString, exists := os.LookupEnv("KUBE_APISERVER_AVOID_NOOP_SSA_TIMESTAMP_UPDATES"); exists {
			if ret, err := strconv.ParseBool(avoidNoopTimestampUpdatesString); err == nil && !ret {
				// leave avoidTimestampEqualities empty.
				return
			} else {
				klog.Errorf("failed to parse envar KUBE_APISERVER_AVOID_NOOP_SSA_TIMESTAMP_UPDATES: %v", err)
			}
		}

		var eqs = equality.Semantic.Copy()
		err := eqs.AddFuncs(
			func(a, b metav1.ManagedFieldsEntry) bool {
				// Two objects' managed fields are equivalent if, ignoring timestamp,
				//	the objects are deeply equal.
				a.Time = nil
				b.Time = nil
				return reflect.DeepEqual(a, b)
			},
			func(a, b unstructured.Unstructured) bool {
				// Check if the managed fields are equal by converting to structured types and leveraging the above
				// function, then, ignoring the managed fields, equality check the rest of the unstructured data.
				if !avoidTimestampEqualities.DeepEqual(a.GetManagedFields(), b.GetManagedFields()) {
					return false
				}
				return equalIgnoringValueAtPath(a.Object, b.Object, []string{"metadata", "managedFields"})
			},
		)

		if err != nil {
			panic(fmt.Errorf("failed to instantiate semantic equalities: %w", err))
		}

		avoidTimestampEqualities = eqs
	})
	return avoidTimestampEqualities
}

func equalIgnoringValueAtPath(a, b any, path []string) bool {
	if len(path) == 0 { // found the value to ignore
		return true
	}
	aMap, aOk := a.(map[string]any)
	bMap, bOk := b.(map[string]any)
	if !aOk || !bOk {
		// Can't traverse into non-maps, ignore
		return true
	}
	if len(aMap) != len(bMap) {
		return false
	}
	pathHead := path[0]
	for k, aVal := range aMap {
		bVal, ok := bMap[k]
		if !ok {
			return false
		}
		if k == pathHead {
			if !equalIgnoringValueAtPath(aVal, bVal, path[1:]) {
				return false
			}
		} else if !avoidTimestampEqualities.DeepEqual(aVal, bVal) {
			return false
		}
	}
	return true
}

// IgnoreManagedFieldsTimestampsTransformer reverts timestamp updates
// if the non-managed parts of the object are equivalent
func IgnoreManagedFieldsTimestampsTransformer(
	_ context.Context,
	newObj runtime.Object,
	oldObj runtime.Object,
) (res runtime.Object, err error) {
	equalities := getAvoidTimestampEqualities()
	if len(equalities.Equalities) == 0 {
		return newObj, nil
	}

	outcome := "unequal_objects_fast"
	start := time.Now()
	err = nil
	res = nil

	defer func() {
		if err != nil {
			outcome = "error"
		}

		metrics.RecordTimestampComparisonLatency(outcome, time.Since(start))
	}()

	// If managedFields modulo timestamps are unchanged
	//		and
	//	rest of object is unchanged
	//		then
	//	revert any changes to timestamps in managed fields
	//		(to prevent spurious ResourceVersion bump)
	//
	// Procecure:
	// Do a quicker check to see if just managed fields modulo timestamps are
	//	unchanged. If so, then do the full, slower check.
	//
	// In most cases which actually update the object, the managed fields modulo
	//	timestamp check will fail, and we will be able to return early.
	//
	// In other cases, the managed fields may be exactly the same,
	// 	except for timestamp, but the objects are the different. This is the
	//	slow path which checks the full object.
	oldAccessor, err := meta.Accessor(oldObj)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire accessor for oldObj: %v", err)
	}

	accessor, err := meta.Accessor(newObj)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire accessor for newObj: %v", err)
	}

	oldManagedFields := oldAccessor.GetManagedFields()
	newManagedFields := accessor.GetManagedFields()

	if len(oldManagedFields) != len(newManagedFields) {
		// Return early if any managed fields entry was added/removed.
		// We want to retain user expectation that even if they write to a field
		// whose value did not change, they will still result as the field
		// manager at the end.
		return newObj, nil
	} else if len(newManagedFields) == 0 {
		// This transformation only makes sense when managedFields are
		// non-empty
		return newObj, nil
	}

	// This transformation only makes sense if the managed fields has at least one
	// changed timestamp; and are otherwise equal. Return early if there are no
	// changed timestamps.
	allTimesUnchanged := true
	for i, e := range newManagedFields {
		if !e.Time.Equal(oldManagedFields[i].Time) {
			allTimesUnchanged = false
			break
		}
	}

	if allTimesUnchanged {
		return newObj, nil
	}

	eqFn := equalities.DeepEqual
	if _, ok := newObj.(*unstructured.Unstructured); ok {
		// Use strict equality with unstructured
		eqFn = equalities.DeepEqualWithNilDifferentFromEmpty
	}

	// This condition ensures the managed fields are always compared first. If
	//	this check fails, the if statement will short circuit. If the check
	// 	succeeds the slow path is taken which compares entire objects.
	if !eqFn(oldManagedFields, newManagedFields) {
		return newObj, nil
	}

	if eqFn(newObj, oldObj) {
		// Remove any changed timestamps, so that timestamp is not the only
		// change seen by etcd.
		//
		// newManagedFields is known to be exactly pairwise equal to
		// oldManagedFields except for timestamps.
		//
		// Simply replace possibly changed new timestamps with their old values.
		for idx := 0; idx < len(oldManagedFields); idx++ {
			newManagedFields[idx].Time = oldManagedFields[idx].Time
		}

		accessor.SetManagedFields(newManagedFields)
		outcome = "equal_objects"
		return newObj, nil
	}

	outcome = "unequal_objects_slow"
	return newObj, nil
}

```

// === FILE: references!/kubernetes/staging/src/k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/OWNERS ===
```text
approvers:
  - apelisse
reviewers:
  - kwiesmueller
emeritus_approvers:
  - jennybuckley

```

// === FILE: references!/kubernetes/staging/src/k8s.io/apiserver/pkg/server/genericapiserver.go ===
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

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	gpath "path"
	"strings"
	"sync"
	"time"

	systemd "github.com/coreos/go-systemd/v22/daemon"

	"golang.org/x/time/rate"
	apidiscoveryv2 "k8s.io/api/apidiscovery/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/cbor"
	"k8s.io/apimachinery/pkg/util/managedfields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	utilwaitgroup "k8s.io/apimachinery/pkg/util/waitgroup"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapi "k8s.io/apiserver/pkg/endpoints"
	"k8s.io/apiserver/pkg/endpoints/discovery"
	discoveryendpoint "k8s.io/apiserver/pkg/endpoints/discovery/aggregated"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server/flagz"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/routes"
	"k8s.io/apiserver/pkg/server/statusz"
	"k8s.io/apiserver/pkg/storageversion"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	restclient "k8s.io/client-go/rest"
	basecompatibility "k8s.io/component-base/compatibility"
	"k8s.io/component-base/featuregate"
	zpagesfeatures "k8s.io/component-base/zpages/features"
	"k8s.io/klog/v2"
	openapibuilder3 "k8s.io/kube-openapi/pkg/builder3"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/handler"
	"k8s.io/kube-openapi/pkg/handler3"
	openapiutil "k8s.io/kube-openapi/pkg/util"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// Info about an API group.
type APIGroupInfo struct {
	PrioritizedVersions []schema.GroupVersion
	// Info about the resources in this group. It's a map from version to resource to the storage.
	VersionedResourcesStorageMap map[string]map[string]rest.Storage
	// OptionsExternalVersion controls the APIVersion used for common objects in the
	// schema like api.Status, api.DeleteOptions, and metav1.ListOptions. Other implementors may
	// define a version "v1beta1" but want to use the Kubernetes "v1" internal objects.
	// If nil, defaults to groupMeta.GroupVersion.
	// TODO: Remove this when https://github.com/kubernetes/kubernetes/issues/19018 is fixed.
	OptionsExternalVersion *schema.GroupVersion
	// MetaGroupVersion defaults to "meta.k8s.io/v1" and is the scheme group version used to decode
	// common API implementations like ListOptions. Future changes will allow this to vary by group
	// version (for when the inevitable meta/v2 group emerges).
	MetaGroupVersion *schema.GroupVersion

	// Scheme includes all of the types used by this group and how to convert between them (or
	// to convert objects from outside of this group that are accepted in this API).
	// TODO: replace with interfaces
	Scheme *runtime.Scheme
	// NegotiatedSerializer controls how this group encodes and decodes data
	NegotiatedSerializer runtime.NegotiatedSerializer
	// ParameterCodec performs conversions for query parameters passed to API calls
	ParameterCodec runtime.ParameterCodec

	// StaticOpenAPISpec is the spec derived from the definitions of all resources installed together.
	// It is set during InstallAPIGroups, InstallAPIGroup, and InstallLegacyAPIGroup.
	StaticOpenAPISpec map[string]*spec.Schema
}

func (a *APIGroupInfo) destroyStorage() {
	for _, stores := range a.VersionedResourcesStorageMap {
		for _, store := range stores {
			store.Destroy()
		}
	}
}

// GenericAPIServer contains state for a Kubernetes cluster api server.
type GenericAPIServer struct {
	// discoveryAddresses is used to build cluster IPs for discovery.
	discoveryAddresses discovery.Addresses

	// LoopbackClientConfig is a config for a privileged loopback connection to the API server
	LoopbackClientConfig *restclient.Config

	// Flagz is used to set up flagz endpoint.
	Flagz flagz.Reader

	// minRequestTimeout is how short the request timeout can be.  This is used to build the RESTHandler
	minRequestTimeout time.Duration

	// ShutdownTimeout is the timeout used for server shutdown. This specifies the timeout before server
	// gracefully shutdown returns.
	ShutdownTimeout time.Duration

	// legacyAPIGroupPrefixes is used to set up URL parsing for authorization and for validating requests
	// to InstallLegacyAPIGroup
	legacyAPIGroupPrefixes sets.String

	// admissionControl is used to build the RESTStorage that backs an API Group.
	admissionControl admission.Interface

	// SecureServingInfo holds configuration of the TLS server.
	SecureServingInfo *SecureServingInfo

	// ExternalAddress is the address (hostname or IP and port) that should be used in
	// external (public internet) URLs for this GenericAPIServer.
	ExternalAddress string

	// Serializer controls how common API objects not in a group/version prefix are serialized for this server.
	// Individual APIGroups may define their own serializers.
	Serializer runtime.NegotiatedSerializer

	// "Outputs"
	// Handler holds the handlers being used by this API server
	Handler *APIServerHandler

	// UnprotectedDebugSocket is used to serve pprof information in a unix-domain socket. This socket is
	// not protected by authentication/authorization.
	UnprotectedDebugSocket *routes.DebugSocket

	// listedPathProvider is a lister which provides the set of paths to show at /
	listedPathProvider routes.ListedPathProvider

	// DiscoveryGroupManager serves /apis in an unaggregated form.
	DiscoveryGroupManager discovery.GroupManager

	// AggregatedDiscoveryGroupManager serves /apis in an aggregated form.
	AggregatedDiscoveryGroupManager discoveryendpoint.ResourceManager

	// PeerAggregatedDiscoveryManager serves /apis aggregated from all peer apiservers.
	PeerAggregatedDiscoveryManager discoveryendpoint.PeerAggregatedResourceManager

	// AggregatedLegacyDiscoveryGroupManager serves /api in an aggregated form.
	AggregatedLegacyDiscoveryGroupManager discoveryendpoint.ResourceManager

	// Enable swagger and/or OpenAPI if these configs are non-nil.
	openAPIConfig *openapicommon.Config

	// Enable swagger and/or OpenAPI V3 if these configs are non-nil.
	openAPIV3Config *openapicommon.OpenAPIV3Config

	// SkipOpenAPIInstallation indicates not to install the OpenAPI handler
	// during PrepareRun.
	// Set this to true when the specific API Server has its own OpenAPI handler
	// (e.g. kube-aggregator)
	skipOpenAPIInstallation bool

	// OpenAPIVersionedService controls the /openapi/v2 endpoint, and can be used to update the served spec.
	// It is set during PrepareRun if `openAPIConfig` is non-nil unless `skipOpenAPIInstallation` is true.
	OpenAPIVersionedService *handler.OpenAPIService

	// OpenAPIV3VersionedService controls the /openapi/v3 endpoint and can be used to update the served spec.
	// It is set during PrepareRun if `openAPIConfig` is non-nil unless `skipOpenAPIInstallation` is true.
	OpenAPIV3VersionedService *handler3.OpenAPIService

	// StaticOpenAPISpec is the spec derived from the restful container endpoints.
	// It is set during PrepareRun.
	StaticOpenAPISpec *spec.Swagger

	// PostStartHooks are each called after the server has started listening, in a separate go func for each
	// with no guarantee of ordering between them.  The map key is a name used for error reporting.
	// It may kill the process with a panic if it wishes to by returning an error.
	postStartHookLock      sync.Mutex
	postStartHooks         map[string]postStartHookEntry
	postStartHooksCalled   bool
	disabledPostStartHooks sets.String

	preShutdownHookLock    sync.Mutex
	preShutdownHooks       map[string]preShutdownHookEntry
	preShutdownHooksCalled bool

	// healthz checks
	healthzRegistry healthCheckRegistry
	readyzRegistry  healthCheckRegistry
	livezRegistry   healthCheckRegistry

	livezGracePeriod time.Duration

	// auditing. The backend is started before the server starts listening.
	AuditBackend audit.Backend

	// Authorizer determines whether a user is allowed to make a certain request. The Handler does a preliminary
	// authorization check using the request URI but it may be necessary to make additional checks, such as in
	// the create-on-update case
	Authorizer authorizer.UnconditionalAuthorizer

	// EquivalentResourceRegistry provides information about resources equivalent to a given resource,
	// and the kind associated with a given resource. As resources are installed, they are registered here.
	EquivalentResourceRegistry runtime.EquivalentResourceRegistry

	// delegationTarget is the next delegate in the chain. This is never nil.
	delegationTarget DelegationTarget

	// NonLongRunningRequestWaitGroup allows you to wait for all chain
	// handlers associated with non long-running requests
	// to complete while the server is shuting down.
	NonLongRunningRequestWaitGroup *utilwaitgroup.SafeWaitGroup
	// WatchRequestWaitGroup allows us to wait for all chain
	// handlers associated with active watch requests to
	// complete while the server is shuting down.
	WatchRequestWaitGroup *utilwaitgroup.RateLimitedSafeWaitGroup

	// ShutdownDelayDuration allows to block shutdown for some time, e.g. until endpoints pointing to this API server
	// have converged on all node. During this time, the API server keeps serving, /healthz will return 200,
	// but /readyz will return failure.
	ShutdownDelayDuration time.Duration

	// The limit on the request body size that would be accepted and decoded in a write request.
	// 0 means no limit.
	maxRequestBodyBytes int64

	// APIServerID is the ID of this API server
	APIServerID string

	// StorageReadinessHook implements post-start-hook functionality for checking readiness
	// of underlying storage for registered resources.
	StorageReadinessHook *StorageReadinessHook

	// StorageVersionManager holds the storage versions of the API resources installed by this server.
	StorageVersionManager storageversion.Manager

	// EffectiveVersion determines which apis and features are available
	// based on when the api/feature lifecyle.
	EffectiveVersion basecompatibility.EffectiveVersion
	// EmulationForwardCompatible is an option to implicitly enable all APIs which are introduced after the emulation version and
	// have higher priority than APIs of the same group resource enabled at the emulation version.
	// If true, all APIs that have higher priority than the APIs(beta+) of the same group resource enabled at the emulation version will be installed.
	// This is needed when a controller implementation migrates to newer API versions, for the binary version, and also uses the newer API versions even when emulation version is set.
	// Not applicable to alpha APIs.
	EmulationForwardCompatible bool
	// RuntimeConfigEmulationForwardCompatible is an option to explicitly enable specific APIs introduced after the emulation version through the runtime-config.
	// If true, APIs identified by group/version that are enabled in the --runtime-config flag will be installed even if it is introduced after the emulation version. --runtime-config flag values that identify multiple APIs, such as api/all,api/ga,api/beta, are not influenced by this flag and will only enable APIs available at the current emulation version.
	// If false, error would be thrown if any GroupVersion or GroupVersionResource explicitly enabled in the --runtime-config flag is introduced after the emulation version.
	RuntimeConfigEmulationForwardCompatible bool

	// FeatureGate is a way to plumb feature gate through if you have them.
	FeatureGate featuregate.FeatureGate

	// lifecycleSignals provides access to the various signals that happen during the life cycle of the apiserver.
	lifecycleSignals lifecycleSignals

	// destroyFns contains a list of functions that should be called on shutdown to clean up resources.
	destroyFns []func()

	// muxAndDiscoveryCompleteSignals holds signals that indicate all known HTTP paths have been registered.
	// it exists primarily to avoid returning a 404 response when a resource actually exists but we haven't installed the path to a handler.
	// it is exposed for easier composition of the individual servers.
	// the primary users of this field are the WithMuxCompleteProtection filter and the NotFoundHandler
	muxAndDiscoveryCompleteSignals map[string]<-chan struct{}

	// ShutdownSendRetryAfter dictates when to initiate shutdown of the HTTP
	// Server during the graceful termination of the apiserver. If true, we wait
	// for non longrunning requests in flight to be drained and then initiate a
	// shutdown of the HTTP Server. If false, we initiate a shutdown of the HTTP
	// Server as soon as ShutdownDelayDuration has elapsed.
	// If enabled, after ShutdownDelayDuration elapses, any incoming request is
	// rejected with a 429 status code and a 'Retry-After' response.
	ShutdownSendRetryAfter bool

	// ShutdownWatchTerminationGracePeriod, if set to a positive value,
	// is the maximum duration the apiserver will wait for all active
	// watch request(s) to drain.
	// Once this grace period elapses, the apiserver will no longer
	// wait for any active watch request(s) in flight to drain, it will
	// proceed to the next step in the graceful server shutdown process.
	// If set to a positive value, the apiserver will keep track of the
	// number of active watch request(s) in flight and during shutdown
	// it will wait, at most, for the specified duration and allow these
	// active watch requests to drain with some rate limiting in effect.
	// The default is zero, which implies the apiserver will not keep
	// track of active watch request(s) in flight and will not wait
	// for them to drain, this maintains backward compatibility.
	// This grace period is orthogonal to other grace periods, and
	// it is not overridden by any other grace period.
	ShutdownWatchTerminationGracePeriod time.Duration
}

// DelegationTarget is an interface which allows for composition of API servers with top level handling that works
// as expected.
type DelegationTarget interface {
	// UnprotectedHandler returns a handler that is NOT protected by a normal chain
	UnprotectedHandler() http.Handler

	// PostStartHooks returns the post-start hooks that need to be combined
	PostStartHooks() map[string]postStartHookEntry

	// PreShutdownHooks returns the pre-stop hooks that need to be combined
	PreShutdownHooks() map[string]preShutdownHookEntry

	// HealthzChecks returns the healthz checks that need to be combined
	HealthzChecks() []healthz.HealthChecker

	// ListedPaths returns the paths for supporting an index
	ListedPaths() []string

	// NextDelegate returns the next delegationTarget in the chain of delegations
	NextDelegate() DelegationTarget

	// PrepareRun does post API installation setup steps. It calls recursively the same function of the delegates.
	PrepareRun() preparedGenericAPIServer

	// MuxAndDiscoveryCompleteSignals exposes registered signals that indicate if all known HTTP paths have been installed.
	MuxAndDiscoveryCompleteSignals() map[string]<-chan struct{}

	// Destroy cleans up its resources on shutdown.
	// Destroy has to be implemented in thread-safe way and be prepared
	// for being called more than once.
	Destroy()
}

func (s *GenericAPIServer) UnprotectedHandler() http.Handler {
	// when we delegate, we need the server we're delegating to choose whether or not to use gorestful
	return s.Handler.Director
}
func (s *GenericAPIServer) PostStartHooks() map[string]postStartHookEntry {
	return s.postStartHooks
}
func (s *GenericAPIServer) PreShutdownHooks() map[string]preShutdownHookEntry {
	return s.preShutdownHooks
}
func (s *GenericAPIServer) HealthzChecks() []healthz.HealthChecker {
	return s.healthzRegistry.checks
}
func (s *GenericAPIServer) ListedPaths() []string {
	return s.listedPathProvider.ListedPaths()
}

func (s *GenericAPIServer) NextDelegate() DelegationTarget {
	return s.delegationTarget
}

// RegisterMuxAndDiscoveryCompleteSignal registers the given signal that will be used to determine if all known
// HTTP paths have been registered. It is okay to call this method after instantiating the generic server but before running.
func (s *GenericAPIServer) RegisterMuxAndDiscoveryCompleteSignal(signalName string, signal <-chan struct{}) error {
	if _, exists := s.muxAndDiscoveryCompleteSignals[signalName]; exists {
		return fmt.Errorf("%s already registered", signalName)
	}
	s.muxAndDiscoveryCompleteSignals[signalName] = signal
	return nil
}

func (s *GenericAPIServer) MuxAndDiscoveryCompleteSignals() map[string]<-chan struct{} {
	return s.muxAndDiscoveryCompleteSignals
}

// RegisterDestroyFunc registers a function that will be called during Destroy().
// The function have to be idempotent and prepared to be called more than once.
func (s *GenericAPIServer) RegisterDestroyFunc(destroyFn func()) {
	s.destroyFns = append(s.destroyFns, destroyFn)
}

// Destroy cleans up all its and its delegation target resources on shutdown.
// It starts with destroying its own resources and later proceeds with
// its delegation target.
func (s *GenericAPIServer) Destroy() {
	for _, destroyFn := range s.destroyFns {
		destroyFn()
	}
	if s.delegationTarget != nil {
		s.delegationTarget.Destroy()
	}
}

type emptyDelegate struct {
	// handler is called at the end of the delegation chain
	// when a request has been made against an unregistered HTTP path the individual servers will simply pass it through until it reaches the handler.
	handler http.Handler
}

func NewEmptyDelegate() DelegationTarget {
	return emptyDelegate{}
}

// NewEmptyDelegateWithCustomHandler allows for registering a custom handler usually for special handling of 404 requests
func NewEmptyDelegateWithCustomHandler(handler http.Handler) DelegationTarget {
	return emptyDelegate{handler}
}

func (s emptyDelegate) UnprotectedHandler() http.Handler {
	return s.handler
}
func (s emptyDelegate) PostStartHooks() map[string]postStartHookEntry {
	return map[string]postStartHookEntry{}
}
func (s emptyDelegate) PreShutdownHooks() map[string]preShutdownHookEntry {
	return map[string]preShutdownHookEntry{}
}
func (s emptyDelegate) HealthzChecks() []healthz.HealthChecker {
	return []healthz.HealthChecker{}
}
func (s emptyDelegate) ListedPaths() []string {
	return []string{}
}
func (s emptyDelegate) NextDelegate() DelegationTarget {
	return nil
}
func (s emptyDelegate) PrepareRun() preparedGenericAPIServer {
	return preparedGenericAPIServer{nil}
}
func (s emptyDelegate) MuxAndDiscoveryCompleteSignals() map[string]<-chan struct{} {
	return map[string]<-chan struct{}{}
}
func (s emptyDelegate) Destroy() {
}

// preparedGenericAPIServer is a private wrapper that enforces a call of PrepareRun() before Run can be invoked.
type preparedGenericAPIServer struct {
	*GenericAPIServer
}

// PrepareRun does post API installation setup steps. It calls recursively the same function of the delegates.
func (s *GenericAPIServer) PrepareRun() preparedGenericAPIServer {
	s.delegationTarget.PrepareRun()

	if s.openAPIConfig != nil && !s.skipOpenAPIInstallation {
		s.OpenAPIVersionedService, s.StaticOpenAPISpec = routes.OpenAPI{
			Config: s.openAPIConfig,
		}.InstallV2(s.Handler.GoRestfulContainer, s.Handler.NonGoRestfulMux)
	}

	if s.openAPIV3Config != nil && !s.skipOpenAPIInstallation {
		s.OpenAPIV3VersionedService = routes.OpenAPI{
			V3Config: s.openAPIV3Config,
		}.InstallV3(s.Handler.GoRestfulContainer, s.Handler.NonGoRestfulMux)
	}

	s.installHealthz()
	s.installLivez()

	// as soon as shutdown is initiated, readiness should start failing
	readinessStopCh := s.lifecycleSignals.ShutdownInitiated.Signaled()
	err := s.addReadyzShutdownCheck(readinessStopCh)
	if err != nil {
		klog.Errorf("Failed to install readyz shutdown check %s", err)
	}
	s.installReadyz()

	componentName := "apiserver"
	if utilfeature.DefaultFeatureGate.Enabled(zpagesfeatures.ComponentFlagz) {
		if s.Flagz != nil {
			flagz.Install(s.Handler.NonGoRestfulMux, componentName, s.Flagz)
		}
	}
	// statusz is installed last so that it can list all the paths that have been registered
	if utilfeature.DefaultFeatureGate.Enabled(zpagesfeatures.ComponentStatusz) {
		statusz.Install(s.Handler.NonGoRestfulMux, componentName, statusz.NewRegistry(s.EffectiveVersion, statusz.WithListedPaths(s.ListedPaths())))
	}

	return preparedGenericAPIServer{s}
}

// Run spawns the secure http server. It only returns if stopCh is closed
// or the secure port cannot be listened on initially.
//
// Deprecated: use RunWithContext instead. Run will not get removed to avoid
// breaking consumers, but should not be used in new code.
func (s preparedGenericAPIServer) Run(stopCh <-chan struct{}) error {
	ctx := wait.ContextForChannel(stopCh)
	return s.RunWithContext(ctx)
}

// RunWithContext spawns the secure http server. It only returns if ctx is canceled
// or the secure port cannot be listened on initially.
// This is the diagram of what contexts/channels/signals are dependent on each other:
//
// |                                   ctx
// |                                    |
// |           ---------------------------------------------------------
// |           |                                                       |
// |    ShutdownInitiated (shutdownInitiatedCh)                        |
// |           |                                                       |
// | (ShutdownDelayDuration)                                    (PreShutdownHooks)
// |           |                                                       |
// |  AfterShutdownDelayDuration (delayedStopCh)   PreShutdownHooksStopped (preShutdownHooksHasStoppedCh)
// |           |                                                       |
// |           |-------------------------------------------------------|
// |                                    |
// |                                    |
// |               NotAcceptingNewRequest (notAcceptingNewRequestCh)
// |                                    |
// |                                    |
// |           |----------------------------------------------------------------------------------|
// |           |                        |              |                                          |
// |        [without                 [with             |                                          |
// | ShutdownSendRetryAfter]  ShutdownSendRetryAfter]  |                                          |
// |           |                        |              |                                          |
// |           |                        ---------------|                                          |
// |           |                                       |                                          |
// |           |                      |----------------|-----------------------|                  |
// |           |                      |                                        |                  |
// |           |         (NonLongRunningRequestWaitGroup::Wait)   (WatchRequestWaitGroup::Wait)   |
// |           |                      |                                        |                  |
// |           |                      |------------------|---------------------|                  |
// |           |                                         |                                        |
// |           |                         InFlightRequestsDrained (drainedCh)                      |
// |           |                                         |                                        |
// |           |-------------------|---------------------|----------------------------------------|
// |                               |                     |
// |                       stopHttpServerCtx     (AuditBackend::Shutdown())
// |                               |
// |                       listenerStoppedCh
// |                               |
// |      HTTPServerStoppedListening (httpServerStoppedListeningCh)
func (s preparedGenericAPIServer) RunWithContext(ctx context.Context) error {
	stopCh := ctx.Done()
	delayedStopCh := s.lifecycleSignals.AfterShutdownDelayDuration
	shutdownInitiatedCh := s.lifecycleSignals.ShutdownInitiated

	// Clean up resources on shutdown.
	defer s.Destroy()

	// If UDS profiling is enabled, start a local http server listening on that socket
	if s.UnprotectedDebugSocket != nil {
		go func() {
			defer utilruntime.HandleCrashWithContext(ctx)
			klog.Error(s.UnprotectedDebugSocket.RunWithContext(ctx))
		}()
	}

	// spawn a new goroutine for closing the MuxAndDiscoveryComplete signal
	// registration happens during construction of the generic api server
	// the last server in the chain aggregates signals from the previous instances
	go func() {
		for _, muxAndDiscoveryCompletedSignal := range s.GenericAPIServer.MuxAndDiscoveryCompleteSignals() {
			select {
			case <-muxAndDiscoveryCompletedSignal:
				continue
			case <-stopCh:
				klog.V(1).Infof("haven't completed %s, stop requested", s.lifecycleSignals.MuxAndDiscoveryComplete.Name())
				return
			}
		}
		s.lifecycleSignals.MuxAndDiscoveryComplete.Signal()
		klog.V(1).Infof("%s has all endpoints registered and discovery information is complete", s.lifecycleSignals.MuxAndDiscoveryComplete.Name())
	}()

	go func() {
		defer delayedStopCh.Signal()
		defer klog.V(1).InfoS("[graceful-termination] shutdown event", "name", delayedStopCh.Name())

		<-stopCh

		// As soon as shutdown is initiated, /readyz should start returning failure.
		// This gives the load balancer a window defined by ShutdownDelayDuration to detect that /readyz is red
		// and stop sending traffic to this server.
		shutdownInitiatedCh.Signal()
		klog.V(1).InfoS("[graceful-termination] shutdown event", "name", shutdownInitiatedCh.Name())

		time.Sleep(s.ShutdownDelayDuration)
	}()

	// close socket after delayed stopCh
	shutdownTimeout := s.ShutdownTimeout
	if s.ShutdownSendRetryAfter {
		// when this mode is enabled, we do the following:
		// - the server will continue to listen until all existing requests in flight
		//   (not including active long running requests) have been drained.
		// - once drained, http Server Shutdown is invoked with a timeout of 2s,
		//   net/http waits for 1s for the peer to respond to a GO_AWAY frame, so
		//   we should wait for a minimum of 2s
		shutdownTimeout = 2 * time.Second
		klog.V(1).InfoS("[graceful-termination] using HTTP Server shutdown timeout", "shutdownTimeout", shutdownTimeout)
	}

	notAcceptingNewRequestCh := s.lifecycleSignals.NotAcceptingNewRequest
	drainedCh := s.lifecycleSignals.InFlightRequestsDrained
	// Canceling the parent context does not immediately cancel the HTTP server.
	// We only inherit context values here and deal with cancellation ourselves.
	stopHTTPServerCtx, stopHTTPServer := context.WithCancelCause(context.WithoutCancel(ctx))
	go func() {
		defer stopHTTPServer(errors.New("time to stop HTTP server"))

		timeToStopHttpServerCh := notAcceptingNewRequestCh.Signaled()
		if s.ShutdownSendRetryAfter {
			timeToStopHttpServerCh = drainedCh.Signaled()
		}

		<-timeToStopHttpServerCh
	}()

	// Start the audit backend before any request comes in. This means we must call Backend.Run
	// before http server start serving. Otherwise the Backend.ProcessEvents call might block.
	// AuditBackend.Run will stop as soon as all in-flight requests are drained.
	if s.AuditBackend != nil {
		if err := s.AuditBackend.Run(drainedCh.Signaled()); err != nil {
			return fmt.Errorf("failed to run the audit backend: %v", err)
		}
	}

	stoppedCh, listenerStoppedCh, err := s.NonBlockingRunWithContext(stopHTTPServerCtx, shutdownTimeout)
	if err != nil {
		return err
	}

	httpServerStoppedListeningCh := s.lifecycleSignals.HTTPServerStoppedListening
	go func() {
		<-listenerStoppedCh
		httpServerStoppedListeningCh.Signal()
		klog.V(1).InfoS("[graceful-termination] shutdown event", "name", httpServerStoppedListeningCh.Name())
	}()

	// we don't accept new request as soon as both ShutdownDelayDuration has
	// elapsed and preshutdown hooks have completed.
	preShutdownHooksHasStoppedCh := s.lifecycleSignals.PreShutdownHooksStopped
	go func() {
		defer klog.V(1).InfoS("[graceful-termination] shutdown event", "name", notAcceptingNewRequestCh.Name())
		defer notAcceptingNewRequestCh.Signal()

		// wait for the delayed stopCh before closing the handler chain
		<-delayedStopCh.Signaled()

		// Additionally wait for preshutdown hooks to also be finished, as some of them need
		// to send API calls to clean up after themselves (e.g. lease reconcilers removing
		// itself from the active servers).
		<-preShutdownHooksHasStoppedCh.Signaled()
	}()

	// wait for all in-flight non-long running requests to finish
	nonLongRunningRequestDrainedCh := make(chan struct{})
	go func() {
		defer close(nonLongRunningRequestDrainedCh)
		defer klog.V(1).Info("[graceful-termination] in-flight non long-running request(s) have drained")

		// wait for the delayed stopCh before closing the handler chain (it rejects everything after Wait has been called).
		<-notAcceptingNewRequestCh.Signaled()

		// Wait for all requests to finish, which are bounded by the RequestTimeout variable.
		// once NonLongRunningRequestWaitGroup.Wait is invoked, the apiserver is
		// expected to reject any incoming request with a {503, Retry-After}
		// response via the WithWaitGroup filter. On the contrary, we observe
		// that incoming request(s) get a 'connection refused' error, this is
		// because, at this point, we have called 'Server.Shutdown' and
		// net/http server has stopped listening. This causes incoming
		// request to get a 'connection refused' error.
		// On the other hand, if 'ShutdownSendRetryAfter' is enabled incoming
		// requests will be rejected with a {429, Retry-After} since
		// 'Server.Shutdown' will be invoked only after in-flight requests
		// have been drained.
		// TODO: can we consolidate these two modes of graceful termination?
		s.NonLongRunningRequestWaitGroup.Wait()
	}()

	// wait for all in-flight watches to finish
	activeWatchesDrainedCh := make(chan struct{})
	go func() {
		defer close(activeWatchesDrainedCh)

		<-notAcceptingNewRequestCh.Signaled()
		if s.ShutdownWatchTerminationGracePeriod <= time.Duration(0) {
			klog.V(1).InfoS("[graceful-termination] not going to wait for active watch request(s) to drain")
			return
		}

		// Wait for all active watches to finish
		grace := s.ShutdownWatchTerminationGracePeriod
		activeBefore, activeAfter, err := s.WatchRequestWaitGroup.Wait(func(count int) (utilwaitgroup.RateLimiter, context.Context, context.CancelFunc) {
			qps := float64(count) / grace.Seconds()
			// TODO: we don't want the QPS (max requests drained per second) to
			//  get below a certain floor value, since we want the server to
			//  drain the active watch requests as soon as possible.
			//  For now, it's hard coded to 200, and it is subject to change
			//  based on the result from the scale testing.
			if qps < 200 {
				qps = 200
			}

			ctx, cancel := context.WithTimeout(context.Background(), grace)
			// We don't expect more than one token to be consumed
			// in a single Wait call, so setting burst to 1.
			return rate.NewLimiter(rate.Limit(qps), 1), ctx, cancel
		})
		klog.V(1).InfoS("[graceful-termination] active watch request(s) have drained",
			"duration", grace, "activeWatchesBefore", activeBefore, "activeWatchesAfter", activeAfter, "error", err)
	}()

	go func() {
		defer klog.V(1).InfoS("[graceful-termination] shutdown event", "name", drainedCh.Name())
		defer drainedCh.Signal()

		<-nonLongRunningRequestDrainedCh
		<-activeWatchesDrainedCh
	}()

	klog.V(1).Info("[graceful-termination] waiting for shutdown to be initiated")
	<-stopCh

	// run shutdown hooks directly. This includes deregistering from
	// the kubernetes endpoint in case of kube-apiserver.
	func() {
		defer func() {
			preShutdownHooksHasStoppedCh.Signal()
			klog.V(1).InfoS("[graceful-termination] pre-shutdown hooks completed", "name", preShutdownHooksHasStoppedCh.Name())
		}()
		err = s.RunPreShutdownHooks()
	}()
	if err != nil {
		return err
	}

	// Wait for all requests in flight to drain, bounded by the RequestTimeout variable.
	<-drainedCh.Signaled()

	if s.AuditBackend != nil {
		s.AuditBackend.Shutdown()
		klog.V(1).InfoS("[graceful-termination] audit backend shutdown completed")
	}

	// wait for stoppedCh that is closed when the graceful termination (server.Shutdown) is finished.
	<-listenerStoppedCh
	<-stoppedCh

	klog.V(1).Info("[graceful-termination] apiserver is exiting")
	return nil
}

// NonBlockingRun spawns the secure http server. An error is
// returned if the secure port cannot be listened on.
// The returned channel is closed when the (asynchronous) termination is finished.
//
// Deprecated: use RunWithContext instead. Run will not get removed to avoid
// breaking consumers, but should not be used in new code.
func (s preparedGenericAPIServer) NonBlockingRun(stopCh <-chan struct{}, shutdownTimeout time.Duration) (<-chan struct{}, <-chan struct{}, error) {
	ctx := wait.ContextForChannel(stopCh)
	return s.NonBlockingRunWithContext(ctx, shutdownTimeout)
}

// NonBlockingRunWithContext spawns the secure http server. An error is
// returned if the secure port cannot be listened on.
// The returned channel is closed when the (asynchronous) termination is finished.
func (s preparedGenericAPIServer) NonBlockingRunWithContext(ctx context.Context, shutdownTimeout time.Duration) (<-chan struct{}, <-chan struct{}, error) {
	// Use an internal stop channel to allow cleanup of the listeners on error.
	internalStopCh := make(chan struct{})
	var stoppedCh <-chan struct{}
	var listenerStoppedCh <-chan struct{}
	if s.SecureServingInfo != nil && s.Handler != nil {
		var err error
		stoppedCh, listenerStoppedCh, err = s.SecureServingInfo.Serve(s.Handler, shutdownTimeout, internalStopCh)
		if err != nil {
			close(internalStopCh)
			return nil, nil, err
		}
	}

	// Now that listener have bound successfully, it is the
	// responsibility of the caller to close the provided channel to
	// ensure cleanup.
	go func() {
		<-ctx.Done()
		close(internalStopCh)
	}()

	s.RunPostStartHooks(ctx)

	if _, err := systemd.SdNotify(true, "READY=1\n"); err != nil {
		klog.Errorf("Unable to send systemd daemon successful start message: %v\n", err)
	}

	return stoppedCh, listenerStoppedCh, nil
}

// installAPIResources is a private method for installing the REST storage backing each api groupversionresource
func (s *GenericAPIServer) installAPIResources(apiPrefix string, apiGroupInfo *APIGroupInfo, typeConverter managedfields.TypeConverter) error {
	var resourceInfos []*storageversion.ResourceInfo
	for _, groupVersion := range apiGroupInfo.PrioritizedVersions {
		if len(apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version]) == 0 {
			klog.Warningf("Skipping API %v because it has no resources.", groupVersion)
			continue
		}

		apiGroupVersion, err := s.getAPIGroupVersion(apiGroupInfo, groupVersion, apiPrefix)
		if err != nil {
			return err
		}
		if apiGroupInfo.OptionsExternalVersion != nil {
			apiGroupVersion.OptionsExternalVersion = apiGroupInfo.OptionsExternalVersion
		}
		apiGroupVersion.TypeConverter = typeConverter
		apiGroupVersion.MaxRequestBodyBytes = s.maxRequestBodyBytes

		discoveryAPIResources, r, err := apiGroupVersion.InstallREST(s.Handler.GoRestfulContainer)

		if err != nil {
			return fmt.Errorf("unable to setup API %v: %v", apiGroupInfo, err)
		}
		resourceInfos = append(resourceInfos, r...)

		// Aggregated discovery only aggregates resources under /apis
		if apiPrefix == APIGroupPrefix {
			s.AggregatedDiscoveryGroupManager.AddGroupVersion(
				groupVersion.Group,
				apidiscoveryv2.APIVersionDiscovery{
					Freshness: apidiscoveryv2.DiscoveryFreshnessCurrent,
					Version:   groupVersion.Version,
					Resources: discoveryAPIResources,
				},
			)
		} else {
			// There is only one group version for legacy resources, priority can be defaulted to 0.
			s.AggregatedLegacyDiscoveryGroupManager.AddGroupVersion(
				groupVersion.Group,
				apidiscoveryv2.APIVersionDiscovery{
					Freshness: apidiscoveryv2.DiscoveryFreshnessCurrent,
					Version:   groupVersion.Version,
					Resources: discoveryAPIResources,
				},
			)
		}

	}

	s.RegisterDestroyFunc(apiGroupInfo.destroyStorage)

	if s.FeatureGate.Enabled(features.StorageVersionAPI) &&
		s.FeatureGate.Enabled(features.APIServerIdentity) {
		// API installation happens before we start listening on the handlers,
		// therefore it is safe to register ResourceInfos here. The handler will block
		// write requests until the storage versions of the targeting resources are updated.
		s.StorageVersionManager.AddResourceInfo(resourceInfos...)
	}

	return nil
}

// InstallLegacyAPIGroup exposes the given legacy api group in the API.
// The <apiGroupInfo> passed into this function shouldn't be used elsewhere as the
// underlying storage will be destroyed on this servers shutdown.
func (s *GenericAPIServer) InstallLegacyAPIGroup(apiPrefix string, apiGroupInfo *APIGroupInfo) error {
	if !s.legacyAPIGroupPrefixes.Has(apiPrefix) {
		return fmt.Errorf("%q is not in the allowed legacy API prefixes: %v", apiPrefix, s.legacyAPIGroupPrefixes.List())
	}

	openAPIModels, err := s.getOpenAPIModels(apiPrefix, apiGroupInfo)
	if err != nil {
		return fmt.Errorf("unable to get openapi models: %v", err)
	}

	if err := s.installAPIResources(apiPrefix, apiGroupInfo, openAPIModels); err != nil {
		return err
	}

	// Install the version handler.
	// Add a handler at /<apiPrefix> to enumerate the supported api versions.
	legacyRootAPIHandler := discovery.NewLegacyRootAPIHandler(s.discoveryAddresses, s.Serializer, apiPrefix)
	// No peer-to-peer discovery for legacy API group.
	wrapped := discoveryendpoint.WrapAggregatedDiscoveryToHandler(legacyRootAPIHandler, s.AggregatedLegacyDiscoveryGroupManager, s.AggregatedLegacyDiscoveryGroupManager)
	s.Handler.GoRestfulContainer.Add(wrapped.GenerateWebService("/api", metav1.APIVersions{}))
	s.registerStorageReadinessCheck("", apiGroupInfo)

	return nil
}

// InstallAPIGroups exposes given api groups in the API.
// The <apiGroupInfos> passed into this function shouldn't be used elsewhere as the
// underlying storage will be destroyed on this servers shutdown.
func (s *GenericAPIServer) InstallAPIGroups(apiGroupInfos ...*APIGroupInfo) error {
	for _, apiGroupInfo := range apiGroupInfos {
		if len(apiGroupInfo.PrioritizedVersions) == 0 {
			return fmt.Errorf("no version priority set for %#v", *apiGroupInfo)
		}
		// Do not register empty group or empty version.  Doing so claims /apis/ for the wrong entity to be returned.
		// Catching these here places the error  much closer to its origin
		if len(apiGroupInfo.PrioritizedVersions[0].Group) == 0 {
			return fmt.Errorf("cannot register handler with an empty group for %#v", *apiGroupInfo)
		}
		if len(apiGroupInfo.PrioritizedVersions[0].Version) == 0 {
			return fmt.Errorf("cannot register handler with an empty version for %#v", *apiGroupInfo)
		}
	}

	openAPIModels, err := s.getOpenAPIModels(APIGroupPrefix, apiGroupInfos...)
	if err != nil {
		return fmt.Errorf("unable to get openapi models: %v", err)
	}

	for _, apiGroupInfo := range apiGroupInfos {
		if err := s.installAPIResources(APIGroupPrefix, apiGroupInfo, openAPIModels); err != nil {
			return fmt.Errorf("unable to install api resources: %v", err)
		}

		// setup discovery
		// Install the version handler.
		// Add a handler at /apis/<groupName> to enumerate all versions supported by this group.
		apiVersionsForDiscovery := []metav1.GroupVersionForDiscovery{}
		for _, groupVersion := range apiGroupInfo.PrioritizedVersions {
			// Check the config to make sure that we elide versions that don't have any resources
			if len(apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version]) == 0 {
				continue
			}
			apiVersionsForDiscovery = append(apiVersionsForDiscovery, metav1.GroupVersionForDiscovery{
				GroupVersion: groupVersion.String(),
				Version:      groupVersion.Version,
			})
		}
		preferredVersionForDiscovery := metav1.GroupVersionForDiscovery{
			GroupVersion: apiGroupInfo.PrioritizedVersions[0].String(),
			Version:      apiGroupInfo.PrioritizedVersions[0].Version,
		}
		apiGroup := metav1.APIGroup{
			Name:             apiGroupInfo.PrioritizedVersions[0].Group,
			Versions:         apiVersionsForDiscovery,
			PreferredVersion: preferredVersionForDiscovery,
		}

		s.DiscoveryGroupManager.AddGroup(apiGroup)
		s.Handler.GoRestfulContainer.Add(discovery.NewAPIGroupHandler(s.Serializer, apiGroup).WebService())
		s.registerStorageReadinessCheck(apiGroupInfo.PrioritizedVersions[0].Group, apiGroupInfo)
	}
	return nil
}

// registerStorageReadinessCheck registers the readiness checks for all underlying storages
// for a given APIGroup.
func (s *GenericAPIServer) registerStorageReadinessCheck(groupName string, apiGroupInfo *APIGroupInfo) {
	for version, storageMap := range apiGroupInfo.VersionedResourcesStorageMap {
		for resource, storage := range storageMap {
			if withReadiness, ok := storage.(rest.StorageWithReadiness); ok {
				gvr := metav1.GroupVersionResource{
					Group:    groupName,
					Version:  version,
					Resource: resource,
				}
				s.StorageReadinessHook.RegisterStorage(gvr, withReadiness)
			}
		}
	}
}

// InstallAPIGroup exposes the given api group in the API.
// The <apiGroupInfo> passed into this function shouldn't be used elsewhere as the
// underlying storage will be destroyed on this servers shutdown.
func (s *GenericAPIServer) InstallAPIGroup(apiGroupInfo *APIGroupInfo) error {
	return s.InstallAPIGroups(apiGroupInfo)
}

func (s *GenericAPIServer) getAPIGroupVersion(apiGroupInfo *APIGroupInfo, groupVersion schema.GroupVersion, apiPrefix string) (*genericapi.APIGroupVersion, error) {
	storage := make(map[string]rest.Storage)
	for k, v := range apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version] {
		if strings.ToLower(k) != k {
			return nil, fmt.Errorf("resource names must be lowercase only, not %q", k)
		}
		storage[k] = v
	}
	version := s.newAPIGroupVersion(apiGroupInfo, groupVersion)
	version.Root = apiPrefix
	version.Storage = storage
	return version, nil
}

func (s *GenericAPIServer) newAPIGroupVersion(apiGroupInfo *APIGroupInfo, groupVersion schema.GroupVersion) *genericapi.APIGroupVersion {

	allServedVersionsByResource := map[string][]string{}
	for version, resourcesInVersion := range apiGroupInfo.VersionedResourcesStorageMap {
		for resource := range resourcesInVersion {
			if len(groupVersion.Group) == 0 {
				allServedVersionsByResource[resource] = append(allServedVersionsByResource[resource], version)
			} else {
				allServedVersionsByResource[resource] = append(allServedVersionsByResource[resource], fmt.Sprintf("%s/%s", groupVersion.Group, version))
			}
		}
	}

	return &genericapi.APIGroupVersion{
		GroupVersion:                groupVersion,
		AllServedVersionsByResource: allServedVersionsByResource,
		MetaGroupVersion:            apiGroupInfo.MetaGroupVersion,

		ParameterCodec:        apiGroupInfo.ParameterCodec,
		Serializer:            apiGroupInfo.NegotiatedSerializer,
		Creater:               apiGroupInfo.Scheme,
		Convertor:             apiGroupInfo.Scheme,
		ConvertabilityChecker: apiGroupInfo.Scheme,
		UnsafeConvertor:       runtime.UnsafeObjectConvertor(apiGroupInfo.Scheme),
		Defaulter:             apiGroupInfo.Scheme,
		Typer:                 apiGroupInfo.Scheme,
		Namer:                 runtime.Namer(meta.NewAccessor()),

		EquivalentResourceRegistry: s.EquivalentResourceRegistry,

		Admit:             s.admissionControl,
		MinRequestTimeout: s.minRequestTimeout,
		Authorizer:        s.Authorizer,
	}
}

// NewDefaultAPIGroupInfo returns an APIGroupInfo stubbed with "normal" values
// exposed for easier composition from other packages
func NewDefaultAPIGroupInfo(group string, scheme *runtime.Scheme, parameterCodec runtime.ParameterCodec, codecs serializer.CodecFactory) APIGroupInfo {
	opts := []serializer.CodecFactoryOptionsMutator{
		serializer.WithStreamingCollectionEncodingToJSON(),
		serializer.WithStreamingCollectionEncodingToProtobuf(),
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.CBORServingAndStorage) {
		opts = append(opts, serializer.WithSerializer(cbor.NewSerializerInfo))
	}
	if len(opts) != 0 {
		codecs = serializer.NewCodecFactory(scheme, opts...)
	}
	return APIGroupInfo{
		PrioritizedVersions:          scheme.PrioritizedVersionsForGroup(group),
		VersionedResourcesStorageMap: map[string]map[string]rest.Storage{},
		// TODO unhardcode this.  It was hardcoded before, but we need to re-evaluate
		OptionsExternalVersion: &schema.GroupVersion{Version: "v1"},
		Scheme:                 scheme,
		ParameterCodec:         parameterCodec,
		NegotiatedSerializer:   codecs,
	}
}

// getOpenAPIModels is a private method for getting the OpenAPI models
func (s *GenericAPIServer) getOpenAPIModels(apiPrefix string, apiGroupInfos ...*APIGroupInfo) (managedfields.TypeConverter, error) {
	if s.openAPIV3Config == nil {
		// SSA is GA and requires OpenAPI config to be set
		// to create models.
		return nil, errors.New("OpenAPIV3 config must not be nil")
	}
	pathsToIgnore := openapiutil.NewTrie(s.openAPIV3Config.IgnorePrefixes)
	resourceNames := make([]string, 0)
	for _, apiGroupInfo := range apiGroupInfos {
		groupResources, err := getResourceNamesForGroup(apiPrefix, apiGroupInfo, pathsToIgnore)
		if err != nil {
			return nil, err
		}
		resourceNames = append(resourceNames, groupResources...)
	}

	// Build the openapi definitions for those resources and convert it to proto models
	openAPISpec, err := openapibuilder3.BuildOpenAPIDefinitionsForResources(s.openAPIV3Config, resourceNames...)
	if err != nil {
		return nil, err
	}
	for _, apiGroupInfo := range apiGroupInfos {
		apiGroupInfo.StaticOpenAPISpec = openAPISpec
	}

	typeConverter, err := managedfields.NewTypeConverter(openAPISpec, false)
	if err != nil {
		return nil, err
	}

	return typeConverter, nil
}

// getResourceNamesForGroup is a private method for getting the canonical names for each resource to build in an api group
func getResourceNamesForGroup(apiPrefix string, apiGroupInfo *APIGroupInfo, pathsToIgnore openapiutil.Trie) ([]string, error) {
	// Get the canonical names of every resource we need to build in this api group
	resourceNames := make([]string, 0)
	for _, groupVersion := range apiGroupInfo.PrioritizedVersions {
		for resource, storage := range apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version] {
			path := gpath.Join(apiPrefix, groupVersion.Group, groupVersion.Version, resource)
			if !pathsToIgnore.HasPrefix(path) {
				kind, err := genericapi.GetResourceKind(groupVersion, storage, apiGroupInfo.Scheme)
				if err != nil {
					return nil, err
				}
				sampleObject, err := apiGroupInfo.Scheme.New(kind)
				if err != nil {
					return nil, err
				}
				name := openapiutil.GetCanonicalTypeName(sampleObject)
				resourceNames = append(resourceNames, name)
			}
		}
	}

	return resourceNames, nil
}

```

// === FILE: references!/kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd3/store.go ===
```go
/*
Copyright 2016 The Kubernetes Authors.

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

package etcd3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/kubernetes"
	"go.opentelemetry.io/otel/attribute"

	etcdrpc "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/etcd3/metrics"
	etcdfeature "k8s.io/apiserver/pkg/storage/feature"
	"k8s.io/apiserver/pkg/storage/value"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/tracing"
	"k8s.io/klog/v2"
)

const (
	// maxLimit is a maximum page limit increase used when fetching objects from etcd.
	// This limit is used only for increasing page size by kube-apiserver. If request
	// specifies larger limit initially, it won't be changed.
	maxLimit = 10000
)

// authenticatedDataString satisfies the value.Context interface. It uses the key to
// authenticate the stored data. This does not defend against reuse of previously
// encrypted values under the same key, but will prevent an attacker from using an
// encrypted value from a different key. A stronger authenticated data segment would
// include the etcd3 Version field (which is incremented on each write to a key and
// reset when the key is deleted), but an attacker with write access to etcd can
// force deletion and recreation of keys to weaken that angle.
type authenticatedDataString string

// AuthenticatedData implements the value.Context interface.
func (d authenticatedDataString) AuthenticatedData() []byte {
	return []byte(string(d))
}

var _ value.Context = authenticatedDataString("")

type store struct {
	client             *kubernetes.Client
	codec              runtime.Codec
	versioner          storage.Versioner
	transformer        value.Transformer
	pathPrefix         string
	groupResource      schema.GroupResource
	watcher            *watcher
	leaseManager       *leaseManager
	decoder            Decoder
	listErrAggrFactory func() ListErrorAggregator

	resourcePrefix string
	newListFunc    func() runtime.Object
	compactor      Compactor

	collectorMux          sync.RWMutex
	resourceSizeEstimator *resourceSizeEstimator
}

var _ storage.Interface = (*store)(nil)

func (s *store) RequestWatchProgress(ctx context.Context) error {
	// Use watchContext to match ctx metadata provided when creating the watch.
	// In best case scenario we would use the same context that watch was created, but there is no way access it from watchCache.
	return s.client.RequestProgress(s.watchContext(ctx))
}

type objState struct {
	obj   runtime.Object
	meta  *storage.ResponseMeta
	rev   int64
	data  []byte
	stale bool
}

// ListErrorAggregator aggregates the error(s) that the LIST operation
// encounters while retrieving object(s) from the storage
type ListErrorAggregator interface {
	// Aggregate aggregates the given error from list operation
	// key: it identifies the given object in the storage.
	// err: it represents the error the list operation encountered while
	// retrieving the given object from the storage.
	// done: true if the aggregation is done and the list operation should
	// abort, otherwise the list operation will continue
	Aggregate(key string, err error) bool

	// Err returns the aggregated error
	Err() error
}

// defaultListErrorAggregatorFactory returns the default list error
// aggregator that maintains backward compatibility, which is abort
// the list operation as soon as it encounters the first error
func defaultListErrorAggregatorFactory() ListErrorAggregator { return &abortOnFirstError{} }

// LIST aborts on the first error it encounters (backward compatible)
type abortOnFirstError struct {
	err error
}

func (a *abortOnFirstError) Aggregate(key string, err error) bool {
	a.err = err
	return true
}
func (a *abortOnFirstError) Err() error { return a.err }

// New returns an etcd3 implementation of storage.Interface.
func New(c *kubernetes.Client, compactor Compactor, codec runtime.Codec, newFunc, newListFunc func() runtime.Object, prefix, resourcePrefix string, groupResource schema.GroupResource, transformer value.Transformer, leaseManagerConfig LeaseManagerConfig, decoder Decoder, versioner storage.Versioner) (*store, error) {
	// for compatibility with etcd2 impl.
	// no-op for default prefix of '/registry'.
	// keeps compatibility with etcd2 impl for custom prefixes that don't start with '/'
	pathPrefix := path.Join("/", prefix)
	if !strings.HasSuffix(pathPrefix, "/") {
		// Ensure the pathPrefix ends in "/" here to simplify key concatenation later.
		pathPrefix += "/"
	}
	if resourcePrefix == "" {
		return nil, fmt.Errorf("resourcePrefix cannot be empty")
	}
	if resourcePrefix == "/" {
		return nil, fmt.Errorf("resourcePrefix cannot be /")
	}
	if !strings.HasPrefix(resourcePrefix, "/") {
		return nil, fmt.Errorf("resourcePrefix needs to start from /")
	}

	listErrAggrFactory := defaultListErrorAggregatorFactory
	if utilfeature.DefaultFeatureGate.Enabled(features.AllowUnsafeMalformedObjectDeletion) {
		listErrAggrFactory = corruptObjErrAggregatorFactory(100)
	}

	w := &watcher{
		client:        c.Client,
		codec:         codec,
		newFunc:       newFunc,
		groupResource: groupResource,
		versioner:     versioner,
		transformer:   transformer,
	}
	if newFunc == nil {
		w.objectType = "<unknown>"
	} else {
		w.objectType = reflect.TypeOf(newFunc()).String()
	}
	s := &store{
		client:             c,
		codec:              codec,
		versioner:          versioner,
		transformer:        transformer,
		pathPrefix:         pathPrefix,
		groupResource:      groupResource,
		watcher:            w,
		leaseManager:       newDefaultLeaseManager(c.Client, leaseManagerConfig),
		decoder:            decoder,
		listErrAggrFactory: listErrAggrFactory,

		resourcePrefix: resourcePrefix,
		newListFunc:    newListFunc,
		compactor:      compactor,
	}

	w.getResourceSizeEstimator = s.getResourceSizeEstimator
	w.getCurrentStorageRV = func(ctx context.Context) (uint64, error) {
		return s.GetCurrentResourceVersion(ctx)
	}
	etcdfeature.DefaultFeatureSupportChecker.CheckClient(c.Ctx(), c, storage.RequestWatchProgress)
	return s, nil
}

func (s *store) CompactRevision() int64 {
	if s.compactor == nil {
		return 0
	}
	return s.compactor.CompactRevision()
}

// Versioner implements storage.Interface.Versioner.
func (s *store) Versioner() storage.Versioner {
	return s.versioner
}

func (s *store) Close() {
	stats := s.getResourceSizeEstimator()
	if stats != nil {
		stats.Close()
	}
}

func (s *store) getResourceSizeEstimator() *resourceSizeEstimator {
	s.collectorMux.RLock()
	defer s.collectorMux.RUnlock()
	return s.resourceSizeEstimator
}

// Get implements storage.Interface.Get.
func (s *store) Get(ctx context.Context, key string, opts storage.GetOptions, out runtime.Object) error {
	preparedKey, err := s.prepareKey(key, false)
	if err != nil {
		return err
	}
	startTime := time.Now()
	getResp, err := s.client.Kubernetes.Get(ctx, preparedKey, kubernetes.GetOptions{})
	metrics.RecordEtcdRequest("get", s.groupResource, err, startTime)
	if err != nil {
		return err
	}
	if err = s.validateMinimumResourceVersion(opts.ResourceVersion, uint64(getResp.Revision)); err != nil {
		return err
	}

	if getResp.KV == nil {
		if opts.IgnoreNotFound {
			return runtime.SetZeroValue(out)
		}
		return storage.NewKeyNotFoundError(preparedKey, 0)
	}

	data, _, err := s.transformer.TransformFromStorage(ctx, getResp.KV.Value, authenticatedDataString(preparedKey))
	if err != nil {
		return storage.NewInternalError(err)
	}

	err = s.decoder.Decode(data, out, getResp.KV.ModRevision)
	if err != nil {
		recordDecodeError(s.groupResource, preparedKey)
		return err
	}
	return nil
}

// Create implements storage.Interface.Create.
func (s *store) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	preparedKey, err := s.prepareKey(key, false)
	if err != nil {
		return err
	}
	ctx, span := tracing.Start(ctx, "Create etcd3",
		attribute.String("audit-id", audit.GetAuditIDTruncated(ctx)),
		attribute.String("key", key),
		attribute.String("type", getTypeName(obj)),
		attribute.String("group", s.groupResource.Group),
		attribute.String("resource", s.groupResource.Resource),
	)
	defer span.End(500 * time.Millisecond)
	if version, err := s.versioner.ObjectResourceVersion(obj); err == nil && version != 0 {
		return storage.ErrResourceVersionSetOnCreate
	}
	if err := s.versioner.PrepareObjectForStorage(obj); err != nil {
		return fmt.Errorf("PrepareObjectForStorage failed: %v", err)
	}
	span.AddEvent("About to Encode")
	data, err := runtime.Encode(s.codec, obj)
	if err != nil {
		span.AddEvent("Encode failed", attribute.Int("len", len(data)), attribute.String("err", err.Error()))
		return err
	}
	span.AddEvent("Encode succeeded", attribute.Int("len", len(data)))

	var lease clientv3.LeaseID
	if ttl != 0 {
		lease, err = s.leaseManager.GetLease(ctx, int64(ttl))
		if err != nil {
			return err
		}
	}

	newData, err := s.transformer.TransformToStorage(ctx, data, authenticatedDataString(preparedKey))
	if err != nil {
		span.AddEvent("TransformToStorage failed", attribute.String("err", err.Error()))
		return storage.NewInternalError(err)
	}
	span.AddEvent("TransformToStorage succeeded")

	startTime := time.Now()
	txnResp, err := s.client.Kubernetes.OptimisticPut(ctx, preparedKey, newData, 0, kubernetes.PutOptions{LeaseID: lease})
	metrics.RecordEtcdRequest("create", s.groupResource, err, startTime)
	if err != nil {
		span.AddEvent("Txn call failed", attribute.String("err", err.Error()))
		return err
	}
	span.AddEvent("Txn call succeeded")

	if !txnResp.Succeeded {
		return storage.NewKeyExistsError(preparedKey, 0)
	}

	if out != nil {
		err = s.decoder.Decode(data, out, txnResp.Revision)
		if err != nil {
			span.AddEvent("decode failed", attribute.Int("len", len(data)), attribute.String("err", err.Error()))
			recordDecodeError(s.groupResource, preparedKey)
			return err
		}
		span.AddEvent("decode succeeded", attribute.Int("len", len(data)))
	}
	return nil
}

// Delete implements storage.Interface.Delete.
func (s *store) Delete(
	ctx context.Context, key string, out runtime.Object, preconditions *storage.Preconditions,
	validateDeletion storage.ValidateObjectFunc, cachedExistingObject runtime.Object, opts storage.DeleteOptions) error {
	preparedKey, err := s.prepareKey(key, false)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(out)
	if err != nil {
		return fmt.Errorf("unable to convert output object to pointer: %v", err)
	}

	expectTransformOrDecodeError := false
	if utilfeature.DefaultFeatureGate.Enabled(features.AllowUnsafeMalformedObjectDeletion) {
		expectTransformOrDecodeError = opts.ExpectTransformOrDecodeError
	}
	return s.conditionalDelete(ctx, preparedKey, out, v, preconditions, validateDeletion, cachedExistingObject, expectTransformOrDecodeError)
}

func (s *store) conditionalDelete(
	ctx context.Context, key string, out runtime.Object, v reflect.Value, preconditions *storage.Preconditions,
	validateDeletion storage.ValidateObjectFunc, cachedExistingObject runtime.Object, expectTransformOrDecodeError bool) error {
	getCurrentState := s.getCurrentState(ctx, key, v, false, expectTransformOrDecodeError)

	var origState *objState
	var err error
	var origStateIsCurrent bool
	if cachedExistingObject != nil && !expectTransformOrDecodeError {
		origState, err = s.getStateFromObject(cachedExistingObject)
	} else {
		origState, err = getCurrentState()
		origStateIsCurrent = true
	}
	if err != nil {
		return err
	}

	for {
		if preconditions != nil {
			if err := preconditions.Check(key, origState.obj); err != nil {
				if origStateIsCurrent {
					return err
				}

				// It's possible we're working with stale data.
				// Remember the revision of the potentially stale data and the resulting update error
				cachedRev := origState.rev
				cachedUpdateErr := err

				// Actually fetch
				origState, err = getCurrentState()
				if err != nil {
					return err
				}
				origStateIsCurrent = true

				// it turns out our cached data was not stale, return the error
				if cachedRev == origState.rev {
					return cachedUpdateErr
				}

				// Retry
				continue
			}
		}
		if err := validateDeletion(ctx, origState.obj); err != nil {
			if origStateIsCurrent {
				return err
			}

			// It's possible we're working with stale data.
			// Remember the revision of the potentially stale data and the resulting update error
			cachedRev := origState.rev
			cachedUpdateErr := err

			// Actually fetch
			origState, err = getCurrentState()
			if err != nil {
				return err
			}
			origStateIsCurrent = true

			// it turns out our cached data was not stale, return the error
			if cachedRev == origState.rev {
				return cachedUpdateErr
			}

			// Retry
			continue
		}

		startTime := time.Now()
		txnResp, err := s.client.Kubernetes.OptimisticDelete(ctx, key, origState.rev, kubernetes.DeleteOptions{
			GetOnFailure: true,
		})
		metrics.RecordEtcdRequest("delete", s.groupResource, err, startTime)
		if err != nil {
			return err
		}
		if !txnResp.Succeeded {
			klog.V(4).Infof("deletion of %s failed because of a conflict, going to retry", key)
			origState, err = s.getState(ctx, txnResp.KV, key, v, false, expectTransformOrDecodeError)
			if err != nil {
				return err
			}
			origStateIsCurrent = true
			continue
		}

		if !expectTransformOrDecodeError {
			err = s.decoder.Decode(origState.data, out, txnResp.Revision)
			if err != nil {
				recordDecodeError(s.groupResource, key)
				return err
			}
		}
		return nil
	}
}

// GuaranteedUpdate implements storage.Interface.GuaranteedUpdate.
func (s *store) GuaranteedUpdate(
	ctx context.Context, key string, destination runtime.Object, ignoreNotFound bool,
	preconditions *storage.Preconditions, tryUpdate storage.UpdateFunc, cachedExistingObject runtime.Object) error {
	preparedKey, err := s.prepareKey(key, false)
	if err != nil {
		return err
	}
	ctx, span := tracing.Start(ctx, "GuaranteedUpdate etcd3",
		attribute.String("audit-id", audit.GetAuditIDTruncated(ctx)),
		attribute.String("key", key),
		attribute.String("type", getTypeName(destination)),
		attribute.String("group", s.groupResource.Group),
		attribute.String("resource", s.groupResource.Resource))
	defer span.End(500 * time.Millisecond)

	v, err := conversion.EnforcePtr(destination)
	if err != nil {
		return fmt.Errorf("unable to convert output object to pointer: %v", err)
	}

	getCurrentState := s.getCurrentState(ctx, preparedKey, v, ignoreNotFound, false)

	var origState *objState
	var origStateIsCurrent bool
	if cachedExistingObject != nil {
		origState, err = s.getStateFromObject(cachedExistingObject)
	} else {
		origState, err = getCurrentState()
		origStateIsCurrent = true
	}
	if err != nil {
		return err
	}
	span.AddEvent("initial value restored")

	transformContext := authenticatedDataString(preparedKey)
	for {
		if err := preconditions.Check(preparedKey, origState.obj); err != nil {
			// If our data is already up to date, return the error
			if origStateIsCurrent {
				return err
			}

			// It's possible we were working with stale data
			// Actually fetch
			origState, err = getCurrentState()
			if err != nil {
				return err
			}
			origStateIsCurrent = true
			// Retry
			continue
		}

		ret, ttl, err := s.updateState(origState, tryUpdate)
		if err != nil {
			// If our data is already up to date, return the error
			if origStateIsCurrent {
				return err
			}

			// It's possible we were working with stale data
			// Remember the revision of the potentially stale data and the resulting update error
			cachedRev := origState.rev
			cachedUpdateErr := err

			// Actually fetch
			origState, err = getCurrentState()
			if err != nil {
				return err
			}
			origStateIsCurrent = true

			// it turns out our cached data was not stale, return the error
			if cachedRev == origState.rev {
				return cachedUpdateErr
			}

			// Retry
			continue
		}

		span.AddEvent("About to Encode")
		data, err := runtime.Encode(s.codec, ret)
		if err != nil {
			span.AddEvent("Encode failed", attribute.Int("len", len(data)), attribute.String("err", err.Error()))
			return err
		}
		span.AddEvent("Encode succeeded", attribute.Int("len", len(data)))
		if !origState.stale && bytes.Equal(data, origState.data) {
			// if we skipped the original Get in this loop, we must refresh from
			// etcd in order to be sure the data in the store is equivalent to
			// our desired serialization
			if !origStateIsCurrent {
				origState, err = getCurrentState()
				if err != nil {
					return err
				}
				origStateIsCurrent = true
				if !bytes.Equal(data, origState.data) {
					// original data changed, restart loop
					continue
				}
			}
			// recheck that the data from etcd is not stale before short-circuiting a write
			if !origState.stale {
				err = s.decoder.Decode(origState.data, destination, origState.rev)
				if err != nil {
					recordDecodeError(s.groupResource, preparedKey)
					return err
				}
				return nil
			}
		}

		newData, err := s.transformer.TransformToStorage(ctx, data, transformContext)
		if err != nil {
			span.AddEvent("TransformToStorage failed", attribute.String("err", err.Error()))
			return storage.NewInternalError(err)
		}
		span.AddEvent("TransformToStorage succeeded")

		var lease clientv3.LeaseID
		if ttl != 0 {
			lease, err = s.leaseManager.GetLease(ctx, int64(ttl))
			if err != nil {
				return err
			}
		}
		span.AddEvent("Transaction prepared")

		startTime := time.Now()

		txnResp, err := s.client.Kubernetes.OptimisticPut(ctx, preparedKey, newData, origState.rev, kubernetes.PutOptions{
			GetOnFailure: true,
			LeaseID:      lease,
		})
		metrics.RecordEtcdRequest("update", s.groupResource, err, startTime)
		if err != nil {
			span.AddEvent("Txn call failed", attribute.String("err", err.Error()))
			return err
		}
		span.AddEvent("Txn call completed")
		span.AddEvent("Transaction committed")
		if !txnResp.Succeeded {
			klog.V(4).Infof("GuaranteedUpdate of %s failed because of a conflict, going to retry", preparedKey)
			origState, err = s.getState(ctx, txnResp.KV, preparedKey, v, ignoreNotFound, false)
			if err != nil {
				return err
			}
			span.AddEvent("Retry value restored")
			origStateIsCurrent = true
			continue
		}

		err = s.decoder.Decode(data, destination, txnResp.Revision)
		if err != nil {
			span.AddEvent("decode failed", attribute.Int("len", len(data)), attribute.String("err", err.Error()))
			recordDecodeError(s.groupResource, preparedKey)
			return err
		}
		span.AddEvent("decode succeeded", attribute.Int("len", len(data)))
		return nil
	}
}

func getNewItemFunc(listObj runtime.Object, v reflect.Value) func() runtime.Object {
	// For unstructured lists with a target group/version, preserve the group/version in the instantiated list items
	if unstructuredList, isUnstructured := listObj.(*unstructured.UnstructuredList); isUnstructured {
		if apiVersion := unstructuredList.GetAPIVersion(); len(apiVersion) > 0 {
			return func() runtime.Object {
				return &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": apiVersion}}
			}
		}
	}

	// Otherwise just instantiate an empty item
	elem := v.Type().Elem()
	return func() runtime.Object {
		return reflect.New(elem).Interface().(runtime.Object)
	}
}

func (s *store) Stats(ctx context.Context) (storage.Stats, error) {
	if collector := s.getResourceSizeEstimator(); collector != nil {
		return collector.Stats(ctx)
	}
	// returning stats without resource size

	startTime := time.Now()
	prefix, err := s.prepareKey(s.resourcePrefix, true)
	if err != nil {
		return storage.Stats{}, err
	}
	count, err := s.client.Kubernetes.Count(ctx, prefix, kubernetes.CountOptions{})
	metrics.RecordEtcdRequest("listWithCount", s.groupResource, err, startTime)
	if err != nil {
		return storage.Stats{}, err
	}
	return storage.Stats{
		ObjectCount: count,
	}, nil
}

func (s *store) EnableResourceSizeEstimation(getKeys storage.KeysFunc) error {
	if getKeys == nil {
		return errors.New("KeysFunc cannot be nil")
	}
	s.collectorMux.Lock()
	defer s.collectorMux.Unlock()
	if s.resourceSizeEstimator != nil {
		return errors.New("resourceSizeEstimator already enabled")
	}
	s.resourceSizeEstimator = newResourceSizeEstimator(s.pathPrefix, getKeys)
	return nil
}

// TestOnlyResetResourceSizeEstimator clears the resource size estimator so a
// subsequent EnableResourceSizeEstimation call succeeds.
func TestOnlyResetResourceSizeEstimator(s storage.Interface) {
	st, ok := s.(*store)
	if !ok {
		return
	}
	st.collectorMux.Lock()
	defer st.collectorMux.Unlock()
	if st.resourceSizeEstimator != nil {
		st.resourceSizeEstimator.Close()
		st.resourceSizeEstimator = nil
	}
}

func (s *store) getKeys(ctx context.Context) ([]string, error) {
	startTime := time.Now()
	prefix, err := s.prepareKey(s.resourcePrefix, true)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.KV.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	metrics.RecordEtcdRequest("listOnlyKeys", s.groupResource, err, startTime)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}
	return keys, nil
}

// ReadinessCheck implements storage.Interface.
func (s *store) ReadinessCheck() error {
	return nil
}

func (s *store) GetCurrentResourceVersion(ctx context.Context) (uint64, error) {
	preparedKey, err := s.prepareKey(s.resourcePrefix, false)
	if err != nil {
		return 0, err
	}

	startTime := time.Now()
	getResp, err := s.client.Kubernetes.Get(ctx, preparedKey, kubernetes.GetOptions{})
	metrics.RecordEtcdRequest("getCurrentResourceVersion", s.groupResource, err, startTime)
	if err != nil {
		return 0, err
	}

	if getResp.Revision == 0 {
		return 0, fmt.Errorf("the current resource version must be greater than 0")
	}
	return uint64(getResp.Revision), nil
}

// GetList implements storage.Interface.
func (s *store) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	keyPrefix, err := s.prepareKey(key, opts.Recursive)
	if err != nil {
		return err
	}
	ctx, span := tracing.Start(ctx, fmt.Sprintf("List(recursive=%v) etcd3", opts.Recursive),
		attribute.String("audit-id", audit.GetAuditIDTruncated(ctx)),
		attribute.String("key", key),
		attribute.String("resourceVersion", opts.ResourceVersion),
		attribute.String("resourceVersionMatch", string(opts.ResourceVersionMatch)),
		attribute.Int("limit", int(opts.Predicate.Limit)),
		attribute.String("continue", opts.Predicate.Continue))
	defer span.End(500 * time.Millisecond)
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return fmt.Errorf("need ptr to slice: %v", err)
	}

	// set the appropriate clientv3 options to filter the returned data set
	limit := opts.Predicate.Limit
	paging := opts.Predicate.Limit > 0
	newItemFunc := getNewItemFunc(listObj, v)

	withRev, continueKey, err := storage.ValidateListOptions(keyPrefix, s.versioner, opts)
	if err != nil {
		return err
	}

	// loop until we have filled the requested limit from etcd or there are no more results
	var lastKey []byte
	var hasMore bool
	var getResp kubernetes.ListResponse
	var numFetched int
	var numEvald int
	// Because these metrics are for understanding the costs of handling LIST requests,
	// get them recorded even in error cases.
	defer func() {
		numReturn := v.Len()
		metrics.RecordStorageListMetrics(s.groupResource, "", numFetched, numEvald, numReturn)
	}()

	aggregator := s.listErrAggrFactory()
	for {
		getResp, err = s.getList(ctx, keyPrefix, opts.Recursive, kubernetes.ListOptions{
			Revision: withRev,
			Limit:    limit,
			Continue: continueKey,
		})
		if err != nil {
			if errors.Is(err, etcdrpc.ErrFutureRev) {
				currentRV, getRVErr := s.GetCurrentResourceVersion(ctx)
				if getRVErr != nil {
					// If we can't get the current RV, use 0 as a fallback.
					currentRV = 0
				}
				return storage.NewTooLargeResourceVersionError(uint64(withRev), currentRV, 0)
			}
			return interpretListError(err, len(opts.Predicate.Continue) > 0, continueKey, keyPrefix)
		}
		numFetched += len(getResp.Kvs)
		if err = s.validateMinimumResourceVersion(opts.ResourceVersion, uint64(getResp.Revision)); err != nil {
			return err
		}
		hasMore = int64(len(getResp.Kvs)) < getResp.Count

		if len(getResp.Kvs) == 0 && hasMore {
			return fmt.Errorf("no results were found, but etcd indicated there were more values remaining")
		}
		// indicate to the client which resource version was returned, and use the same resource version for subsequent requests.
		if withRev == 0 {
			withRev = getResp.Revision
		}

		// avoid small allocations for the result slice, since this can be called in many
		// different contexts and we don't know how significantly the result will be filtered
		if opts.Predicate.Empty() {
			growSlice(v, len(getResp.Kvs))
		} else {
			growSlice(v, 2048, len(getResp.Kvs))
		}

		// take items from the response until the bucket is full, filtering as we go
		for i, kv := range getResp.Kvs {
			if paging && int64(v.Len()) >= opts.Predicate.Limit {
				hasMore = true
				break
			}
			lastKey = kv.Key
			evaluated, err := s.processListItem(ctx, kv, opts.Predicate, newItemFunc, aggregator, v)
			if err != nil {
				return err
			}
			if evaluated {
				numEvald++
			}
			// free kv early. Long lists can take O(seconds) to decode.
			getResp.Kvs[i] = nil
		}
		continueKey = string(lastKey) + "\x00"

		// no more results remain or we didn't request paging
		if !hasMore || !paging {
			break
		}
		// we're paging but we have filled our bucket
		if int64(v.Len()) >= opts.Predicate.Limit {
			break
		}

		if limit < maxLimit {
			// We got incomplete result due to field/label selector dropping the object.
			// Double page size to reduce total number of calls to etcd.
			limit *= 2
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	if err := aggregator.Err(); err != nil {
		return err
	}

	if v.IsNil() {
		// Ensure that we never return a nil Items pointer in the result for consistency.
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}

	continueValue, remainingItemCount, err := storage.PrepareContinueToken(string(lastKey), keyPrefix, withRev, getResp.Count, hasMore, opts)
	if err != nil {
		return err
	}
	if err := s.versioner.UpdateList(listObj, uint64(withRev), continueValue, remainingItemCount); err != nil {
		return err
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.ShardedListAndWatch) {
		opts.Predicate.SetShardInfoOnList(listObj)
	}
	return nil
}

func (s *store) processListItem(ctx context.Context, kv *mvccpb.KeyValue, pred storage.SelectionPredicate, newItemFunc func() runtime.Object, aggregator ListErrorAggregator, v reflect.Value) (bool, error) {
	data, _, err := s.transformer.TransformFromStorage(ctx, kv.Value, authenticatedDataString(kv.Key))
	if err != nil {
		if done := aggregator.Aggregate(string(kv.Key), storage.NewInternalError(fmt.Errorf("unable to transform key %q: %w", kv.Key, err))); done {
			return false, aggregator.Err()
		}
		return false, nil
	}

	// Check if the request has already timed out before decode object
	select {
	case <-ctx.Done():
		// parent context is canceled or timed out, no point in continuing
		return false, storage.NewTimeoutError(string(kv.Key), "request did not complete within requested timeout")
	default:
	}

	obj, err := s.decoder.DecodeListItem(ctx, data, uint64(kv.ModRevision), newItemFunc)
	if err != nil {
		recordDecodeError(s.groupResource, string(kv.Key))
		if done := aggregator.Aggregate(string(kv.Key), err); done {
			return false, aggregator.Err()
		}
		return false, nil
	}

	// being unable to set the version does not prevent the object from being extracted
	if matched, err := pred.Matches(obj); err == nil && matched {
		v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
	}

	return true, nil
}

func (s *store) getList(ctx context.Context, keyPrefix string, recursive bool, options kubernetes.ListOptions) (resp kubernetes.ListResponse, err error) {
	startTime := time.Now()
	if recursive {
		resp, err = s.client.Kubernetes.List(ctx, keyPrefix, options)
		metrics.RecordEtcdRequest("list", s.groupResource, err, startTime)
	} else {
		var getResp kubernetes.GetResponse
		getResp, err = s.client.Kubernetes.Get(ctx, keyPrefix, kubernetes.GetOptions{
			Revision: options.Revision,
		})
		metrics.RecordEtcdRequest("get", s.groupResource, err, startTime)
		if getResp.KV != nil {
			resp.Kvs = []*mvccpb.KeyValue{getResp.KV}
			resp.Count = 1
			resp.Revision = getResp.Revision
		} else {
			resp.Kvs = []*mvccpb.KeyValue{}
			resp.Count = 0
			resp.Revision = getResp.Revision
		}
	}

	stats := s.getResourceSizeEstimator()
	if len(resp.Kvs) > 0 && stats != nil {
		stats.Update(resp.Kvs)
	}
	return resp, err
}

// growSlice takes a slice value and grows its capacity up
// to the maximum of the passed sizes or maxCapacity, whichever
// is smaller. Above maxCapacity decisions about allocation are left
// to the Go runtime on append. This allows a caller to make an
// educated guess about the potential size of the total list while
// still avoiding overly aggressive initial allocation. If sizes
// is empty maxCapacity will be used as the size to grow.
func growSlice(v reflect.Value, maxCapacity int, sizes ...int) {
	cap := v.Cap()
	max := cap
	for _, size := range sizes {
		if size > max {
			max = size
		}
	}
	if len(sizes) == 0 || max > maxCapacity {
		max = maxCapacity
	}
	if max <= cap {
		return
	}
	if v.Len() > 0 {
		extra := reflect.MakeSlice(v.Type(), v.Len(), max)
		reflect.Copy(extra, v)
		v.Set(extra)
	} else {
		extra := reflect.MakeSlice(v.Type(), 0, max)
		v.Set(extra)
	}
}

// Watch implements storage.Interface.Watch.
func (s *store) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	preparedKey, err := s.prepareKey(key, opts.Recursive)
	if err != nil {
		return nil, err
	}
	rev, err := s.versioner.ParseResourceVersion(opts.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return s.watcher.Watch(s.watchContext(ctx), preparedKey, int64(rev), opts)
}

func (s *store) watchContext(ctx context.Context) context.Context {
	// The etcd server waits until it cannot find a leader for 3 election
	// timeouts to cancel existing streams. 3 is currently a hard coded
	// constant. The election timeout defaults to 1000ms. If the cluster is
	// healthy, when the leader is stopped, the leadership transfer should be
	// smooth. (leader transfers its leadership before stopping). If leader is
	// hard killed, other servers will take an election timeout to realize
	// leader lost and start campaign.
	return clientv3.WithRequireLeader(ctx)
}

func (s *store) getCurrentState(ctx context.Context, key string, v reflect.Value, ignoreNotFound bool, expectTransformOrDecodeError bool) func() (*objState, error) {
	return func() (*objState, error) {
		startTime := time.Now()
		getResp, err := s.client.Kubernetes.Get(ctx, key, kubernetes.GetOptions{})
		metrics.RecordEtcdRequest("get", s.groupResource, err, startTime)
		if err != nil {
			return nil, err
		}
		return s.getState(ctx, getResp.KV, key, v, ignoreNotFound, expectTransformOrDecodeError)
	}
}

// getState constructs a new objState from the given response from the storage. If
// expectTransformOrDecodeError is true and neither transformation nor decode fails, returns an
// InvalidObj error; if either fails, the returned error and the 'obj' field of the returned
// objState will both be nil.
func (s *store) getState(ctx context.Context, kv *mvccpb.KeyValue, key string, v reflect.Value, ignoreNotFound bool, expectTransformOrDecodeError bool) (*objState, error) {
	state := &objState{
		meta: &storage.ResponseMeta{},
	}

	if u, ok := v.Addr().Interface().(runtime.Unstructured); ok {
		state.obj = u.NewEmptyInstance()
	} else {
		state.obj = reflect.New(v.Type()).Interface().(runtime.Object)
	}

	if kv == nil {
		if !ignoreNotFound {
			return nil, storage.NewKeyNotFoundError(key, 0)
		}
		if err := runtime.SetZeroValue(state.obj); err != nil {
			return nil, err
		}
		return state, nil
	}

	state.rev = kv.ModRevision
	state.meta.ResourceVersion = uint64(state.rev)

	data, stale, err := s.transformer.TransformFromStorage(ctx, kv.Value, authenticatedDataString(key))
	if err != nil {
		if !expectTransformOrDecodeError {
			return nil, storage.NewInternalError(err)
		}

		// be explicit that we don't have the object
		state.obj = nil
		state.stale = true // this seems a more sane value here
		return state, nil
	}

	state.data = data
	state.stale = stale

	if err := s.decoder.Decode(state.data, state.obj, state.rev); err != nil {
		if !expectTransformOrDecodeError {
			recordDecodeError(s.groupResource, key)
			return nil, err
		}

		// be explicit that we don't have the object
		state.obj = nil
		return state, nil
	}

	if expectTransformOrDecodeError {
		return nil, storage.NewInvalidObjError(key, "unsafe deletion is not allowed because the object is decodable from storage")
	}

	return state, nil
}

func (s *store) getStateFromObject(obj runtime.Object) (*objState, error) {
	state := &objState{
		obj:  obj,
		meta: &storage.ResponseMeta{},
	}

	rv, err := s.versioner.ObjectResourceVersion(obj)
	if err != nil {
		return nil, fmt.Errorf("couldn't get resource version: %v", err)
	}
	state.rev = int64(rv)
	state.meta.ResourceVersion = uint64(state.rev)

	// Compute the serialized form - for that we need to temporarily clean
	// its resource version field (those are not stored in etcd).
	if err := s.versioner.PrepareObjectForStorage(obj); err != nil {
		return nil, fmt.Errorf("PrepareObjectForStorage failed: %v", err)
	}
	state.data, err = runtime.Encode(s.codec, obj)
	if err != nil {
		return nil, err
	}
	if err := s.versioner.UpdateObject(state.obj, uint64(rv)); err != nil {
		klog.Errorf("failed to update object version: %v", err)
	}
	return state, nil
}

func (s *store) updateState(st *objState, userUpdate storage.UpdateFunc) (runtime.Object, uint64, error) {
	ret, ttlPtr, err := userUpdate(st.obj, *st.meta)
	if err != nil {
		return nil, 0, err
	}

	if err := s.versioner.PrepareObjectForStorage(ret); err != nil {
		return nil, 0, fmt.Errorf("PrepareObjectForStorage failed: %v", err)
	}
	var ttl uint64
	if ttlPtr != nil {
		ttl = *ttlPtr
	}
	return ret, ttl, nil
}

// validateMinimumResourceVersion returns a 'too large resource' version error when the provided minimumResourceVersion is
// greater than the most recent actualRevision available from storage.
func (s *store) validateMinimumResourceVersion(minimumResourceVersion string, actualRevision uint64) error {
	if minimumResourceVersion == "" {
		return nil
	}
	minimumRV, err := s.versioner.ParseResourceVersion(minimumResourceVersion)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("invalid resource version: %v", err))
	}
	// Enforce the storage.Interface guarantee that the resource version of the returned data
	// "will be at least 'resourceVersion'".
	if minimumRV > actualRevision {
		return storage.NewTooLargeResourceVersionError(minimumRV, actualRevision, 0)
	}
	return nil
}

func (s *store) prepareKey(key string, recursive bool) (string, error) {
	key, err := storage.PrepareKey(s.resourcePrefix, key, recursive)
	if err != nil {
		return "", err
	}
	// We ensured that pathPrefix ends in '/' in construction, so skip any leading '/' in the key now.
	startIndex := 0
	if key[0] == '/' {
		startIndex = 1
	}
	return s.pathPrefix + key[startIndex:], nil
}

// recordDecodeError record decode error split by object type.
func recordDecodeError(groupResource schema.GroupResource, key string) {
	metrics.RecordDecodeError(groupResource)
	klog.V(4).Infof("Decoding %s \"%s\" failed", groupResource, key)
}

// getTypeName returns type name of an object for reporting purposes.
func getTypeName(obj interface{}) string {
	return reflect.TypeOf(obj).String()
}

```

