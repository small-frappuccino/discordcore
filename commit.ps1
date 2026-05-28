$ErrorActionPreference = "Stop"

Write-Host "Committing M5..."
release -m "refactor(log): extract log policy to decouple from control" -y pkg/logpolicy/ pkg/discord/logging/event_policy.go pkg/discord/logging/event_policy_test.go pkg/discord/logging/automod.go pkg/discord/logging/automod_test.go pkg/discord/logging/member_events.go pkg/discord/logging/message_events.go pkg/discord/logging/moderation_logging.go pkg/discord/logging/reaction_events.go pkg/discord/maintenance/user_prune.go pkg/control/features_catalog.go pkg/control/features_helpers.go pkg/control/features_readiness.go pkg/control/feature_workspace_builder.go pkg/discord/commands/moderation/clean_command.go pkg/util/application.go

Write-Host "Committing M6..."
release -m "refactor(qotd): remove storage types leakage in control layer" -y pkg/control/qotd_routes.go pkg/control/qotd_workspace.go pkg/qotd/types.go pkg/qotd/official_post_url.go pkg/util/application.go

Write-Host "Committing m7..."
release -m "refactor(config): move deprecated types to unmarshal structs" -y pkg/files/types.go pkg/util/application.go

Write-Host "Committing M3..."
release -m "refactor(commands): decompose moderation commands" -y pkg/discord/commands/moderation/ pkg/util/application.go

Write-Host "Committing m1, M2..."
release -m "refactor(storage): decompose qotd store and remove nil checks" -y pkg/storage/ pkg/util/application.go

Write-Host "Committing M1, m10..."
release -m "refactor(monitoring): decompose backfill and enforce strong typing on router" -y pkg/discord/logging/monitoring.go pkg/discord/logging/monitoring_backfill.go pkg/discord/logging/monitoring_backfill_test.go pkg/discord/logging/monitoring_runtime_toggle.go pkg/discord/logging/monitoring_user_events.go pkg/task/router.go pkg/task/adapters.go pkg/util/application.go

Write-Host "Committing M4..."
release -m "refactor(qotd): decompose service" -y pkg/qotd/service.go pkg/qotd/service_settings.go pkg/qotd/service_questions.go pkg/qotd/service_publish.go pkg/qotd/service_reconcile.go pkg/qotd/service_helpers.go pkg/util/application.go

Write-Host "Committing M7..."
release -m "refactor(ui): decompose RolesPage drawers" -y ui/src/pages/RolesPage.tsx ui/src/pages/roles/ pkg/util/application.go

Write-Host "Committing Stylistic changes..."
release -m "refactor(style): stylistic improvements and magic id removal" -y pkg/discord/commands/core/utils.go pkg/discord/commands/runtime/runtime_config_commands.go pkg/discord/cache/unified_cache.go pkg/files/custom_embed_config.go pkg/files/custom_embed_manager.go pkg/files/features.go pkg/files/preferences.go pkg/util/application.go

Write-Host "All commits done."
