import os
import re

files_to_modify = [
    r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime_capabilities_routing_test.go',
    r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime_capabilities_test.go',
    r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime_runner_test.go',
    r'd:\Users\alice\git\discordcore\pkg\app\bot_runtime_runner_warmup_test.go',
    r'd:\Users\alice\git\discordcore\pkg\app\runner_rollback_test.go',
    r'd:\Users\alice\git\discordcore\pkg\app\runner_run_test.go',
    r'd:\Users\alice\git\discordcore\pkg\app\task_router_budget_test.go',
    r'd:\Users\alice\git\discordcore\pkg\control\settings_routes_test.go',
    r'd:\Users\alice\git\discordcore\pkg\discord\commands\handler_lifecycle_test.go',
    r'd:\Users\alice\git\discordcore\pkg\discord\commands\handler_route_test.go',
    r'd:\Users\alice\git\discordcore\pkg\discord\commands\core\manager_lifecycle_test.go',
    r'd:\Users\alice\git\discordcore\pkg\discord\commands\moderation\reaction_block_commands_test.go',
    r'd:\Users\alice\git\discordcore\pkg\discord\qotd\runtime_service_test.go',
    r'd:\Users\alice\git\discordcore\pkg\files\legacy_guild_config_test.go',
    r'd:\Users\alice\git\discordcore\pkg\files\legacy_moderation_migration_test.go',
    r'd:\Users\alice\git\discordcore\pkg\monitoring\monitoring_guild_create_test.go',
    r'd:\Users\alice\git\discordcore\pkg\monitoring\runtime_activity_test.go',
    r'd:\Users\alice\git\discordcore\pkg\stats\service_routing_test.go',
    r'd:\Users\alice\git\discordcore\pkg\storage\postgres_store_additional_test.go'
]

for filepath in files_to_modify:
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    # Replace "main" with "generic"
    content = re.sub(r'"main"', '"generic"', content)
    # Replace "custom" with "generic"
    content = re.sub(r'"custom"', '"generic"', content)
    # Fix duplicate map keys: {"generic": "a", "generic": "s"} -> {"generic": "a"}
    content = re.sub(r'{"generic":\s*"[^"]+",\s*"generic":\s*"[^"]+"}', r'{"generic": "a"}', content)
    
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)
