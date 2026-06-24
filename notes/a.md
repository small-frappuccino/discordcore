?   	github.com/small-frappuccino/discordcore/cmd/clean-config	[no test files]
?   	github.com/small-frappuccino/discordcore/cmd/discordcore	[no test files]
?   	github.com/small-frappuccino/discordcore/cmd/tsgen	[no test files]
=== RUN   TestBotRuntime_InitializationRouting
=== RUN   TestBotRuntime_InitializationRouting/Exhaustive_Mocking:_All_Features_Enabled
2026/06/23 22:07:42 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:42 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:42 INFO Logging bot runtime capability activated guild_id=g1 bot_instance_id=main intents_mask=2131459
2026/06/23 22:07:42 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=main concurrency_budget=8
2026/06/23 22:07:42 INFO Service registered service=messages type=messages priority=5 dependencies=[]
2026/06/23 22:07:42 INFO Service registered service=member_events_main type=monitoring priority=5 dependencies=[]
2026/06/23 22:07:42 INFO Service registered service=discord_automod_adapter type=automod priority=5 dependencies=[]
2026/06/23 22:07:42 INFO Architectural state transition: QOTD runtime initialized botInstanceID=main
2026/06/23 22:07:42 INFO Service registered service=qotd type=monitoring priority=5 dependencies=[]
2026/06/23 22:07:42 INFO Registered DiscordGo event handlers for stats
2026/06/23 22:07:42 INFO Service registered service=stats type=monitoring priority=1 dependencies=[]
2026/06/23 22:07:42 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:42 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=main
2026/06/23 22:07:42 INFO Stopping all services...
2026/06/23 22:07:42 INFO All services stopped successfully
=== RUN   TestBotRuntime_InitializationRouting/Routing_Disabled_Features_Yields_Idle_Core
2026/06/23 22:07:42 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:42 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:42 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=main concurrency_budget=8
2026/06/23 22:07:42 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=main
2026/06/23 22:07:42 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=main
2026/06/23 22:07:42 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=main
2026/06/23 22:07:42 INFO Stopping all services...
2026/06/23 22:07:42 INFO All services stopped successfully
--- PASS: TestBotRuntime_InitializationRouting (0.00s)
    --- PASS: TestBotRuntime_InitializationRouting/Exhaustive_Mocking:_All_Features_Enabled (0.00s)
    --- PASS: TestBotRuntime_InitializationRouting/Routing_Disabled_Features_Yields_Idle_Core (0.00s)
=== RUN   TestBotRuntime_CapabilityBitmaskDerivation
=== PAUSE TestBotRuntime_CapabilityBitmaskDerivation
=== RUN   TestBotRuntimeResolver_ConcurrentMemoryRotation
=== PAUSE TestBotRuntimeResolver_ConcurrentMemoryRotation
=== RUN   TestBotRuntimeResolver_WaitBarrierOrchestration
2026/06/23 22:07:42 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:42 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:42 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
--- PASS: TestBotRuntimeResolver_WaitBarrierOrchestration (0.07s)
=== RUN   TestSupervisorFaultIsolation
2026/06/23 22:07:42 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:42 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:42 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=3
2026/06/23 22:07:42 INFO Architectural state transition: Background worker pool initialized parallelism_limit=4 queue_capacity=6
2026/06/23 22:07:42 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=3
2026/06/23 22:07:42 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:42 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/23 22:07:42 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child3
2026/06/23 22:07:42 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/23 22:07:42 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/23 22:07:42 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/23 22:07:42 WARN Instance authentication compromised, triggering token revocation botInstanceID=child3 error="open discord session for child3: HTTP 401 Unauthorized"
2026/06/23 22:07:42 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/23 22:07:42 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:42 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:42 INFO Starting service... service=bot-runtime-child1
2026/06/23 22:07:42 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:42 INFO Starting all services...
2026/06/23 22:07:42 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child2
2026/06/23 22:07:42 INFO Starting service... service=command-handler
2026/06/23 22:07:42 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:42 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:42 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:42 INFO Service started successfully service=command-handler
2026/06/23 22:07:42 INFO All services started successfully services_count=1
2026/06/23 22:07:42 INFO Service started successfully service=bot-runtime-child1
2026/06/23 22:07:42 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:42 INFO Planned instance shutdown triggered by token removal botInstanceID=child3
2026/06/23 22:07:42 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/23 22:07:42 INFO executeStopAndRemove DELETING instance id=child3
2026/06/23 22:07:42 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:42 INFO Stopping service... service=bot-runtime-child1
2026/06/23 22:07:42 INFO executeStopAndRemove DELETING instance id=child2
2026/06/23 22:07:42 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=child1
2026/06/23 22:07:42 INFO Stopping all services...
2026/06/23 22:07:42 INFO Stopping service... service=command-handler
2026/06/23 22:07:42 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:42 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:42 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:42 INFO All services stopped successfully
2026/06/23 22:07:42 INFO Service stopped service=bot-runtime-child1
2026/06/23 22:07:42 INFO executeStopAndRemove DELETING instance id=child1
2026/06/23 22:07:42 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
--- PASS: TestSupervisorFaultIsolation (0.72s)
=== RUN   TestZeroStateIdling
2026/06/23 22:07:43 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:43 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:43 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:43 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/23 22:07:43 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/23 22:07:43 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/23 22:07:43 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:43 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/23 22:07:43 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/23 22:07:43 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:43 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestZeroStateIdling (0.05s)
=== RUN   TestSupervisorSwarmTopology
2026/06/23 22:07:43 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:43 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:43 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=10
2026/06/23 22:07:43 INFO Architectural state transition: Background worker pool initialized parallelism_limit=4 queue_capacity=20
2026/06/23 22:07:43 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=10
2026/06/23 22:07:43 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:43 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childE
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childE botUser=test#
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childJ
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childE concurrency_budget=8
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childE
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childJ botUser=test#
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childJ concurrency_budget=8
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childJ
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childE
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childF
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childF botUser=test#
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childJ
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childF concurrency_budget=8
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childF
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childC
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childC botUser=test#
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childC concurrency_budget=8
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childC
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childF
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childC
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childE
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childB
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childB botUser=test#
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childD
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childJ
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childB concurrency_budget=8
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childD botUser=test#
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childD concurrency_budget=8
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childD
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childF
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childB
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childC
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childD
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childB
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childA
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childH
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childH botUser=test#
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childA botUser=test#
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childH concurrency_budget=8
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childA concurrency_budget=8
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childH
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childB
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childA
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childD
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childH
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childG
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childA
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childG botUser=test#
2026/06/23 22:07:43 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childI
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childG concurrency_budget=8
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childG
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childI botUser=test#
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childH
2026/06/23 22:07:43 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childI concurrency_budget=8
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childA
2026/06/23 22:07:43 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childI
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childG
2026/06/23 22:07:43 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:43 INFO Starting service... service=bot-runtime-childI
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Starting all services...
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO Starting service... service=command-handler
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childG
2026/06/23 22:07:43 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:43 INFO Service started successfully service=command-handler
2026/06/23 22:07:43 INFO All services started successfully services_count=1
2026/06/23 22:07:43 INFO Service started successfully service=bot-runtime-childI
2026/06/23 22:07:43 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childE
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childD
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childG
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childI
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childE
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childA
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childD
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childC
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childB
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childI
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childF
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childC
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childA
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childF
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childB
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childH
2026/06/23 22:07:43 INFO Stopping service... service=bot-runtime-childJ
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childG
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childJ
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childH
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childD
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childD
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childE
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childE
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO Stopping all services...
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childC
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childA
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childC
2026/06/23 22:07:43 INFO Stopping service... service=command-handler
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childG
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childA
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childJ
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childG
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childJ
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO All services stopped successfully
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childI
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childI
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childB
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childH
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childB
2026/06/23 22:07:43 INFO Service stopped service=bot-runtime-childF
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childH
2026/06/23 22:07:43 INFO executeStopAndRemove DELETING instance id=childF
2026/06/23 22:07:43 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:43 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:43 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:44 WARN Handshake failure with guild interface reported by central hub guildID=g1
--- PASS: TestSupervisorSwarmTopology (1.01s)
=== RUN   TestSupervisorConfigChange
2026/06/23 22:07:44 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:44 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:44 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/23 22:07:44 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/23 22:07:44 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:44 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/23 22:07:44 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:44 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-child1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Starting service... service=command-handler
2026/06/23 22:07:44 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:44 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:44 INFO Service started successfully service=command-handler
2026/06/23 22:07:44 INFO All services started successfully services_count=1
2026/06/23 22:07:44 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-child1
2026/06/23 22:07:44 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:44 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:44 INFO Planned instance shutdown triggered by token update botInstanceID=child1
2026/06/23 22:07:44 INFO Stopping service... service=bot-runtime-child1
2026/06/23 22:07:44 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=child1
2026/06/23 22:07:44 INFO Stopping all services...
2026/06/23 22:07:44 INFO Stopping service... service=command-handler
2026/06/23 22:07:44 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:44 INFO All services stopped successfully
2026/06/23 22:07:44 INFO Service stopped service=bot-runtime-child1
2026/06/23 22:07:44 INFO executeStopAndRemove DELETING instance id=child1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/23 22:07:44 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-child1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Starting service... service=command-handler
2026/06/23 22:07:44 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:44 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/23 22:07:44 INFO Service started successfully service=command-handler
2026/06/23 22:07:44 INFO All services started successfully services_count=1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-child1
2026/06/23 22:07:44 INFO Load summary initialized guilds_count=1
2026/06/23 22:07:44 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:44 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:44 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:44 INFO Planned instance shutdown triggered by token removal botInstanceID=child1
2026/06/23 22:07:44 INFO Stopping service... service=bot-runtime-child1
2026/06/23 22:07:44 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=child1
2026/06/23 22:07:44 INFO Stopping all services...
2026/06/23 22:07:44 INFO Stopping service... service=command-handler
2026/06/23 22:07:44 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Service stopped successfully service=command-handler
2026/06/23 22:07:44 INFO All services stopped successfully
2026/06/23 22:07:44 INFO Service stopped service=bot-runtime-child1
2026/06/23 22:07:44 INFO executeStopAndRemove DELETING instance id=child1
2026/06/23 22:07:44 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:44 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/23 22:07:44 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/23 22:07:44 WARN Handshake failure with guild interface reported by central hub guildID=g1
--- PASS: TestSupervisorConfigChange (0.19s)
=== RUN   TestBotSupervisor_ConcurrentConfigThrashing
2026/06/23 22:07:44 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/23 22:07:44 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/23 22:07:44 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/23 22:07:44 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:44 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:44 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:44 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/23 22:07:44 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/23 22:07:44 INFO Planned instance shutdown triggered by token update botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Planned instance shutdown triggered by token update botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Planned instance shutdown triggered by token update botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO executeStopAndRemove SKIPPING deletion: pointer mismatch id=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO executeStopAndRemove SKIPPING deletion: pointer mismatch id=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO executeStopAndRemove SKIPPING deletion: pointer mismatch id=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 INFO Stopping service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Stopping all services...
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services stopped successfully
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 INFO Service stopped service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO executeStopAndRemove SKIPPING deletion: pointer mismatch id=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 WARN Service manager memory conflict detected; executing forceful override botInstanceID=instance_1
2026/06/23 22:07:44 INFO Starting service... service=bot-runtime-instance_1
2026/06/23 22:07:44 INFO Starting all services...
2026/06/23 22:07:44 INFO All services started successfully services_count=0
2026/06/23 22:07:44 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/23 22:07:44 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/23 22:07:44 INFO Service started successfully service=bot-runtime-instance_1
--- PASS: TestBotSupervisor_ConcurrentConfigThrashing (0.05s)
=== RUN   TestBotSupervisor_GracefulShutdownOrchestration
2026/06/23 22:07:44 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/23 22:07:44 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/23 22:07:44 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/23 22:07:44 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:44 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:44 INFO executeStopAndRemove DELETING instance id=zombie_instance
2026/06/23 22:07:44 ERROR BotSupervisor stop timeout exceeded before background task completion request_id=supervisor_shutdown error="context deadline exceeded"
2026/06/23 22:07:44 ERROR Failed to purge I/O, escalated to ForceRemove request_id=stop_remove_zombie_instance botInstanceID=zombie_instance error="stop signal failed for bot-runtime-zombie_instance: context deadline exceeded"
2026/06/23 22:07:44 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestBotSupervisor_GracefulShutdownOrchestration (0.05s)
=== RUN   TestBotSupervisor_StopMemoryBarrier
2026/06/23 22:07:44 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:44 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/23 22:07:44 INFO executeStopAndRemove DELETING instance id=zombie_instance
2026/06/23 22:07:44 ERROR BotSupervisor stop timeout exceeded before background task completion request_id=supervisor_shutdown error="context deadline exceeded"
2026/06/23 22:07:44 ERROR Failed to purge I/O, escalated to ForceRemove request_id=stop_remove_zombie_instance botInstanceID=zombie_instance error="stop signal failed for bot-runtime-zombie_instance: context deadline exceeded"
--- PASS: TestBotSupervisor_StopMemoryBarrier (0.00s)
=== RUN   TestCatalogRegistrars_RegisterArikawa
=== PAUSE TestCatalogRegistrars_RegisterArikawa
=== RUN   TestCatalogRegistrars_DIFailures
=== PAUSE TestCatalogRegistrars_DIFailures
=== RUN   TestCatalogRegistrars_Capabilities
=== PAUSE TestCatalogRegistrars_Capabilities
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity
=== RUN   TestRuntimeCommandCatalogRegistrar_FailFastBarrier
=== PAUSE TestRuntimeCommandCatalogRegistrar_FailFastBarrier
=== RUN   TestCommandHandlerSetupAndShutdownLifecycle
2026/06/23 22:07:44 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=runtime
2026/06/23 22:07:44 INFO Architectural state transition: Primary routines initialization component=PartnerCommands
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=partner
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=ban
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=timeout
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=massban
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=reaction_block
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=clean
2026/06/23 22:07:44 INFO Architectural state transition: Primary routines initialization component=RolePanelCommands
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=roles
2026/06/23 22:07:44 INFO Architectural state transition: Primary routines initialization component=EmbedCommands
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=embed
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=qotd
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=logging
2026/06/23 22:07:44 INFO Command catalog fragments coupled to the native Arikawa router
2026/06/23 22:07:44 INFO Successfully synchronized commands via BulkOverwrite guild_id="" total_commands=11
2026/06/23 22:07:44 INFO Command architecture successfully established natively botInstanceID=""
2026/06/23 22:07:44 INFO Starting command and route coupling botInstanceID=""
2026/06/23 22:07:44 WARN overlapping handler registration; invoking cleanup of previous registrations botInstanceID=""
2026/06/23 22:07:44 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=runtime
2026/06/23 22:07:44 INFO Architectural state transition: Primary routines initialization component=PartnerCommands
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=partner
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=ban
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=timeout
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=massban
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=reaction_block
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=clean
2026/06/23 22:07:44 INFO Architectural state transition: Primary routines initialization component=RolePanelCommands
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=roles
2026/06/23 22:07:44 INFO Architectural state transition: Primary routines initialization component=EmbedCommands
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=embed
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=qotd
2026/06/23 22:07:44 INFO Architectural state transition: Registering native command command_name=logging
2026/06/23 22:07:44 INFO Command catalog fragments coupled to the native Arikawa router
2026/06/23 22:07:44 INFO Successfully synchronized commands via BulkOverwrite guild_id="" total_commands=11
2026/06/23 22:07:44 INFO Command architecture successfully established natively botInstanceID=""
2026/06/23 22:07:44 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/23 22:07:44 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
--- PASS: TestCommandHandlerSetupAndShutdownLifecycle (0.10s)
=== RUN   TestCommandHandlerSetupRollbackOnManagerFailure
2026/06/23 22:07:44 INFO Starting command and route coupling botInstanceID=""
--- PASS: TestCommandHandlerSetupRollbackOnManagerFailure (0.00s)
=== RUN   TestCommandHandlerSkipsGuildWithoutCommandsFeature
2026/06/23 22:07:44 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:44 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestCommandHandlerSkipsGuildWithoutCommandsFeature (0.00s)
=== RUN   TestCommandHandlerRoutesFeaturesToCorrectBotInstance
2026/06/23 22:07:44 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:44 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Roles_command_goes_to_generic
=== RUN   TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Moderation_command_goes_to_generic
=== RUN   TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Base_command_goes_to_generic
=== RUN   TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Unrouted_QOTD_command_goes_to_no_one
--- PASS: TestCommandHandlerRoutesFeaturesToCorrectBotInstance (0.00s)
    --- PASS: TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Roles_command_goes_to_generic (0.00s)
    --- PASS: TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Moderation_command_goes_to_generic (0.00s)
    --- PASS: TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Base_command_goes_to_generic (0.00s)
    --- PASS: TestCommandHandlerRoutesFeaturesToCorrectBotInstance/Unrouted_QOTD_command_goes_to_no_one (0.00s)
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv/default_empty_is_nil
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv/incomplete_config_fails
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv/complete_config
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv/missing_redirect_without_public_origin_fails
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv/missing_redirect_derives_from_public_origin
=== RUN   TestLoadControlDiscordOAuthConfigFromEnv/explicit_client_id_overrides_repo_default
--- PASS: TestLoadControlDiscordOAuthConfigFromEnv (0.00s)
    --- PASS: TestLoadControlDiscordOAuthConfigFromEnv/default_empty_is_nil (0.00s)
    --- PASS: TestLoadControlDiscordOAuthConfigFromEnv/incomplete_config_fails (0.00s)
    --- PASS: TestLoadControlDiscordOAuthConfigFromEnv/complete_config (0.00s)
    --- PASS: TestLoadControlDiscordOAuthConfigFromEnv/missing_redirect_without_public_origin_fails (0.00s)
    --- PASS: TestLoadControlDiscordOAuthConfigFromEnv/missing_redirect_derives_from_public_origin (0.00s)
    --- PASS: TestLoadControlDiscordOAuthConfigFromEnv/explicit_client_id_overrides_repo_default (0.00s)
=== RUN   TestLoadControlTLSFilesFromEnv
=== RUN   TestLoadControlTLSFilesFromEnv/not_configured
=== RUN   TestLoadControlTLSFilesFromEnv/incomplete_config_fails
=== RUN   TestLoadControlTLSFilesFromEnv/complete_config
--- PASS: TestLoadControlTLSFilesFromEnv (0.00s)
    --- PASS: TestLoadControlTLSFilesFromEnv/not_configured (0.00s)
    --- PASS: TestLoadControlTLSFilesFromEnv/incomplete_config_fails (0.00s)
    --- PASS: TestLoadControlTLSFilesFromEnv/complete_config (0.00s)
=== RUN   TestResolveControlRuntimeUsesManagedLocalHTTPS
2026/06/23 22:07:44 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
2026/06/23 22:07:44 INFO Architectural state transition: Initiating ad-hoc generation of local TLS credentials for control plane binding
--- PASS: TestResolveControlRuntimeUsesManagedLocalHTTPS (1.10s)
=== RUN   TestResolveControlRuntimeDerivesOAuthRedirectFromPublicOrigin
2026/06/23 22:07:45 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
--- PASS: TestResolveControlRuntimeDerivesOAuthRedirectFromPublicOrigin (0.00s)
=== RUN   TestArikawaQOTDPublisher_GetArikawaPublisher
=== PAUSE TestArikawaQOTDPublisher_GetArikawaPublisher
=== RUN   TestArikawaQOTDPublisher_PublishOfficialPost
=== PAUSE TestArikawaQOTDPublisher_PublishOfficialPost
=== RUN   TestArikawaQOTDPublisher_DeleteOfficialPost
=== PAUSE TestArikawaQOTDPublisher_DeleteOfficialPost
=== RUN   TestNotifyLifecycleEventSendsWebhook
2026/06/23 22:07:45 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=starting
2026/06/23 22:07:45 INFO Architectural state transition: Lifecycle webhook notification transmitted successfully reason=starting
2026/06/23 22:07:45 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=fatal
2026/06/23 22:07:45 INFO Architectural state transition: Lifecycle webhook notification transmitted successfully reason=fatal
--- PASS: TestNotifyLifecycleEventSendsWebhook (0.00s)
=== RUN   TestBuildLifecycleContentFormat
--- PASS: TestBuildLifecycleContentFormat (0.00s)
=== RUN   TestBuildLifecycleContentFallsBackWhenIdentityUnset
--- PASS: TestBuildLifecycleContentFallsBackWhenIdentityUnset (0.00s)
=== RUN   TestNotifyLifecycleEventHandles5xx
2026/06/23 22:07:45 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=fatal
2026/06/23 22:07:45 WARN Mitigated service degradation: Discord upstream rejected lifecycle webhook payload operation=lifecycle.webhook reason=fatal status_code=500 retry_after=0
--- PASS: TestNotifyLifecycleEventHandles5xx (0.00s)
=== RUN   TestNotifyLifecycleEventTimeoutContext
2026/06/23 22:07:45 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=stopping
2026/06/23 22:07:48 WARN Mitigated service degradation: External webhook endpoint unreachable; timeout or DNS failure operation=lifecycle.webhook reason=stopping error="Post \"http://127.0.0.1:58772\": context deadline exceeded"
--- PASS: TestNotifyLifecycleEventTimeoutContext (5.00s)
=== RUN   TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild
=== PAUSE TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild
=== RUN   TestCollectStartupWebhookEmbedUpdatesNilConfig
=== PAUSE TestCollectStartupWebhookEmbedUpdatesNilConfig
=== RUN   TestRun_MissingDatabaseURL
2026/06/23 22:07:50 INFO Architectural state transition: Executing application boot sequence
2026/06/23 22:07:50 INFO Architectural state transition: Executing application binary version_info="🚀 Starting testapp (discordcore v0.848.0)..."
2026/06/23 22:07:50 INFO Architectural state transition: Commencing teardown sequence across local orchestrators app_name=testapp
2026/06/23 22:07:50 INFO Stopping all services...
2026/06/23 22:07:50 INFO All services stopped successfully
2026/06/23 22:07:50 ERROR Primary execution routine aborted app_name=testapp error="InitializeIO resolveDatabaseBootstrap: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
--- PASS: TestRun_MissingDatabaseURL (0.00s)
=== RUN   TestRunWithOptions_MissingDatabaseURL
2026/06/23 22:07:50 INFO Architectural state transition: Executing application boot sequence
2026/06/23 22:07:50 INFO Architectural state transition: Executing application binary version_info="🚀 Starting testapp (discordcore v0.848.0)..."
2026/06/23 22:07:50 INFO Architectural state transition: Commencing teardown sequence across local orchestrators app_name=testapp
2026/06/23 22:07:50 INFO Stopping all services...
2026/06/23 22:07:50 INFO All services stopped successfully
2026/06/23 22:07:50 ERROR Primary execution routine aborted app_name=testapp error="InitializeIO resolveDatabaseBootstrap: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
--- PASS: TestRunWithOptions_MissingDatabaseURL (0.00s)
=== RUN   TestSetupStorage
--- PASS: TestSetupStorage (0.12s)
=== RUN   TestRunner_ShutdownStartupServices
--- PASS: TestRunner_ShutdownStartupServices (0.00s)
=== RUN   TestRunner_ResolveRuntimeCapabilities
--- PASS: TestRunner_ResolveRuntimeCapabilities (0.00s)
=== RUN   TestRunner_ApplyConfiguredTheme
2026/06/23 22:07:50 INFO Architectural state transition: Standard UI theme locked
--- PASS: TestRunner_ApplyConfiguredTheme (0.00s)
=== RUN   TestRunner_ScheduleDBCleanup
2026/06/23 22:07:50 INFO Architectural state transition: Initializing persistent cache garbage collector
--- PASS: TestRunner_ScheduleDBCleanup (0.00s)
=== RUN   TestFormatStartupMessage
=== PAUSE TestFormatStartupMessage
=== RUN   TestRun_CascadingRollbackFailures
    runner_test.go:158: Skipping test: database URL not configured
--- SKIP: TestRun_CascadingRollbackFailures (0.00s)
=== RUN   TestRun_ResourceCleanupOnBootFailure
    runner_test.go:158: Skipping test: database URL not configured
--- SKIP: TestRun_ResourceCleanupOnBootFailure (0.00s)
=== RUN   TestResolveDatabaseBootstrapFromEnv
--- PASS: TestResolveDatabaseBootstrapFromEnv (0.00s)
=== RUN   TestResolveDatabaseBootstrapRequiresEnv
--- PASS: TestResolveDatabaseBootstrapRequiresEnv (0.00s)
=== RUN   TestStartupTaskOrchestrator_GoLight
=== PAUSE TestStartupTaskOrchestrator_GoLight
=== RUN   TestStartupTaskOrchestrator_GoHeavy
=== PAUSE TestStartupTaskOrchestrator_GoHeavy
=== RUN   TestStartupTaskOrchestrator_ShutdownWithContextCancellation
=== PAUSE TestStartupTaskOrchestrator_ShutdownWithContextCancellation
=== RUN   TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed
=== PAUSE TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed
=== RUN   TestStartupTaskOrchestrator_GoNil
=== PAUSE TestStartupTaskOrchestrator_GoNil
=== RUN   TestResolveParallelism
=== PAUSE TestResolveParallelism
=== RUN   TestControlServerHolder_SetAndStop
=== PAUSE TestControlServerHolder_SetAndStop
=== RUN   TestScheduleControlServerStartup
=== PAUSE TestScheduleControlServerStartup
=== RUN   TestScheduleStartupWebhookEmbedUpdates
=== PAUSE TestScheduleStartupWebhookEmbedUpdates
=== RUN   TestStartControlServerStartupTask
=== PAUSE TestStartControlServerStartupTask
=== RUN   TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets
=== PAUSE TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets
=== RUN   TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride
=== PAUSE TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride
=== RUN   TestNewRuntimeTaskRouterConfigBuildsSharedLimiter
=== PAUSE TestNewRuntimeTaskRouterConfigBuildsSharedLimiter
=== RUN   TestAppVersion
--- PASS: TestAppVersion (0.00s)
=== CONT  TestBotRuntime_CapabilityBitmaskDerivation
=== CONT  TestControlServerHolder_SetAndStop
=== CONT  TestStartupTaskOrchestrator_GoHeavy
--- PASS: TestControlServerHolder_SetAndStop (0.00s)
=== CONT  TestRuntimeCommandCatalogRegistrar_FailFastBarrier
2026/06/23 22:07:50 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=2
=== CONT  TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets
--- PASS: TestRuntimeCommandCatalogRegistrar_FailFastBarrier (0.00s)
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=3 queue_capacity=6
=== CONT  TestFormatStartupMessage
=== CONT  TestStartControlServerStartupTask
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=2
=== CONT  TestScheduleStartupWebhookEmbedUpdates
=== RUN   TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation
2026/06/23 22:07:50 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
=== PAUSE TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation
=== CONT  TestCollectStartupWebhookEmbedUpdatesNilConfig
--- PASS: TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets (0.00s)
=== CONT  TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride
2026/06/23 22:07:50 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
=== CONT  TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild
2026/06/23 22:07:50 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
=== CONT  TestCatalogRegistrars_DIFailures
2026/06/23 22:07:50 INFO Architectural transition: Binding control server socket address=127.0.0.1:0
=== CONT  TestArikawaQOTDPublisher_DeleteOfficialPost
=== RUN   TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager
2026/06/23 22:07:50 INFO Architectural state transition: Initializing primary HTTP control plane bind_addr=127.0.0.1:0
=== RUN   TestFormatStartupMessage/no_app_version_includes_discordcore
=== PAUSE TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager
2026/06/23 22:07:50 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:50 INFO Architectural state transition: Allocating stateless native Arikawa publisher orchestrator
=== CONT  TestNewRuntimeTaskRouterConfigBuildsSharedLimiter
=== CONT  TestArikawaQOTDPublisher_PublishOfficialPost
2026/06/23 22:07:50 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/23 22:07:50 INFO Architectural state transition: Allocating stateless native Arikawa publisher orchestrator
=== CONT  TestArikawaQOTDPublisher_GetArikawaPublisher
2026/06/23 22:07:50 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=default concurrency_budget=5
2026/06/23 22:07:50 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
--- PASS: TestScheduleStartupWebhookEmbedUpdates (0.00s)
2026/06/23 22:07:50 INFO Architectural state transition: Allocating stateless native Arikawa publisher orchestrator
=== CONT  TestCatalogRegistrars_Capabilities
=== CONT  TestStartupTaskOrchestrator_GoNil
=== PAUSE TestFormatStartupMessage/no_app_version_includes_discordcore
2026/06/23 22:07:50 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
=== CONT  TestResolveParallelism
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
=== CONT  TestBotRuntimeResolver_ConcurrentMemoryRotation
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity
2026/06/23 22:07:50 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=1
=== CONT  TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evaluates_as_true_against_any_base_mask
2026/06/23 22:07:50 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
--- PASS: TestCollectStartupWebhookEmbedUpdatesNilConfig (0.00s)
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
=== RUN   TestCatalogRegistrars_Capabilities/Moderation_Capabilities
--- PASS: TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride (0.00s)
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
=== RUN   TestFormatStartupMessage/different_versions_include_both
=== RUN   TestResolveParallelism/RuntimeStartup
2026/06/23 22:07:50 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
=== CONT  TestStartupTaskOrchestrator_ShutdownWithContextCancellation
2026/06/23 22:07:50 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/23 22:07:50 WARN Mitigated service degradation: Background startup task encountered an error and aborted task=error_task kind=heavy error="simulated task error"
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evaluates_as_true_against_any_base_mask
2026/06/23 22:07:50 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
=== PAUSE TestCatalogRegistrars_Capabilities/Moderation_Capabilities
--- PASS: TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild (0.00s)
=== RUN   TestCatalogRegistrars_Capabilities/Stats_Capabilities
=== PAUSE TestFormatStartupMessage/different_versions_include_both
--- PASS: TestStartupTaskOrchestrator_GoHeavy (0.00s)
=== CONT  TestStartupTaskOrchestrator_GoLight
2026/06/23 22:07:50 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
=== CONT  TestScheduleControlServerStartup
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/23 22:07:50 INFO Architectural transition: Control server startup bypassed via explicit run options
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Empty_mask_rejects_any_specific_capability
2026/06/23 22:07:50 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/23 22:07:50 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
=== PAUSE TestCatalogRegistrars_Capabilities/Stats_Capabilities
=== RUN   TestFormatStartupMessage/same_versions_omit_discordcore_suffix
2026/06/23 22:07:50 INFO Architectural transition: Control server initializing without authentication middleware addr=127.0.0.1:8376 dashboard_only=true
=== PAUSE TestFormatStartupMessage/same_versions_omit_discordcore_suffix
2026/06/23 22:07:50 INFO Architectural transition: Binding control server socket address=127.0.0.1:8376
2026/06/23 22:07:50 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestArikawaQOTDPublisher_DeleteOfficialPost (0.00s)
2026/06/23 22:07:50 INFO Architectural state transition: Initializing primary HTTP control plane bind_addr=127.0.0.1:8376
=== RUN   TestFormatStartupMessage/trims_spaces
--- PASS: TestArikawaQOTDPublisher_PublishOfficialPost (0.00s)
=== CONT  TestCatalogRegistrars_RegisterArikawa
=== RUN   TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring
=== CONT  TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Empty_mask_rejects_any_specific_capability
=== PAUSE TestFormatStartupMessage/trims_spaces
--- PASS: TestNewRuntimeTaskRouterConfigBuildsSharedLimiter (0.00s)
--- PASS: TestArikawaQOTDPublisher_GetArikawaPublisher (0.00s)
--- PASS: TestStartupTaskOrchestrator_GoNil (0.00s)
=== PAUSE TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring
=== CONT  TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager
=== RUN   TestResolveParallelism/RuntimeBackground
=== RUN   TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring
--- PASS: TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed (0.00s)
=== CONT  TestCatalogRegistrars_Capabilities/Stats_Capabilities
--- PASS: TestStartupTaskOrchestrator_ShutdownWithContextCancellation (0.00s)
--- PASS: TestStartupTaskOrchestrator_GoLight (0.00s)
=== PAUSE TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring
=== CONT  TestCatalogRegistrars_Capabilities/Moderation_Capabilities
=== CONT  TestFormatStartupMessage/same_versions_omit_discordcore_suffix
--- PASS: TestCatalogRegistrars_DIFailures (0.00s)
    --- PASS: TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager (0.00s)
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_singular_target
=== CONT  TestFormatStartupMessage/trims_spaces
=== CONT  TestFormatStartupMessage/different_versions_include_both
=== CONT  TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring
=== RUN   TestResolveParallelism/StartupLight
--- PASS: TestBotRuntime_CapabilityBitmaskDerivation (0.00s)
    --- PASS: TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation (0.00s)
--- PASS: TestCatalogRegistrars_Capabilities (0.00s)
    --- PASS: TestCatalogRegistrars_Capabilities/Stats_Capabilities (0.00s)
    --- PASS: TestCatalogRegistrars_Capabilities/Moderation_Capabilities (0.00s)
=== CONT  TestFormatStartupMessage/no_app_version_includes_discordcore
=== RUN   TestResolveParallelism/StartupLightQueue
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_singular_target
--- PASS: TestFormatStartupMessage (0.00s)
    --- PASS: TestFormatStartupMessage/same_versions_omit_discordcore_suffix (0.00s)
    --- PASS: TestFormatStartupMessage/trims_spaces (0.00s)
    --- PASS: TestFormatStartupMessage/different_versions_include_both (0.00s)
    --- PASS: TestFormatStartupMessage/no_app_version_includes_discordcore (0.00s)
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_does_not_contain_missing_target
--- PASS: TestResolveParallelism (0.00s)
    --- PASS: TestResolveParallelism/RuntimeStartup (0.00s)
    --- PASS: TestResolveParallelism/RuntimeBackground (0.00s)
    --- PASS: TestResolveParallelism/StartupLight (0.00s)
    --- PASS: TestResolveParallelism/StartupLightQueue (0.00s)
=== CONT  TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_does_not_contain_missing_target
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_exact_multiple_targets
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_exact_multiple_targets
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evaluates_as_true_against_any_base_mask
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_exact_multiple_targets
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Empty_mask_rejects_any_specific_capability
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_singular_target
--- PASS: TestCatalogRegistrars_RegisterArikawa (0.00s)
    --- PASS: TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring (0.00s)
    --- PASS: TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring (0.00s)
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_does_not_contain_missing_target
--- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evaluates_as_true_against_any_base_mask (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_exact_multiple_targets (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Empty_mask_rejects_any_specific_capability (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_contains_singular_target (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Composite_mask_does_not_contain_missing_target (0.00s)
--- PASS: TestStartControlServerStartupTask (0.01s)
--- PASS: TestScheduleControlServerStartup (0.01s)
--- PASS: TestBotRuntimeResolver_ConcurrentMemoryRotation (0.50s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/app	11.950s
=== RUN   TestRunUsesMainProfileOptions
--- PASS: TestRunUsesMainProfileOptions (0.03s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/app/runtimecmd	2.950s
=== RUN   TestExecutionEvent_Golden_Unmarshal
=== PAUSE TestExecutionEvent_Golden_Unmarshal
=== RUN   TestExecutionEvent_RoundTrip
=== PAUSE TestExecutionEvent_RoundTrip
=== RUN   TestNopSink_OnAutomodBlock
=== PAUSE TestNopSink_OnAutomodBlock
=== CONT  TestExecutionEvent_Golden_Unmarshal
=== CONT  TestExecutionEvent_RoundTrip
=== CONT  TestNopSink_OnAutomodBlock
=== RUN   TestNopSink_OnAutomodBlock/should_execute_without_panics_or_side_effects
=== PAUSE TestNopSink_OnAutomodBlock/should_execute_without_panics_or_side_effects
=== CONT  TestNopSink_OnAutomodBlock/should_execute_without_panics_or_side_effects
--- PASS: TestExecutionEvent_Golden_Unmarshal (0.00s)
--- PASS: TestNopSink_OnAutomodBlock (0.00s)
    --- PASS: TestNopSink_OnAutomodBlock/should_execute_without_panics_or_side_effects (0.00s)
--- PASS: TestExecutionEvent_RoundTrip (0.00s)
=== RUN   FuzzExecutionEvent_Unmarshal
=== RUN   FuzzExecutionEvent_Unmarshal/seed#0
=== RUN   FuzzExecutionEvent_Unmarshal/seed#1
=== RUN   FuzzExecutionEvent_Unmarshal/seed#2
=== RUN   FuzzExecutionEvent_Unmarshal/seed#3
--- PASS: FuzzExecutionEvent_Unmarshal (0.00s)
    --- PASS: FuzzExecutionEvent_Unmarshal/seed#0 (0.00s)
    --- PASS: FuzzExecutionEvent_Unmarshal/seed#1 (0.00s)
    --- PASS: FuzzExecutionEvent_Unmarshal/seed#2 (0.00s)
    --- PASS: FuzzExecutionEvent_Unmarshal/seed#3 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/automod	2.914s
?   	github.com/small-frappuccino/discordcore/pkg/automod/automodmocks	[no test files]
=== RUN   TestCategorizeMessages
=== PAUSE TestCategorizeMessages
=== RUN   TestApplyFilter
=== PAUSE TestApplyFilter
=== CONT  TestCategorizeMessages
=== RUN   TestCategorizeMessages/exact_boundary_-_bulk
=== CONT  TestApplyFilter
=== RUN   TestCategorizeMessages/exact_boundary_-_single
=== RUN   TestCategorizeMessages/recent
=== RUN   TestCategorizeMessages/old
--- PASS: TestCategorizeMessages (0.00s)
    --- PASS: TestCategorizeMessages/exact_boundary_-_bulk (0.00s)
    --- PASS: TestCategorizeMessages/exact_boundary_-_single (0.00s)
    --- PASS: TestCategorizeMessages/recent (0.00s)
    --- PASS: TestCategorizeMessages/old (0.00s)
--- PASS: TestApplyFilter (0.00s)
=== RUN   FuzzSnowflake
=== RUN   FuzzSnowflake/seed#0
=== RUN   FuzzSnowflake/seed#1
=== RUN   FuzzSnowflake/seed#2
=== RUN   FuzzSnowflake/seed#3
=== RUN   FuzzSnowflake/seed#4
=== RUN   FuzzSnowflake/seed#5
--- PASS: FuzzSnowflake (0.00s)
    --- PASS: FuzzSnowflake/seed#0 (0.00s)
    --- PASS: FuzzSnowflake/seed#1 (0.00s)
    --- PASS: FuzzSnowflake/seed#2 (0.00s)
    --- PASS: FuzzSnowflake/seed#3 (0.00s)
    --- PASS: FuzzSnowflake/seed#4 (0.00s)
    --- PASS: FuzzSnowflake/seed#5 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/clean	1.390s
=== RUN   TestHTTPClock_Success
=== PAUSE TestHTTPClock_Success
=== RUN   TestHTTPClock_Timeout
=== PAUSE TestHTTPClock_Timeout
=== RUN   TestHTTPClock_MalformedHeader
=== PAUSE TestHTTPClock_MalformedHeader
=== RUN   TestHTTPClock_MissingHeader
=== PAUSE TestHTTPClock_MissingHeader
=== RUN   TestMockClock_Concurrency
=== PAUSE TestMockClock_Concurrency
=== RUN   TestMockClock_TimersAndTickers
=== PAUSE TestMockClock_TimersAndTickers
=== RUN   TestMockClock_NonBlockingDispatch
=== PAUSE TestMockClock_NonBlockingDispatch
=== CONT  TestMockClock_NonBlockingDispatch
=== CONT  TestHTTPClock_MissingHeader
--- PASS: TestMockClock_NonBlockingDispatch (0.00s)
=== CONT  TestHTTPClock_MalformedHeader
=== CONT  TestMockClock_TimersAndTickers
=== CONT  TestHTTPClock_Success
=== CONT  TestHTTPClock_Timeout
--- PASS: TestMockClock_TimersAndTickers (0.00s)
=== CONT  TestMockClock_Concurrency
2026/06/23 22:07:43 INFO HTTPClock sync completed url=http://127.0.0.1:57476 offset=4m59.2495623s
2026/06/23 22:07:43 INFO HTTPClock sync completed url=http://127.0.0.1:57474 offset=-750.7684ms
2026/06/23 22:07:43 WARN HTTPClock sync failed: unparseable Date header; falling back to OS time url=http://127.0.0.1:57477 header="Invalid Date String" err="parsing time \"Invalid Date String\" as \"Mon, 02 Jan 2006 15:04:05 MST\": cannot parse \"Invalid Date String\" as \"Mon\""
--- PASS: TestHTTPClock_MalformedHeader (0.02s)
--- PASS: TestHTTPClock_MissingHeader (0.02s)
--- PASS: TestHTTPClock_Success (0.02s)
--- PASS: TestMockClock_Concurrency (0.03s)
2026/06/23 22:07:48 WARN HTTPClock sync failed; falling back to OS time url=http://127.0.0.1:57475 err="Head \"http://127.0.0.1:57475\": context deadline exceeded"
--- PASS: TestHTTPClock_Timeout (5.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/clock	6.643s
=== RUN   TestDashboard_CompressionNegotiation
=== PAUSE TestDashboard_CompressionNegotiation
=== RUN   TestFeaturesSettings_RaceConditions
=== PAUSE TestFeaturesSettings_RaceConditions
=== RUN   TestGuilds_SimpleGet
=== PAUSE TestGuilds_SimpleGet
=== RUN   TestHealth_GenericReflection
=== PAUSE TestHealth_GenericReflection
=== RUN   TestMiddleware_OOMPrevention
=== PAUSE TestMiddleware_OOMPrevention
=== RUN   TestMiddleware_TimingAttack
=== PAUSE TestMiddleware_TimingAttack
=== RUN   TestMiddleware_AdminAccess
=== PAUSE TestMiddleware_AdminAccess
=== RUN   TestOAuth_CSRFPurge
=== PAUSE TestOAuth_CSRFPurge
=== RUN   TestRouter_Go122MethodMultiplexing
=== PAUSE TestRouter_Go122MethodMultiplexing
=== RUN   TestServer_GracefulDegradation
=== PAUSE TestServer_GracefulDegradation
=== CONT  TestDashboard_CompressionNegotiation
=== CONT  TestMiddleware_OOMPrevention
=== CONT  TestServer_GracefulDegradation
=== CONT  TestRouter_Go122MethodMultiplexing
=== CONT  TestMiddleware_AdminAccess
=== CONT  TestMiddleware_TimingAttack
=== CONT  TestOAuth_CSRFPurge
=== CONT  TestHealth_GenericReflection
2026/06/23 22:07:45 WARN Mitigated service degradation: Forbidden access attempt by non-admin identity
2026/06/23 22:07:45 WARN Mitigated service degradation: Invalid Authorization token provided
--- PASS: TestMiddleware_TimingAttack (0.01s)
=== CONT  TestGuilds_SimpleGet
--- PASS: TestGuilds_SimpleGet (0.01s)
=== CONT  TestFeaturesSettings_RaceConditions
--- PASS: TestRouter_Go122MethodMultiplexing (0.02s)
--- PASS: TestMiddleware_AdminAccess (0.02s)
2026/06/23 22:07:45 INFO Architectural state transition: Commencing graceful shutdown of HTTP control plane
--- PASS: TestHealth_GenericReflection (0.02s)
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
=== RUN   TestDashboard_CompressionNegotiation/Gzip_supported
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
=== PAUSE TestDashboard_CompressionNegotiation/Gzip_supported
=== RUN   TestDashboard_CompressionNegotiation/Brotli_fallback_to_gzip
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
=== PAUSE TestDashboard_CompressionNegotiation/Brotli_fallback_to_gzip
=== RUN   TestDashboard_CompressionNegotiation/No_compression
=== PAUSE TestDashboard_CompressionNegotiation/No_compression
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
=== CONT  TestDashboard_CompressionNegotiation/Brotli_fallback_to_gzip
=== CONT  TestDashboard_CompressionNegotiation/No_compression
=== CONT  TestDashboard_CompressionNegotiation/Gzip_supported
--- PASS: TestDashboard_CompressionNegotiation (0.03s)
    --- PASS: TestDashboard_CompressionNegotiation/No_compression (0.00s)
    --- PASS: TestDashboard_CompressionNegotiation/Brotli_fallback_to_gzip (0.00s)
    --- PASS: TestDashboard_CompressionNegotiation/Gzip_supported (0.00s)
2026/06/23 22:07:45 WARN Mitigated service degradation: OAuth state CSRF validation failed received_state=forged_state
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
--- PASS: TestOAuth_CSRFPurge (0.02s)
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
2026/06/23 22:07:45 INFO Architectural state transition: Runtime configuration updated via control plane
--- PASS: TestFeaturesSettings_RaceConditions (0.01s)
--- PASS: TestMiddleware_OOMPrevention (0.11s)
--- PASS: TestServer_GracefulDegradation (5.03s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/control	6.139s
=== RUN   TestEnsureReadyCreatesMaterialsAndTrusts
=== PAUSE TestEnsureReadyCreatesMaterialsAndTrusts
=== RUN   TestEnsureReadyReusesExistingMaterials
=== PAUSE TestEnsureReadyReusesExistingMaterials
=== RUN   TestEnsureReadyRotatesServerCertificateWhenSANSChange
=== PAUSE TestEnsureReadyRotatesServerCertificateWhenSANSChange
=== RUN   TestEnsureReadyErrorsOnCorruptKey
=== PAUSE TestEnsureReadyErrorsOnCorruptKey
=== CONT  TestEnsureReadyRotatesServerCertificateWhenSANSChange
=== CONT  TestEnsureReadyReusesExistingMaterials
=== CONT  TestEnsureReadyErrorsOnCorruptKey
=== CONT  TestEnsureReadyCreatesMaterialsAndTrusts
--- PASS: TestEnsureReadyCreatesMaterialsAndTrusts (0.42s)
--- PASS: TestEnsureReadyReusesExistingMaterials (0.42s)
--- PASS: TestEnsureReadyRotatesServerCertificateWhenSANSChange (0.50s)
--- PASS: TestEnsureReadyErrorsOnCorruptKey (0.52s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/control/localtls	1.573s
?   	github.com/small-frappuccino/discordcore/pkg/discord	[no test files]
?   	github.com/small-frappuccino/discordcore/pkg/discord/automod	[no test files]
=== RUN   TestCache_GCEviction
=== PAUSE TestCache_GCEviction
=== RUN   TestCache_StaleReads
=== PAUSE TestCache_StaleReads
=== RUN   TestCache_ReferenceCycles
=== PAUSE TestCache_ReferenceCycles
=== RUN   TestCache_AsyncIO
=== PAUSE TestCache_AsyncIO
=== RUN   TestCache_CorruptRecovery
=== PAUSE TestCache_CorruptRecovery
=== RUN   TestSession_SingleflightLoad
=== PAUSE TestSession_SingleflightLoad
=== RUN   TestSession_SingleflightError
=== PAUSE TestSession_SingleflightError
=== RUN   TestSession_PartialInvalidation
=== PAUSE TestSession_PartialInvalidation
=== RUN   TestSession_RaceUpdate
=== PAUSE TestSession_RaceUpdate
=== CONT  TestCache_GCEviction
=== CONT  TestSession_SingleflightLoad
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=1m0s
=== CONT  TestSession_PartialInvalidation
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=1m0s
=== CONT  TestCache_CorruptRecovery
2026/06/23 22:07:49 INFO Architectural state transition: Initializing CachedSession wrapper
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=0s
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=0s
2026/06/23 22:07:49 INFO Architectural state transition: Initializing CachedSession wrapper
--- PASS: TestCache_CorruptRecovery (0.00s)
2026/06/23 22:07:49 INFO Architectural state transition: Partial Invalidation via Gateway event=GuildRoleDelete
=== CONT  TestCache_AsyncIO
--- PASS: TestSession_PartialInvalidation (0.00s)
=== CONT  TestSession_RaceUpdate
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=1m0s
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=1m0s guild_ttl=0s
2026/06/23 22:07:49 INFO Architectural state transition: Initializing CachedSession wrapper
=== CONT  TestCache_ReferenceCycles
2026/06/23 22:07:49 INFO Architectural state transition: Invalidation via Gateway event=GuildMemberUpdate
=== CONT  TestCache_StaleReads
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=1m0s
--- PASS: TestSession_RaceUpdate (0.00s)
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=1m0s guild_ttl=0s
=== CONT  TestSession_SingleflightError
2026/06/23 22:07:49 INFO Architectural state transition: Initializing UnifiedCache member_ttl=0s guild_ttl=1m0s
2026/06/23 22:07:49 INFO Architectural state transition: Initializing CachedSession wrapper
--- PASS: TestSession_SingleflightError (0.00s)
--- PASS: TestCache_AsyncIO (0.01s)
2026/06/23 22:07:49 WARN Mitigated service degradation: Stale read detected, weak pointer collected before explicit invalidation key=123
--- PASS: TestCache_GCEviction (0.01s)
2026/06/23 22:07:49 WARN Mitigated service degradation: Stale read detected, weak pointer collected before explicit invalidation key=1:1
2026/06/23 22:07:49 WARN Mitigated service degradation: Stale read detected, weak pointer collected before explicit invalidation key=456
--- PASS: TestCache_ReferenceCycles (0.01s)
--- PASS: TestCache_StaleReads (0.01s)
--- PASS: TestSession_SingleflightLoad (0.02s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/cache	1.087s
=== RUN   TestExecuteClean_Pagination
=== PAUSE TestExecuteClean_Pagination
=== RUN   TestExecuteClean_Degradation_50034
=== PAUSE TestExecuteClean_Degradation_50034
=== RUN   TestExecuteClean_Concurrency_Race
=== PAUSE TestExecuteClean_Concurrency_Race
=== RUN   TestExecuteClean_AuditDispatch
=== PAUSE TestExecuteClean_AuditDispatch
=== CONT  TestExecuteClean_Concurrency_Race
=== CONT  TestExecuteClean_Degradation_50034
=== RUN   TestExecuteClean_Concurrency_Race/concurrency_race_test
=== CONT  TestExecuteClean_Pagination
2026/06/23 22:07:50 WARN Bulk delete failed with 50034, falling back to sequential channel_id=1
=== CONT  TestExecuteClean_AuditDispatch
--- PASS: TestExecuteClean_Pagination (0.00s)
--- PASS: TestExecuteClean_Degradation_50034 (0.00s)
--- PASS: TestExecuteClean_Concurrency_Race (0.00s)
    --- PASS: TestExecuteClean_Concurrency_Race/concurrency_race_test (0.00s)
2026/06/23 22:07:50 ERROR Failed to send clean audit log error="audit failure" audit_channel_id=2
--- PASS: TestExecuteClean_AuditDispatch (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/clean	1.128s
=== RUN   TestArikawaGroupCommand_Handle
=== PAUSE TestArikawaGroupCommand_Handle
=== RUN   TestArikawaGroupCommand_Options
=== PAUSE TestArikawaGroupCommand_Options
=== RUN   TestArikawaGroupCommand_Invariants
=== PAUSE TestArikawaGroupCommand_Invariants
=== RUN   TestGetArikawaSubCommandOptions
=== PAUSE TestGetArikawaSubCommandOptions
=== RUN   TestArikawaOptionList_String
=== PAUSE TestArikawaOptionList_String
=== RUN   TestArikawaOptionList_Int
=== PAUSE TestArikawaOptionList_Int
=== RUN   TestArikawaOptionList_Bool
=== PAUSE TestArikawaOptionList_Bool
=== RUN   TestArikawaOptionList_Float
=== PAUSE TestArikawaOptionList_Float
=== RUN   TestArikawaOptionList_ChannelID
=== PAUSE TestArikawaOptionList_ChannelID
=== RUN   TestArikawaOptionList_RoleID
=== PAUSE TestArikawaOptionList_RoleID
=== RUN   TestArikawaOptionList_HasOption
=== PAUSE TestArikawaOptionList_HasOption
=== RUN   TestResolveFeatureForCommandPath
=== PAUSE TestResolveFeatureForCommandPath
=== RUN   TestCommandSyncer_BuildCreateData
=== PAUSE TestCommandSyncer_BuildCreateData
=== RUN   TestCommandSyncer_SyncBulkOverwrite_Routing
=== PAUSE TestCommandSyncer_SyncBulkOverwrite_Routing
=== RUN   TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors
=== RUN   TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors/Cenário_de_Sucesso
=== RUN   TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors/Cenário_de_Falha
--- PASS: TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors (0.00s)
    --- PASS: TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors/Cenário_de_Sucesso (0.00s)
    --- PASS: TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors/Cenário_de_Falha (0.00s)
=== RUN   TestCommandSyncer_Diff
=== PAUSE TestCommandSyncer_Diff
=== RUN   TestNewArikawaMissingConfigErrorData
=== PAUSE TestNewArikawaMissingConfigErrorData
=== RUN   TestNewArikawaContext_InitializationAndFailFast
=== PAUSE TestNewArikawaContext_InitializationAndFailFast
=== RUN   TestArikawaContext_ContextResolution
=== PAUSE TestArikawaContext_ContextResolution
=== RUN   TestArikawaContext_APIWrappers_DefensiveChecks
=== PAUSE TestArikawaContext_APIWrappers_DefensiveChecks
=== RUN   TestCommandRegistry_ConcurrentSafety
=== PAUSE TestCommandRegistry_ConcurrentSafety
=== RUN   TestRouteRegistry_BulkOverwrite
=== PAUSE TestRouteRegistry_BulkOverwrite
=== RUN   TestRouteRegistry_Diff
=== PAUSE TestRouteRegistry_Diff
=== RUN   TestCommandRouter_RouteInteraction
=== PAUSE TestCommandRouter_RouteInteraction
=== CONT  TestArikawaGroupCommand_Handle
=== CONT  TestArikawaContext_APIWrappers_DefensiveChecks
=== CONT  TestArikawaOptionList_HasOption
=== RUN   TestArikawaContext_APIWrappers_DefensiveChecks/Respond_triggers_error_on_nil_Interaction
=== CONT  TestCommandRegistry_ConcurrentSafety
=== CONT  TestRouteRegistry_Diff
=== RUN   TestArikawaContext_APIWrappers_DefensiveChecks/Defer_triggers_error_on_nil_Client
=== CONT  TestCommandRouter_RouteInteraction
=== RUN   TestCommandRouter_RouteInteraction/Valid_Slash_Command_Routing
--- PASS: TestArikawaContext_APIWrappers_DefensiveChecks (0.00s)
    --- PASS: TestArikawaContext_APIWrappers_DefensiveChecks/Respond_triggers_error_on_nil_Interaction (0.00s)
    --- PASS: TestArikawaContext_APIWrappers_DefensiveChecks/Defer_triggers_error_on_nil_Client (0.00s)
=== PAUSE TestCommandRouter_RouteInteraction/Valid_Slash_Command_Routing
=== RUN   TestCommandRouter_RouteInteraction/Unregistered_Command_Fallback
=== CONT  TestArikawaOptionList_Bool
=== PAUSE TestCommandRouter_RouteInteraction/Unregistered_Command_Fallback
=== CONT  TestRouteRegistry_BulkOverwrite
=== RUN   TestCommandRouter_RouteInteraction/Nil_Interaction_Protection
--- PASS: TestRouteRegistry_Diff (0.00s)
=== PAUSE TestCommandRouter_RouteInteraction/Nil_Interaction_Protection
=== RUN   TestArikawaGroupCommand_Handle/fails_on_invalid_type_assertion
=== CONT  TestArikawaOptionList_Float
=== RUN   TestArikawaOptionList_HasOption/Existing_Key
=== CONT  TestArikawaOptionList_ChannelID
=== RUN   TestArikawaOptionList_Bool/Happy_Path
--- PASS: TestRouteRegistry_BulkOverwrite (0.00s)
=== CONT  TestGetArikawaSubCommandOptions
=== CONT  TestArikawaOptionList_Int
=== RUN   TestArikawaOptionList_HasOption/Missing_Key
=== PAUSE TestArikawaGroupCommand_Handle/fails_on_invalid_type_assertion
=== RUN   TestGetArikawaSubCommandOptions/Invalid_Type_Assertion_(Ping_Interaction)
=== RUN   TestArikawaOptionList_Float/Happy_Path
=== RUN   TestArikawaGroupCommand_Handle/delegates_to_correct_subcommand
=== RUN   TestArikawaOptionList_HasOption/Nil_Value
=== PAUSE TestArikawaGroupCommand_Handle/delegates_to_correct_subcommand
=== RUN   TestArikawaOptionList_Int/Happy_Path
=== RUN   TestArikawaOptionList_Float/Missing_Key
=== RUN   TestArikawaOptionList_ChannelID/Happy_Path
--- PASS: TestArikawaOptionList_HasOption (0.00s)
    --- PASS: TestArikawaOptionList_HasOption/Existing_Key (0.00s)
    --- PASS: TestArikawaOptionList_HasOption/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_HasOption/Nil_Value (0.00s)
=== CONT  TestArikawaGroupCommand_Invariants
=== RUN   TestArikawaOptionList_Int/Missing_Key
=== RUN   TestArikawaOptionList_Bool/Missing_Key
=== RUN   TestArikawaOptionList_Float/Type_Mismatch
=== RUN   TestArikawaOptionList_Int/Type_Mismatch
=== RUN   TestArikawaGroupCommand_Invariants/memory_initialization
=== RUN   TestArikawaOptionList_Float/Nil_Value
=== RUN   TestArikawaOptionList_ChannelID/Missing_Key
=== RUN   TestArikawaOptionList_Bool/Type_Mismatch
=== RUN   TestArikawaOptionList_Int/Nil_Value
=== RUN   TestArikawaOptionList_ChannelID/Type_Mismatch
=== CONT  TestArikawaGroupCommand_Options
--- PASS: TestArikawaOptionList_Float (0.00s)
    --- PASS: TestArikawaOptionList_Float/Happy_Path (0.00s)
    --- PASS: TestArikawaOptionList_Float/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_Float/Type_Mismatch (0.00s)
    --- PASS: TestArikawaOptionList_Float/Nil_Value (0.00s)
=== PAUSE TestArikawaGroupCommand_Invariants/memory_initialization
=== RUN   TestArikawaGroupCommand_Handle/returns_error_on_unknown_subcommand
=== RUN   TestArikawaGroupCommand_Options/empty_state
=== CONT  TestArikawaOptionList_String
=== RUN   TestGetArikawaSubCommandOptions/Flat_Command_(No_Subcommands)
=== RUN   TestArikawaOptionList_String/Happy_Path
=== RUN   TestArikawaOptionList_ChannelID/Nil_Value
=== RUN   TestArikawaOptionList_Bool/Nil_Value
=== RUN   TestArikawaGroupCommand_Invariants/overwriting_protection
--- PASS: TestArikawaOptionList_Int (0.00s)
    --- PASS: TestArikawaOptionList_Int/Happy_Path (0.00s)
    --- PASS: TestArikawaOptionList_Int/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_Int/Type_Mismatch (0.00s)
    --- PASS: TestArikawaOptionList_Int/Nil_Value (0.00s)
=== PAUSE TestArikawaGroupCommand_Handle/returns_error_on_unknown_subcommand
=== PAUSE TestArikawaGroupCommand_Options/empty_state
=== RUN   TestArikawaGroupCommand_Handle/fails_on_empty_options
--- PASS: TestArikawaOptionList_ChannelID (0.00s)
    --- PASS: TestArikawaOptionList_ChannelID/Happy_Path (0.00s)
    --- PASS: TestArikawaOptionList_ChannelID/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_ChannelID/Type_Mismatch (0.00s)
    --- PASS: TestArikawaOptionList_ChannelID/Nil_Value (0.00s)
=== PAUSE TestArikawaGroupCommand_Invariants/overwriting_protection
=== CONT  TestArikawaContext_ContextResolution
=== RUN   TestArikawaGroupCommand_Invariants/load-bearing_invariants
--- PASS: TestArikawaOptionList_Bool (0.00s)
    --- PASS: TestArikawaOptionList_Bool/Happy_Path (0.00s)
    --- PASS: TestArikawaOptionList_Bool/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_Bool/Type_Mismatch (0.00s)
    --- PASS: TestArikawaOptionList_Bool/Nil_Value (0.00s)
=== PAUSE TestArikawaGroupCommand_Handle/fails_on_empty_options
=== PAUSE TestArikawaGroupCommand_Invariants/load-bearing_invariants
=== RUN   TestArikawaOptionList_String/Missing_Key
=== CONT  TestCommandSyncer_BuildCreateData
=== RUN   TestGetArikawaSubCommandOptions/Level_1_Subcommand
=== RUN   TestArikawaGroupCommand_Options/flat_resolution
=== CONT  TestNewArikawaContext_InitializationAndFailFast
=== PAUSE TestArikawaGroupCommand_Options/flat_resolution
=== CONT  TestResolveFeatureForCommandPath
--- PASS: TestArikawaContext_ContextResolution (0.00s)
=== CONT  TestCommandSyncer_SyncBulkOverwrite_Routing
=== RUN   TestResolveFeatureForCommandPath/Moderation_prefix
=== RUN   TestCommandSyncer_SyncBulkOverwrite_Routing/Global_Sync
=== RUN   TestGetArikawaSubCommandOptions/Level_2_Subcommand_Group
=== PAUSE TestCommandSyncer_SyncBulkOverwrite_Routing/Global_Sync
=== RUN   TestCommandSyncer_BuildCreateData/Cenário_B_(Fallback/Omissão)
=== RUN   TestNewArikawaContext_InitializationAndFailFast/Valid_Interaction
=== RUN   TestArikawaGroupCommand_Options/nested_group_resolution
=== RUN   TestGetArikawaSubCommandOptions/Nil_Interaction
=== PAUSE TestNewArikawaContext_InitializationAndFailFast/Valid_Interaction
=== RUN   TestResolveFeatureForCommandPath/QOTD_prefix
=== RUN   TestCommandSyncer_SyncBulkOverwrite_Routing/Guild_Sync_Dinâmico
=== PAUSE TestCommandSyncer_BuildCreateData/Cenário_B_(Fallback/Omissão)
=== RUN   TestArikawaOptionList_String/Nil_Value
=== RUN   TestCommandSyncer_BuildCreateData/Cenário_A_(Implementação_Completa)
=== PAUSE TestArikawaGroupCommand_Options/nested_group_resolution
=== RUN   TestNewArikawaContext_InitializationAndFailFast/Invalid_Event_Data_-_SenderID_0
=== PAUSE TestCommandSyncer_SyncBulkOverwrite_Routing/Guild_Sync_Dinâmico
=== CONT  TestArikawaOptionList_RoleID
--- PASS: TestGetArikawaSubCommandOptions (0.00s)
    --- PASS: TestGetArikawaSubCommandOptions/Invalid_Type_Assertion_(Ping_Interaction) (0.00s)
    --- PASS: TestGetArikawaSubCommandOptions/Flat_Command_(No_Subcommands) (0.00s)
    --- PASS: TestGetArikawaSubCommandOptions/Level_1_Subcommand (0.00s)
    --- PASS: TestGetArikawaSubCommandOptions/Level_2_Subcommand_Group (0.00s)
    --- PASS: TestGetArikawaSubCommandOptions/Nil_Interaction (0.00s)
=== PAUSE TestCommandSyncer_BuildCreateData/Cenário_A_(Implementação_Completa)
--- PASS: TestArikawaOptionList_String (0.00s)
    --- PASS: TestArikawaOptionList_String/Happy_Path (0.00s)
    --- PASS: TestArikawaOptionList_String/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_String/Nil_Value (0.00s)
=== CONT  TestCommandSyncer_Diff
=== PAUSE TestNewArikawaContext_InitializationAndFailFast/Invalid_Event_Data_-_SenderID_0
=== CONT  TestNewArikawaMissingConfigErrorData
=== RUN   TestResolveFeatureForCommandPath/Role_management_prefix
=== RUN   TestArikawaOptionList_RoleID/Happy_Path
=== CONT  TestCommandRouter_RouteInteraction/Unregistered_Command_Fallback
=== RUN   TestResolveFeatureForCommandPath/Partner_prefix
=== RUN   TestArikawaOptionList_RoleID/Missing_Key
=== CONT  TestCommandRouter_RouteInteraction/Nil_Interaction_Protection
=== RUN   TestResolveFeatureForCommandPath/Embed_prefix
=== CONT  TestArikawaGroupCommand_Handle/fails_on_empty_options
=== RUN   TestArikawaOptionList_RoleID/Type_Mismatch
=== CONT  TestArikawaGroupCommand_Handle/delegates_to_correct_subcommand
=== RUN   TestResolveFeatureForCommandPath/Ticket_prefix
=== CONT  TestCommandRouter_RouteInteraction/Valid_Slash_Command_Routing
=== RUN   TestArikawaOptionList_RoleID/Nil_Value
--- PASS: TestArikawaOptionList_RoleID (0.00s)
    --- PASS: TestArikawaOptionList_RoleID/Happy_Path (0.00s)
    --- PASS: TestArikawaOptionList_RoleID/Missing_Key (0.00s)
    --- PASS: TestArikawaOptionList_RoleID/Type_Mismatch (0.00s)
    --- PASS: TestArikawaOptionList_RoleID/Nil_Value (0.00s)
=== CONT  TestArikawaGroupCommand_Invariants/overwriting_protection
=== RUN   TestResolveFeatureForCommandPath/Stats_prefix
=== CONT  TestArikawaGroupCommand_Handle/returns_error_on_unknown_subcommand
=== CONT  TestArikawaGroupCommand_Invariants/load-bearing_invariants
=== RUN   TestNewArikawaMissingConfigErrorData/standard_feature_missing
=== CONT  TestArikawaGroupCommand_Handle/fails_on_invalid_type_assertion
--- PASS: TestCommandRouter_RouteInteraction (0.00s)
    --- PASS: TestCommandRouter_RouteInteraction/Unregistered_Command_Fallback (0.00s)
    --- PASS: TestCommandRouter_RouteInteraction/Nil_Interaction_Protection (0.00s)
    --- PASS: TestCommandRouter_RouteInteraction/Valid_Slash_Command_Routing (0.00s)
=== PAUSE TestNewArikawaMissingConfigErrorData/standard_feature_missing
=== RUN   TestResolveFeatureForCommandPath/Exact_match_without_args
=== CONT  TestArikawaGroupCommand_Options/nested_group_resolution
=== RUN   TestNewArikawaMissingConfigErrorData/ignored_parameters_do_not_mutate_output
=== PAUSE TestNewArikawaMissingConfigErrorData/ignored_parameters_do_not_mutate_output
=== CONT  TestArikawaGroupCommand_Options/empty_state
=== RUN   TestResolveFeatureForCommandPath/Unknown_path_triggers_fallback
--- PASS: TestArikawaGroupCommand_Handle (0.00s)
    --- PASS: TestArikawaGroupCommand_Handle/fails_on_empty_options (0.00s)
    --- PASS: TestArikawaGroupCommand_Handle/returns_error_on_unknown_subcommand (0.00s)
    --- PASS: TestArikawaGroupCommand_Handle/fails_on_invalid_type_assertion (0.00s)
    --- PASS: TestArikawaGroupCommand_Handle/delegates_to_correct_subcommand (0.00s)
=== RUN   TestNewArikawaMissingConfigErrorData/empty_feature_string_edge_case
=== PAUSE TestNewArikawaMissingConfigErrorData/empty_feature_string_edge_case
=== CONT  TestArikawaGroupCommand_Options/flat_resolution
=== RUN   TestNewArikawaMissingConfigErrorData/special_characters_in_feature
=== RUN   TestResolveFeatureForCommandPath/Empty_string
=== CONT  TestArikawaGroupCommand_Invariants/memory_initialization
=== CONT  TestCommandSyncer_SyncBulkOverwrite_Routing/Guild_Sync_Dinâmico
=== PAUSE TestNewArikawaMissingConfigErrorData/special_characters_in_feature
=== CONT  TestCommandSyncer_BuildCreateData/Cenário_B_(Fallback/Omissão)
--- PASS: TestCommandSyncer_Diff (0.00s)
=== CONT  TestCommandSyncer_BuildCreateData/Cenário_A_(Implementação_Completa)
--- PASS: TestArikawaGroupCommand_Invariants (0.00s)
    --- PASS: TestArikawaGroupCommand_Invariants/load-bearing_invariants (0.00s)
    --- PASS: TestArikawaGroupCommand_Invariants/overwriting_protection (0.00s)
    --- PASS: TestArikawaGroupCommand_Invariants/memory_initialization (0.00s)
=== CONT  TestNewArikawaContext_InitializationAndFailFast/Valid_Interaction
=== CONT  TestNewArikawaMissingConfigErrorData/ignored_parameters_do_not_mutate_output
=== CONT  TestCommandSyncer_SyncBulkOverwrite_Routing/Global_Sync
=== CONT  TestNewArikawaMissingConfigErrorData/standard_feature_missing
--- PASS: TestCommandSyncer_BuildCreateData (0.00s)
    --- PASS: TestCommandSyncer_BuildCreateData/Cenário_B_(Fallback/Omissão) (0.00s)
    --- PASS: TestCommandSyncer_BuildCreateData/Cenário_A_(Implementação_Completa) (0.00s)
=== CONT  TestNewArikawaMissingConfigErrorData/special_characters_in_feature
=== RUN   TestResolveFeatureForCommandPath/Malformed_payload
=== CONT  TestNewArikawaMissingConfigErrorData/empty_feature_string_edge_case
--- PASS: TestArikawaGroupCommand_Options (0.00s)
    --- PASS: TestArikawaGroupCommand_Options/empty_state (0.00s)
    --- PASS: TestArikawaGroupCommand_Options/nested_group_resolution (0.00s)
    --- PASS: TestArikawaGroupCommand_Options/flat_resolution (0.00s)
=== CONT  TestNewArikawaContext_InitializationAndFailFast/Invalid_Event_Data_-_SenderID_0
--- PASS: TestNewArikawaMissingConfigErrorData (0.00s)
    --- PASS: TestNewArikawaMissingConfigErrorData/ignored_parameters_do_not_mutate_output (0.00s)
    --- PASS: TestNewArikawaMissingConfigErrorData/standard_feature_missing (0.00s)
    --- PASS: TestNewArikawaMissingConfigErrorData/special_characters_in_feature (0.00s)
    --- PASS: TestNewArikawaMissingConfigErrorData/empty_feature_string_edge_case (0.00s)
--- PASS: TestResolveFeatureForCommandPath (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Moderation_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/QOTD_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Role_management_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Partner_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Embed_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Ticket_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Stats_prefix (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Exact_match_without_args (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Unknown_path_triggers_fallback (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Empty_string (0.00s)
    --- PASS: TestResolveFeatureForCommandPath/Malformed_payload (0.00s)
--- PASS: TestCommandSyncer_SyncBulkOverwrite_Routing (0.00s)
    --- PASS: TestCommandSyncer_SyncBulkOverwrite_Routing/Guild_Sync_Dinâmico (0.00s)
    --- PASS: TestCommandSyncer_SyncBulkOverwrite_Routing/Global_Sync (0.00s)
--- PASS: TestNewArikawaContext_InitializationAndFailFast (0.00s)
    --- PASS: TestNewArikawaContext_InitializationAndFailFast/Valid_Interaction (0.00s)
    --- PASS: TestNewArikawaContext_InitializationAndFailFast/Invalid_Event_Data_-_SenderID_0 (0.00s)
--- PASS: TestCommandRegistry_ConcurrentSafety (0.04s)
=== RUN   FuzzArikawaOptionList_String
=== RUN   FuzzArikawaOptionList_String/seed#0
=== RUN   FuzzArikawaOptionList_String/seed#1
=== RUN   FuzzArikawaOptionList_String/seed#2
--- PASS: FuzzArikawaOptionList_String (0.00s)
    --- PASS: FuzzArikawaOptionList_String/seed#0 (0.00s)
    --- PASS: FuzzArikawaOptionList_String/seed#1 (0.00s)
    --- PASS: FuzzArikawaOptionList_String/seed#2 (0.00s)
=== RUN   FuzzArikawaOptionList_AllTypes
=== RUN   FuzzArikawaOptionList_AllTypes/seed#0
=== RUN   FuzzArikawaOptionList_AllTypes/seed#1
=== RUN   FuzzArikawaOptionList_AllTypes/seed#2
--- PASS: FuzzArikawaOptionList_AllTypes (0.00s)
    --- PASS: FuzzArikawaOptionList_AllTypes/seed#0 (0.00s)
    --- PASS: FuzzArikawaOptionList_AllTypes/seed#1 (0.00s)
    --- PASS: FuzzArikawaOptionList_AllTypes/seed#2 (0.00s)
=== RUN   FuzzResolveFeatureForCommandPath
=== RUN   FuzzResolveFeatureForCommandPath/seed#0
=== RUN   FuzzResolveFeatureForCommandPath/seed#1
=== RUN   FuzzResolveFeatureForCommandPath/seed#2
=== RUN   FuzzResolveFeatureForCommandPath/seed#3
--- PASS: FuzzResolveFeatureForCommandPath (0.00s)
    --- PASS: FuzzResolveFeatureForCommandPath/seed#0 (0.00s)
    --- PASS: FuzzResolveFeatureForCommandPath/seed#1 (0.00s)
    --- PASS: FuzzResolveFeatureForCommandPath/seed#2 (0.00s)
    --- PASS: FuzzResolveFeatureForCommandPath/seed#3 (0.00s)
=== RUN   FuzzCommandSyncer_BuildCreateData
=== RUN   FuzzCommandSyncer_BuildCreateData/seed#0
=== RUN   FuzzCommandSyncer_BuildCreateData/seed#1
=== RUN   FuzzCommandSyncer_BuildCreateData/seed#2
=== RUN   FuzzCommandSyncer_BuildCreateData/seed#3
--- PASS: FuzzCommandSyncer_BuildCreateData (0.00s)
    --- PASS: FuzzCommandSyncer_BuildCreateData/seed#0 (0.00s)
    --- PASS: FuzzCommandSyncer_BuildCreateData/seed#1 (0.00s)
    --- PASS: FuzzCommandSyncer_BuildCreateData/seed#2 (0.00s)
    --- PASS: FuzzCommandSyncer_BuildCreateData/seed#3 (0.00s)
=== RUN   FuzzContextBuilder_PayloadResilience
=== RUN   FuzzContextBuilder_PayloadResilience/seed#0
=== RUN   FuzzContextBuilder_PayloadResilience/seed#1
=== RUN   FuzzContextBuilder_PayloadResilience/seed#2
--- PASS: FuzzContextBuilder_PayloadResilience (0.00s)
    --- PASS: FuzzContextBuilder_PayloadResilience/seed#0 (0.00s)
    --- PASS: FuzzContextBuilder_PayloadResilience/seed#1 (0.00s)
    --- PASS: FuzzContextBuilder_PayloadResilience/seed#2 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands	1.283s
=== RUN   TestArikawaCleanCommand_SyntheticPayloadInjection
=== PAUSE TestArikawaCleanCommand_SyntheticPayloadInjection
=== RUN   TestArikawaCleanCommand_StatelessExecution
=== PAUSE TestArikawaCleanCommand_StatelessExecution
=== CONT  TestArikawaCleanCommand_StatelessExecution
--- PASS: TestArikawaCleanCommand_StatelessExecution (0.00s)
=== CONT  TestArikawaCleanCommand_SyntheticPayloadInjection
2026/06/23 22:07:50 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:07:50 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestArikawaCleanCommand_SyntheticPayloadInjection (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/clean	1.171s
=== RUN   TestContext_StringOption
=== PAUSE TestContext_StringOption
=== RUN   TestContext_HasRole
=== PAUSE TestContext_HasRole
=== RUN   TestDispatcher_ValidCommand
=== PAUSE TestDispatcher_ValidCommand
=== RUN   TestErrors_Operational
=== PAUSE TestErrors_Operational
=== RUN   TestErrors_Validation
=== PAUSE TestErrors_Validation
=== RUN   TestRegistry_SyncMock
=== PAUSE TestRegistry_SyncMock
=== RUN   TestRegistry_ParallelReads
=== PAUSE TestRegistry_ParallelReads
=== CONT  TestContext_HasRole
=== CONT  TestErrors_Operational
--- PASS: TestContext_HasRole (0.00s)
=== RUN   TestErrors_Operational/Network_Timeout
=== CONT  TestContext_StringOption
=== CONT  TestDispatcher_ValidCommand
=== CONT  TestRegistry_SyncMock
=== CONT  TestRegistry_ParallelReads
2026/06/23 22:07:50 INFO Syncing commands to Discord operation=registry.sync appID=1 count=1
=== RUN   TestRegistry_ParallelReads/parallel_read_0
--- PASS: TestContext_StringOption (0.00s)
--- PASS: TestRegistry_SyncMock (0.00s)
=== CONT  TestErrors_Validation
=== RUN   TestErrors_Operational/DB_Error
=== PAUSE TestRegistry_ParallelReads/parallel_read_0
--- PASS: TestErrors_Validation (0.00s)
=== RUN   TestRegistry_ParallelReads/parallel_read_1
--- PASS: TestErrors_Operational (0.00s)
    --- PASS: TestErrors_Operational/Network_Timeout (0.00s)
    --- PASS: TestErrors_Operational/DB_Error (0.00s)
=== PAUSE TestRegistry_ParallelReads/parallel_read_1
=== RUN   TestRegistry_ParallelReads/parallel_read_2
=== PAUSE TestRegistry_ParallelReads/parallel_read_2
=== RUN   TestRegistry_ParallelReads/parallel_read_3
=== PAUSE TestRegistry_ParallelReads/parallel_read_3
=== RUN   TestRegistry_ParallelReads/parallel_read_4
=== PAUSE TestRegistry_ParallelReads/parallel_read_4
=== RUN   TestRegistry_ParallelReads/parallel_read_5
=== PAUSE TestRegistry_ParallelReads/parallel_read_5
=== RUN   TestRegistry_ParallelReads/parallel_read_6
=== PAUSE TestRegistry_ParallelReads/parallel_read_6
=== RUN   TestRegistry_ParallelReads/parallel_read_7
=== PAUSE TestRegistry_ParallelReads/parallel_read_7
=== RUN   TestRegistry_ParallelReads/parallel_read_8
=== PAUSE TestRegistry_ParallelReads/parallel_read_8
=== RUN   TestRegistry_ParallelReads/parallel_read_9
=== PAUSE TestRegistry_ParallelReads/parallel_read_9
=== RUN   TestRegistry_ParallelReads/parallel_read_10
=== PAUSE TestRegistry_ParallelReads/parallel_read_10
=== RUN   TestRegistry_ParallelReads/parallel_read_11
=== PAUSE TestRegistry_ParallelReads/parallel_read_11
=== RUN   TestRegistry_ParallelReads/parallel_read_12
=== PAUSE TestRegistry_ParallelReads/parallel_read_12
=== RUN   TestRegistry_ParallelReads/parallel_read_13
=== PAUSE TestRegistry_ParallelReads/parallel_read_13
=== RUN   TestRegistry_ParallelReads/parallel_read_14
=== PAUSE TestRegistry_ParallelReads/parallel_read_14
=== RUN   TestRegistry_ParallelReads/parallel_read_15
=== PAUSE TestRegistry_ParallelReads/parallel_read_15
=== RUN   TestRegistry_ParallelReads/parallel_read_16
=== PAUSE TestRegistry_ParallelReads/parallel_read_16
=== RUN   TestRegistry_ParallelReads/parallel_read_17
=== PAUSE TestRegistry_ParallelReads/parallel_read_17
=== RUN   TestRegistry_ParallelReads/parallel_read_18
=== PAUSE TestRegistry_ParallelReads/parallel_read_18
=== RUN   TestRegistry_ParallelReads/parallel_read_19
=== PAUSE TestRegistry_ParallelReads/parallel_read_19
=== RUN   TestRegistry_ParallelReads/parallel_read_20
=== PAUSE TestRegistry_ParallelReads/parallel_read_20
=== RUN   TestRegistry_ParallelReads/parallel_read_21
=== PAUSE TestRegistry_ParallelReads/parallel_read_21
=== RUN   TestRegistry_ParallelReads/parallel_read_22
=== PAUSE TestRegistry_ParallelReads/parallel_read_22
=== RUN   TestRegistry_ParallelReads/parallel_read_23
=== PAUSE TestRegistry_ParallelReads/parallel_read_23
=== RUN   TestRegistry_ParallelReads/parallel_read_24
=== PAUSE TestRegistry_ParallelReads/parallel_read_24
=== RUN   TestRegistry_ParallelReads/parallel_read_25
=== PAUSE TestRegistry_ParallelReads/parallel_read_25
=== RUN   TestRegistry_ParallelReads/parallel_read_26
=== PAUSE TestRegistry_ParallelReads/parallel_read_26
=== RUN   TestRegistry_ParallelReads/parallel_read_27
=== PAUSE TestRegistry_ParallelReads/parallel_read_27
=== RUN   TestRegistry_ParallelReads/parallel_read_28
=== PAUSE TestRegistry_ParallelReads/parallel_read_28
=== RUN   TestRegistry_ParallelReads/parallel_read_29
=== PAUSE TestRegistry_ParallelReads/parallel_read_29
=== RUN   TestRegistry_ParallelReads/parallel_read_30
=== PAUSE TestRegistry_ParallelReads/parallel_read_30
=== RUN   TestRegistry_ParallelReads/parallel_read_31
=== PAUSE TestRegistry_ParallelReads/parallel_read_31
=== RUN   TestRegistry_ParallelReads/parallel_read_32
=== PAUSE TestRegistry_ParallelReads/parallel_read_32
=== RUN   TestRegistry_ParallelReads/parallel_read_33
=== PAUSE TestRegistry_ParallelReads/parallel_read_33
=== RUN   TestRegistry_ParallelReads/parallel_read_34
=== PAUSE TestRegistry_ParallelReads/parallel_read_34
=== RUN   TestRegistry_ParallelReads/parallel_read_35
=== PAUSE TestRegistry_ParallelReads/parallel_read_35
=== RUN   TestRegistry_ParallelReads/parallel_read_36
=== PAUSE TestRegistry_ParallelReads/parallel_read_36
=== RUN   TestRegistry_ParallelReads/parallel_read_37
=== PAUSE TestRegistry_ParallelReads/parallel_read_37
=== RUN   TestRegistry_ParallelReads/parallel_read_38
=== PAUSE TestRegistry_ParallelReads/parallel_read_38
=== RUN   TestRegistry_ParallelReads/parallel_read_39
=== PAUSE TestRegistry_ParallelReads/parallel_read_39
=== RUN   TestRegistry_ParallelReads/parallel_read_40
=== PAUSE TestRegistry_ParallelReads/parallel_read_40
=== RUN   TestRegistry_ParallelReads/parallel_read_41
=== PAUSE TestRegistry_ParallelReads/parallel_read_41
=== RUN   TestRegistry_ParallelReads/parallel_read_42
=== PAUSE TestRegistry_ParallelReads/parallel_read_42
=== RUN   TestRegistry_ParallelReads/parallel_read_43
=== PAUSE TestRegistry_ParallelReads/parallel_read_43
=== RUN   TestRegistry_ParallelReads/parallel_read_44
=== PAUSE TestRegistry_ParallelReads/parallel_read_44
=== RUN   TestRegistry_ParallelReads/parallel_read_45
=== PAUSE TestRegistry_ParallelReads/parallel_read_45
=== RUN   TestRegistry_ParallelReads/parallel_read_46
--- PASS: TestDispatcher_ValidCommand (0.00s)
=== PAUSE TestRegistry_ParallelReads/parallel_read_46
=== RUN   TestRegistry_ParallelReads/parallel_read_47
=== PAUSE TestRegistry_ParallelReads/parallel_read_47
=== RUN   TestRegistry_ParallelReads/parallel_read_48
=== PAUSE TestRegistry_ParallelReads/parallel_read_48
=== RUN   TestRegistry_ParallelReads/parallel_read_49
=== PAUSE TestRegistry_ParallelReads/parallel_read_49
=== RUN   TestRegistry_ParallelReads/parallel_read_50
=== PAUSE TestRegistry_ParallelReads/parallel_read_50
=== RUN   TestRegistry_ParallelReads/parallel_read_51
=== PAUSE TestRegistry_ParallelReads/parallel_read_51
=== RUN   TestRegistry_ParallelReads/parallel_read_52
=== PAUSE TestRegistry_ParallelReads/parallel_read_52
=== RUN   TestRegistry_ParallelReads/parallel_read_53
=== PAUSE TestRegistry_ParallelReads/parallel_read_53
=== RUN   TestRegistry_ParallelReads/parallel_read_54
=== PAUSE TestRegistry_ParallelReads/parallel_read_54
=== RUN   TestRegistry_ParallelReads/parallel_read_55
=== PAUSE TestRegistry_ParallelReads/parallel_read_55
=== RUN   TestRegistry_ParallelReads/parallel_read_56
=== PAUSE TestRegistry_ParallelReads/parallel_read_56
=== RUN   TestRegistry_ParallelReads/parallel_read_57
=== PAUSE TestRegistry_ParallelReads/parallel_read_57
=== RUN   TestRegistry_ParallelReads/parallel_read_58
=== PAUSE TestRegistry_ParallelReads/parallel_read_58
=== RUN   TestRegistry_ParallelReads/parallel_read_59
=== PAUSE TestRegistry_ParallelReads/parallel_read_59
=== RUN   TestRegistry_ParallelReads/parallel_read_60
=== PAUSE TestRegistry_ParallelReads/parallel_read_60
=== RUN   TestRegistry_ParallelReads/parallel_read_61
=== PAUSE TestRegistry_ParallelReads/parallel_read_61
=== RUN   TestRegistry_ParallelReads/parallel_read_62
=== PAUSE TestRegistry_ParallelReads/parallel_read_62
=== RUN   TestRegistry_ParallelReads/parallel_read_63
=== PAUSE TestRegistry_ParallelReads/parallel_read_63
=== RUN   TestRegistry_ParallelReads/parallel_read_64
=== PAUSE TestRegistry_ParallelReads/parallel_read_64
=== RUN   TestRegistry_ParallelReads/parallel_read_65
=== PAUSE TestRegistry_ParallelReads/parallel_read_65
=== RUN   TestRegistry_ParallelReads/parallel_read_66
=== PAUSE TestRegistry_ParallelReads/parallel_read_66
=== RUN   TestRegistry_ParallelReads/parallel_read_67
=== PAUSE TestRegistry_ParallelReads/parallel_read_67
=== RUN   TestRegistry_ParallelReads/parallel_read_68
=== PAUSE TestRegistry_ParallelReads/parallel_read_68
=== RUN   TestRegistry_ParallelReads/parallel_read_69
=== PAUSE TestRegistry_ParallelReads/parallel_read_69
=== RUN   TestRegistry_ParallelReads/parallel_read_70
=== PAUSE TestRegistry_ParallelReads/parallel_read_70
=== RUN   TestRegistry_ParallelReads/parallel_read_71
=== PAUSE TestRegistry_ParallelReads/parallel_read_71
=== RUN   TestRegistry_ParallelReads/parallel_read_72
=== PAUSE TestRegistry_ParallelReads/parallel_read_72
=== RUN   TestRegistry_ParallelReads/parallel_read_73
=== PAUSE TestRegistry_ParallelReads/parallel_read_73
=== RUN   TestRegistry_ParallelReads/parallel_read_74
=== PAUSE TestRegistry_ParallelReads/parallel_read_74
=== RUN   TestRegistry_ParallelReads/parallel_read_75
=== PAUSE TestRegistry_ParallelReads/parallel_read_75
=== RUN   TestRegistry_ParallelReads/parallel_read_76
=== PAUSE TestRegistry_ParallelReads/parallel_read_76
=== RUN   TestRegistry_ParallelReads/parallel_read_77
=== PAUSE TestRegistry_ParallelReads/parallel_read_77
=== RUN   TestRegistry_ParallelReads/parallel_read_78
=== PAUSE TestRegistry_ParallelReads/parallel_read_78
=== RUN   TestRegistry_ParallelReads/parallel_read_79
=== PAUSE TestRegistry_ParallelReads/parallel_read_79
=== RUN   TestRegistry_ParallelReads/parallel_read_80
=== PAUSE TestRegistry_ParallelReads/parallel_read_80
=== RUN   TestRegistry_ParallelReads/parallel_read_81
=== PAUSE TestRegistry_ParallelReads/parallel_read_81
=== RUN   TestRegistry_ParallelReads/parallel_read_82
=== PAUSE TestRegistry_ParallelReads/parallel_read_82
=== RUN   TestRegistry_ParallelReads/parallel_read_83
=== PAUSE TestRegistry_ParallelReads/parallel_read_83
=== RUN   TestRegistry_ParallelReads/parallel_read_84
=== PAUSE TestRegistry_ParallelReads/parallel_read_84
=== RUN   TestRegistry_ParallelReads/parallel_read_85
=== PAUSE TestRegistry_ParallelReads/parallel_read_85
=== RUN   TestRegistry_ParallelReads/parallel_read_86
=== PAUSE TestRegistry_ParallelReads/parallel_read_86
=== RUN   TestRegistry_ParallelReads/parallel_read_87
=== PAUSE TestRegistry_ParallelReads/parallel_read_87
=== RUN   TestRegistry_ParallelReads/parallel_read_88
=== PAUSE TestRegistry_ParallelReads/parallel_read_88
=== RUN   TestRegistry_ParallelReads/parallel_read_89
=== PAUSE TestRegistry_ParallelReads/parallel_read_89
=== RUN   TestRegistry_ParallelReads/parallel_read_90
=== PAUSE TestRegistry_ParallelReads/parallel_read_90
=== RUN   TestRegistry_ParallelReads/parallel_read_91
=== PAUSE TestRegistry_ParallelReads/parallel_read_91
=== RUN   TestRegistry_ParallelReads/parallel_read_92
=== PAUSE TestRegistry_ParallelReads/parallel_read_92
=== RUN   TestRegistry_ParallelReads/parallel_read_93
=== PAUSE TestRegistry_ParallelReads/parallel_read_93
=== RUN   TestRegistry_ParallelReads/parallel_read_94
=== PAUSE TestRegistry_ParallelReads/parallel_read_94
=== RUN   TestRegistry_ParallelReads/parallel_read_95
=== PAUSE TestRegistry_ParallelReads/parallel_read_95
=== RUN   TestRegistry_ParallelReads/parallel_read_96
=== PAUSE TestRegistry_ParallelReads/parallel_read_96
=== RUN   TestRegistry_ParallelReads/parallel_read_97
=== PAUSE TestRegistry_ParallelReads/parallel_read_97
=== RUN   TestRegistry_ParallelReads/parallel_read_98
=== PAUSE TestRegistry_ParallelReads/parallel_read_98
=== RUN   TestRegistry_ParallelReads/parallel_read_99
=== PAUSE TestRegistry_ParallelReads/parallel_read_99
=== RUN   TestRegistry_ParallelReads/parallel_read_100
=== PAUSE TestRegistry_ParallelReads/parallel_read_100
=== RUN   TestRegistry_ParallelReads/parallel_read_101
=== PAUSE TestRegistry_ParallelReads/parallel_read_101
=== RUN   TestRegistry_ParallelReads/parallel_read_102
=== PAUSE TestRegistry_ParallelReads/parallel_read_102
=== RUN   TestRegistry_ParallelReads/parallel_read_103
=== PAUSE TestRegistry_ParallelReads/parallel_read_103
=== RUN   TestRegistry_ParallelReads/parallel_read_104
=== PAUSE TestRegistry_ParallelReads/parallel_read_104
=== RUN   TestRegistry_ParallelReads/parallel_read_105
=== PAUSE TestRegistry_ParallelReads/parallel_read_105
=== RUN   TestRegistry_ParallelReads/parallel_read_106
=== PAUSE TestRegistry_ParallelReads/parallel_read_106
=== RUN   TestRegistry_ParallelReads/parallel_read_107
=== PAUSE TestRegistry_ParallelReads/parallel_read_107
=== RUN   TestRegistry_ParallelReads/parallel_read_108
=== PAUSE TestRegistry_ParallelReads/parallel_read_108
=== RUN   TestRegistry_ParallelReads/parallel_read_109
=== PAUSE TestRegistry_ParallelReads/parallel_read_109
=== RUN   TestRegistry_ParallelReads/parallel_read_110
=== PAUSE TestRegistry_ParallelReads/parallel_read_110
=== RUN   TestRegistry_ParallelReads/parallel_read_111
=== PAUSE TestRegistry_ParallelReads/parallel_read_111
=== RUN   TestRegistry_ParallelReads/parallel_read_112
=== PAUSE TestRegistry_ParallelReads/parallel_read_112
=== RUN   TestRegistry_ParallelReads/parallel_read_113
=== PAUSE TestRegistry_ParallelReads/parallel_read_113
=== RUN   TestRegistry_ParallelReads/parallel_read_114
=== PAUSE TestRegistry_ParallelReads/parallel_read_114
=== RUN   TestRegistry_ParallelReads/parallel_read_115
=== PAUSE TestRegistry_ParallelReads/parallel_read_115
=== RUN   TestRegistry_ParallelReads/parallel_read_116
=== PAUSE TestRegistry_ParallelReads/parallel_read_116
=== RUN   TestRegistry_ParallelReads/parallel_read_117
=== PAUSE TestRegistry_ParallelReads/parallel_read_117
=== RUN   TestRegistry_ParallelReads/parallel_read_118
=== PAUSE TestRegistry_ParallelReads/parallel_read_118
=== RUN   TestRegistry_ParallelReads/parallel_read_119
=== PAUSE TestRegistry_ParallelReads/parallel_read_119
=== RUN   TestRegistry_ParallelReads/parallel_read_120
=== PAUSE TestRegistry_ParallelReads/parallel_read_120
=== RUN   TestRegistry_ParallelReads/parallel_read_121
=== PAUSE TestRegistry_ParallelReads/parallel_read_121
=== RUN   TestRegistry_ParallelReads/parallel_read_122
=== PAUSE TestRegistry_ParallelReads/parallel_read_122
=== RUN   TestRegistry_ParallelReads/parallel_read_123
=== PAUSE TestRegistry_ParallelReads/parallel_read_123
=== RUN   TestRegistry_ParallelReads/parallel_read_124
=== PAUSE TestRegistry_ParallelReads/parallel_read_124
=== RUN   TestRegistry_ParallelReads/parallel_read_125
=== PAUSE TestRegistry_ParallelReads/parallel_read_125
=== RUN   TestRegistry_ParallelReads/parallel_read_126
=== PAUSE TestRegistry_ParallelReads/parallel_read_126
=== RUN   TestRegistry_ParallelReads/parallel_read_127
=== PAUSE TestRegistry_ParallelReads/parallel_read_127
=== RUN   TestRegistry_ParallelReads/parallel_read_128
=== PAUSE TestRegistry_ParallelReads/parallel_read_128
=== RUN   TestRegistry_ParallelReads/parallel_read_129
=== PAUSE TestRegistry_ParallelReads/parallel_read_129
=== RUN   TestRegistry_ParallelReads/parallel_read_130
=== PAUSE TestRegistry_ParallelReads/parallel_read_130
=== RUN   TestRegistry_ParallelReads/parallel_read_131
=== PAUSE TestRegistry_ParallelReads/parallel_read_131
=== RUN   TestRegistry_ParallelReads/parallel_read_132
=== PAUSE TestRegistry_ParallelReads/parallel_read_132
=== RUN   TestRegistry_ParallelReads/parallel_read_133
=== PAUSE TestRegistry_ParallelReads/parallel_read_133
=== RUN   TestRegistry_ParallelReads/parallel_read_134
=== PAUSE TestRegistry_ParallelReads/parallel_read_134
=== RUN   TestRegistry_ParallelReads/parallel_read_135
=== PAUSE TestRegistry_ParallelReads/parallel_read_135
=== RUN   TestRegistry_ParallelReads/parallel_read_136
=== PAUSE TestRegistry_ParallelReads/parallel_read_136
=== RUN   TestRegistry_ParallelReads/parallel_read_137
=== PAUSE TestRegistry_ParallelReads/parallel_read_137
=== RUN   TestRegistry_ParallelReads/parallel_read_138
=== PAUSE TestRegistry_ParallelReads/parallel_read_138
=== RUN   TestRegistry_ParallelReads/parallel_read_139
=== PAUSE TestRegistry_ParallelReads/parallel_read_139
=== RUN   TestRegistry_ParallelReads/parallel_read_140
=== PAUSE TestRegistry_ParallelReads/parallel_read_140
=== RUN   TestRegistry_ParallelReads/parallel_read_141
=== PAUSE TestRegistry_ParallelReads/parallel_read_141
=== RUN   TestRegistry_ParallelReads/parallel_read_142
=== PAUSE TestRegistry_ParallelReads/parallel_read_142
=== RUN   TestRegistry_ParallelReads/parallel_read_143
=== PAUSE TestRegistry_ParallelReads/parallel_read_143
=== RUN   TestRegistry_ParallelReads/parallel_read_144
=== PAUSE TestRegistry_ParallelReads/parallel_read_144
=== RUN   TestRegistry_ParallelReads/parallel_read_145
=== PAUSE TestRegistry_ParallelReads/parallel_read_145
=== RUN   TestRegistry_ParallelReads/parallel_read_146
=== PAUSE TestRegistry_ParallelReads/parallel_read_146
=== RUN   TestRegistry_ParallelReads/parallel_read_147
=== PAUSE TestRegistry_ParallelReads/parallel_read_147
=== RUN   TestRegistry_ParallelReads/parallel_read_148
=== PAUSE TestRegistry_ParallelReads/parallel_read_148
=== RUN   TestRegistry_ParallelReads/parallel_read_149
=== PAUSE TestRegistry_ParallelReads/parallel_read_149
=== RUN   TestRegistry_ParallelReads/parallel_read_150
=== PAUSE TestRegistry_ParallelReads/parallel_read_150
=== RUN   TestRegistry_ParallelReads/parallel_read_151
=== PAUSE TestRegistry_ParallelReads/parallel_read_151
=== RUN   TestRegistry_ParallelReads/parallel_read_152
=== PAUSE TestRegistry_ParallelReads/parallel_read_152
=== RUN   TestRegistry_ParallelReads/parallel_read_153
=== PAUSE TestRegistry_ParallelReads/parallel_read_153
=== RUN   TestRegistry_ParallelReads/parallel_read_154
=== PAUSE TestRegistry_ParallelReads/parallel_read_154
=== RUN   TestRegistry_ParallelReads/parallel_read_155
=== PAUSE TestRegistry_ParallelReads/parallel_read_155
=== RUN   TestRegistry_ParallelReads/parallel_read_156
=== PAUSE TestRegistry_ParallelReads/parallel_read_156
=== RUN   TestRegistry_ParallelReads/parallel_read_157
=== PAUSE TestRegistry_ParallelReads/parallel_read_157
=== RUN   TestRegistry_ParallelReads/parallel_read_158
=== PAUSE TestRegistry_ParallelReads/parallel_read_158
=== RUN   TestRegistry_ParallelReads/parallel_read_159
=== PAUSE TestRegistry_ParallelReads/parallel_read_159
=== RUN   TestRegistry_ParallelReads/parallel_read_160
=== PAUSE TestRegistry_ParallelReads/parallel_read_160
=== RUN   TestRegistry_ParallelReads/parallel_read_161
=== PAUSE TestRegistry_ParallelReads/parallel_read_161
=== RUN   TestRegistry_ParallelReads/parallel_read_162
=== PAUSE TestRegistry_ParallelReads/parallel_read_162
=== RUN   TestRegistry_ParallelReads/parallel_read_163
=== PAUSE TestRegistry_ParallelReads/parallel_read_163
=== RUN   TestRegistry_ParallelReads/parallel_read_164
=== PAUSE TestRegistry_ParallelReads/parallel_read_164
=== RUN   TestRegistry_ParallelReads/parallel_read_165
=== PAUSE TestRegistry_ParallelReads/parallel_read_165
=== RUN   TestRegistry_ParallelReads/parallel_read_166
=== PAUSE TestRegistry_ParallelReads/parallel_read_166
=== RUN   TestRegistry_ParallelReads/parallel_read_167
=== PAUSE TestRegistry_ParallelReads/parallel_read_167
=== RUN   TestRegistry_ParallelReads/parallel_read_168
=== PAUSE TestRegistry_ParallelReads/parallel_read_168
=== RUN   TestRegistry_ParallelReads/parallel_read_169
=== PAUSE TestRegistry_ParallelReads/parallel_read_169
=== RUN   TestRegistry_ParallelReads/parallel_read_170
=== PAUSE TestRegistry_ParallelReads/parallel_read_170
=== RUN   TestRegistry_ParallelReads/parallel_read_171
=== PAUSE TestRegistry_ParallelReads/parallel_read_171
=== RUN   TestRegistry_ParallelReads/parallel_read_172
=== PAUSE TestRegistry_ParallelReads/parallel_read_172
=== RUN   TestRegistry_ParallelReads/parallel_read_173
=== PAUSE TestRegistry_ParallelReads/parallel_read_173
=== RUN   TestRegistry_ParallelReads/parallel_read_174
=== PAUSE TestRegistry_ParallelReads/parallel_read_174
=== RUN   TestRegistry_ParallelReads/parallel_read_175
=== PAUSE TestRegistry_ParallelReads/parallel_read_175
=== RUN   TestRegistry_ParallelReads/parallel_read_176
=== PAUSE TestRegistry_ParallelReads/parallel_read_176
=== RUN   TestRegistry_ParallelReads/parallel_read_177
=== PAUSE TestRegistry_ParallelReads/parallel_read_177
=== RUN   TestRegistry_ParallelReads/parallel_read_178
=== PAUSE TestRegistry_ParallelReads/parallel_read_178
=== RUN   TestRegistry_ParallelReads/parallel_read_179
=== PAUSE TestRegistry_ParallelReads/parallel_read_179
=== RUN   TestRegistry_ParallelReads/parallel_read_180
=== PAUSE TestRegistry_ParallelReads/parallel_read_180
=== RUN   TestRegistry_ParallelReads/parallel_read_181
=== PAUSE TestRegistry_ParallelReads/parallel_read_181
=== RUN   TestRegistry_ParallelReads/parallel_read_182
=== PAUSE TestRegistry_ParallelReads/parallel_read_182
=== RUN   TestRegistry_ParallelReads/parallel_read_183
=== PAUSE TestRegistry_ParallelReads/parallel_read_183
=== RUN   TestRegistry_ParallelReads/parallel_read_184
=== PAUSE TestRegistry_ParallelReads/parallel_read_184
=== RUN   TestRegistry_ParallelReads/parallel_read_185
=== PAUSE TestRegistry_ParallelReads/parallel_read_185
=== RUN   TestRegistry_ParallelReads/parallel_read_186
=== PAUSE TestRegistry_ParallelReads/parallel_read_186
=== RUN   TestRegistry_ParallelReads/parallel_read_187
=== PAUSE TestRegistry_ParallelReads/parallel_read_187
=== RUN   TestRegistry_ParallelReads/parallel_read_188
=== PAUSE TestRegistry_ParallelReads/parallel_read_188
=== RUN   TestRegistry_ParallelReads/parallel_read_189
=== PAUSE TestRegistry_ParallelReads/parallel_read_189
=== RUN   TestRegistry_ParallelReads/parallel_read_190
=== PAUSE TestRegistry_ParallelReads/parallel_read_190
=== RUN   TestRegistry_ParallelReads/parallel_read_191
=== PAUSE TestRegistry_ParallelReads/parallel_read_191
=== RUN   TestRegistry_ParallelReads/parallel_read_192
=== PAUSE TestRegistry_ParallelReads/parallel_read_192
=== RUN   TestRegistry_ParallelReads/parallel_read_193
=== PAUSE TestRegistry_ParallelReads/parallel_read_193
=== RUN   TestRegistry_ParallelReads/parallel_read_194
=== PAUSE TestRegistry_ParallelReads/parallel_read_194
=== RUN   TestRegistry_ParallelReads/parallel_read_195
=== PAUSE TestRegistry_ParallelReads/parallel_read_195
=== RUN   TestRegistry_ParallelReads/parallel_read_196
=== PAUSE TestRegistry_ParallelReads/parallel_read_196
=== RUN   TestRegistry_ParallelReads/parallel_read_197
=== PAUSE TestRegistry_ParallelReads/parallel_read_197
=== RUN   TestRegistry_ParallelReads/parallel_read_198
=== PAUSE TestRegistry_ParallelReads/parallel_read_198
=== RUN   TestRegistry_ParallelReads/parallel_read_199
=== PAUSE TestRegistry_ParallelReads/parallel_read_199
=== RUN   TestRegistry_ParallelReads/parallel_read_200
=== PAUSE TestRegistry_ParallelReads/parallel_read_200
=== RUN   TestRegistry_ParallelReads/parallel_read_201
=== PAUSE TestRegistry_ParallelReads/parallel_read_201
=== RUN   TestRegistry_ParallelReads/parallel_read_202
=== PAUSE TestRegistry_ParallelReads/parallel_read_202
=== RUN   TestRegistry_ParallelReads/parallel_read_203
=== PAUSE TestRegistry_ParallelReads/parallel_read_203
=== RUN   TestRegistry_ParallelReads/parallel_read_204
=== PAUSE TestRegistry_ParallelReads/parallel_read_204
=== RUN   TestRegistry_ParallelReads/parallel_read_205
=== PAUSE TestRegistry_ParallelReads/parallel_read_205
=== RUN   TestRegistry_ParallelReads/parallel_read_206
=== PAUSE TestRegistry_ParallelReads/parallel_read_206
=== RUN   TestRegistry_ParallelReads/parallel_read_207
=== PAUSE TestRegistry_ParallelReads/parallel_read_207
=== RUN   TestRegistry_ParallelReads/parallel_read_208
=== PAUSE TestRegistry_ParallelReads/parallel_read_208
=== RUN   TestRegistry_ParallelReads/parallel_read_209
=== PAUSE TestRegistry_ParallelReads/parallel_read_209
=== RUN   TestRegistry_ParallelReads/parallel_read_210
=== PAUSE TestRegistry_ParallelReads/parallel_read_210
=== RUN   TestRegistry_ParallelReads/parallel_read_211
=== PAUSE TestRegistry_ParallelReads/parallel_read_211
=== RUN   TestRegistry_ParallelReads/parallel_read_212
=== PAUSE TestRegistry_ParallelReads/parallel_read_212
=== RUN   TestRegistry_ParallelReads/parallel_read_213
=== PAUSE TestRegistry_ParallelReads/parallel_read_213
=== RUN   TestRegistry_ParallelReads/parallel_read_214
=== PAUSE TestRegistry_ParallelReads/parallel_read_214
=== RUN   TestRegistry_ParallelReads/parallel_read_215
=== PAUSE TestRegistry_ParallelReads/parallel_read_215
=== RUN   TestRegistry_ParallelReads/parallel_read_216
=== PAUSE TestRegistry_ParallelReads/parallel_read_216
=== RUN   TestRegistry_ParallelReads/parallel_read_217
=== PAUSE TestRegistry_ParallelReads/parallel_read_217
=== RUN   TestRegistry_ParallelReads/parallel_read_218
=== PAUSE TestRegistry_ParallelReads/parallel_read_218
=== RUN   TestRegistry_ParallelReads/parallel_read_219
=== PAUSE TestRegistry_ParallelReads/parallel_read_219
=== RUN   TestRegistry_ParallelReads/parallel_read_220
=== PAUSE TestRegistry_ParallelReads/parallel_read_220
=== RUN   TestRegistry_ParallelReads/parallel_read_221
=== PAUSE TestRegistry_ParallelReads/parallel_read_221
=== RUN   TestRegistry_ParallelReads/parallel_read_222
=== PAUSE TestRegistry_ParallelReads/parallel_read_222
=== RUN   TestRegistry_ParallelReads/parallel_read_223
=== PAUSE TestRegistry_ParallelReads/parallel_read_223
=== RUN   TestRegistry_ParallelReads/parallel_read_224
=== PAUSE TestRegistry_ParallelReads/parallel_read_224
=== RUN   TestRegistry_ParallelReads/parallel_read_225
=== PAUSE TestRegistry_ParallelReads/parallel_read_225
=== RUN   TestRegistry_ParallelReads/parallel_read_226
=== PAUSE TestRegistry_ParallelReads/parallel_read_226
=== RUN   TestRegistry_ParallelReads/parallel_read_227
=== PAUSE TestRegistry_ParallelReads/parallel_read_227
=== RUN   TestRegistry_ParallelReads/parallel_read_228
=== PAUSE TestRegistry_ParallelReads/parallel_read_228
=== RUN   TestRegistry_ParallelReads/parallel_read_229
=== PAUSE TestRegistry_ParallelReads/parallel_read_229
=== RUN   TestRegistry_ParallelReads/parallel_read_230
=== PAUSE TestRegistry_ParallelReads/parallel_read_230
=== RUN   TestRegistry_ParallelReads/parallel_read_231
=== PAUSE TestRegistry_ParallelReads/parallel_read_231
=== RUN   TestRegistry_ParallelReads/parallel_read_232
=== PAUSE TestRegistry_ParallelReads/parallel_read_232
=== RUN   TestRegistry_ParallelReads/parallel_read_233
=== PAUSE TestRegistry_ParallelReads/parallel_read_233
=== RUN   TestRegistry_ParallelReads/parallel_read_234
=== PAUSE TestRegistry_ParallelReads/parallel_read_234
=== RUN   TestRegistry_ParallelReads/parallel_read_235
=== PAUSE TestRegistry_ParallelReads/parallel_read_235
=== RUN   TestRegistry_ParallelReads/parallel_read_236
=== PAUSE TestRegistry_ParallelReads/parallel_read_236
=== RUN   TestRegistry_ParallelReads/parallel_read_237
=== PAUSE TestRegistry_ParallelReads/parallel_read_237
=== RUN   TestRegistry_ParallelReads/parallel_read_238
=== PAUSE TestRegistry_ParallelReads/parallel_read_238
=== RUN   TestRegistry_ParallelReads/parallel_read_239
=== PAUSE TestRegistry_ParallelReads/parallel_read_239
=== RUN   TestRegistry_ParallelReads/parallel_read_240
=== PAUSE TestRegistry_ParallelReads/parallel_read_240
=== RUN   TestRegistry_ParallelReads/parallel_read_241
=== PAUSE TestRegistry_ParallelReads/parallel_read_241
=== RUN   TestRegistry_ParallelReads/parallel_read_242
=== PAUSE TestRegistry_ParallelReads/parallel_read_242
=== RUN   TestRegistry_ParallelReads/parallel_read_243
=== PAUSE TestRegistry_ParallelReads/parallel_read_243
=== RUN   TestRegistry_ParallelReads/parallel_read_244
=== PAUSE TestRegistry_ParallelReads/parallel_read_244
=== RUN   TestRegistry_ParallelReads/parallel_read_245
=== PAUSE TestRegistry_ParallelReads/parallel_read_245
=== RUN   TestRegistry_ParallelReads/parallel_read_246
=== PAUSE TestRegistry_ParallelReads/parallel_read_246
=== RUN   TestRegistry_ParallelReads/parallel_read_247
=== PAUSE TestRegistry_ParallelReads/parallel_read_247
=== RUN   TestRegistry_ParallelReads/parallel_read_248
=== PAUSE TestRegistry_ParallelReads/parallel_read_248
=== RUN   TestRegistry_ParallelReads/parallel_read_249
=== PAUSE TestRegistry_ParallelReads/parallel_read_249
=== RUN   TestRegistry_ParallelReads/parallel_read_250
=== PAUSE TestRegistry_ParallelReads/parallel_read_250
=== RUN   TestRegistry_ParallelReads/parallel_read_251
=== PAUSE TestRegistry_ParallelReads/parallel_read_251
=== RUN   TestRegistry_ParallelReads/parallel_read_252
=== PAUSE TestRegistry_ParallelReads/parallel_read_252
=== RUN   TestRegistry_ParallelReads/parallel_read_253
=== PAUSE TestRegistry_ParallelReads/parallel_read_253
=== RUN   TestRegistry_ParallelReads/parallel_read_254
=== PAUSE TestRegistry_ParallelReads/parallel_read_254
=== RUN   TestRegistry_ParallelReads/parallel_read_255
=== PAUSE TestRegistry_ParallelReads/parallel_read_255
=== RUN   TestRegistry_ParallelReads/parallel_read_256
=== PAUSE TestRegistry_ParallelReads/parallel_read_256
=== RUN   TestRegistry_ParallelReads/parallel_read_257
=== PAUSE TestRegistry_ParallelReads/parallel_read_257
=== RUN   TestRegistry_ParallelReads/parallel_read_258
=== PAUSE TestRegistry_ParallelReads/parallel_read_258
=== RUN   TestRegistry_ParallelReads/parallel_read_259
=== PAUSE TestRegistry_ParallelReads/parallel_read_259
=== RUN   TestRegistry_ParallelReads/parallel_read_260
=== PAUSE TestRegistry_ParallelReads/parallel_read_260
=== RUN   TestRegistry_ParallelReads/parallel_read_261
=== PAUSE TestRegistry_ParallelReads/parallel_read_261
=== RUN   TestRegistry_ParallelReads/parallel_read_262
=== PAUSE TestRegistry_ParallelReads/parallel_read_262
=== RUN   TestRegistry_ParallelReads/parallel_read_263
=== PAUSE TestRegistry_ParallelReads/parallel_read_263
=== RUN   TestRegistry_ParallelReads/parallel_read_264
=== PAUSE TestRegistry_ParallelReads/parallel_read_264
=== RUN   TestRegistry_ParallelReads/parallel_read_265
=== PAUSE TestRegistry_ParallelReads/parallel_read_265
=== RUN   TestRegistry_ParallelReads/parallel_read_266
=== PAUSE TestRegistry_ParallelReads/parallel_read_266
=== RUN   TestRegistry_ParallelReads/parallel_read_267
=== PAUSE TestRegistry_ParallelReads/parallel_read_267
=== RUN   TestRegistry_ParallelReads/parallel_read_268
=== PAUSE TestRegistry_ParallelReads/parallel_read_268
=== RUN   TestRegistry_ParallelReads/parallel_read_269
=== PAUSE TestRegistry_ParallelReads/parallel_read_269
=== RUN   TestRegistry_ParallelReads/parallel_read_270
=== PAUSE TestRegistry_ParallelReads/parallel_read_270
=== RUN   TestRegistry_ParallelReads/parallel_read_271
=== PAUSE TestRegistry_ParallelReads/parallel_read_271
=== RUN   TestRegistry_ParallelReads/parallel_read_272
=== PAUSE TestRegistry_ParallelReads/parallel_read_272
=== RUN   TestRegistry_ParallelReads/parallel_read_273
=== PAUSE TestRegistry_ParallelReads/parallel_read_273
=== RUN   TestRegistry_ParallelReads/parallel_read_274
=== PAUSE TestRegistry_ParallelReads/parallel_read_274
=== RUN   TestRegistry_ParallelReads/parallel_read_275
=== PAUSE TestRegistry_ParallelReads/parallel_read_275
=== RUN   TestRegistry_ParallelReads/parallel_read_276
=== PAUSE TestRegistry_ParallelReads/parallel_read_276
=== RUN   TestRegistry_ParallelReads/parallel_read_277
=== PAUSE TestRegistry_ParallelReads/parallel_read_277
=== RUN   TestRegistry_ParallelReads/parallel_read_278
=== PAUSE TestRegistry_ParallelReads/parallel_read_278
=== RUN   TestRegistry_ParallelReads/parallel_read_279
=== PAUSE TestRegistry_ParallelReads/parallel_read_279
=== RUN   TestRegistry_ParallelReads/parallel_read_280
=== PAUSE TestRegistry_ParallelReads/parallel_read_280
=== RUN   TestRegistry_ParallelReads/parallel_read_281
=== PAUSE TestRegistry_ParallelReads/parallel_read_281
=== RUN   TestRegistry_ParallelReads/parallel_read_282
=== PAUSE TestRegistry_ParallelReads/parallel_read_282
=== RUN   TestRegistry_ParallelReads/parallel_read_283
=== PAUSE TestRegistry_ParallelReads/parallel_read_283
=== RUN   TestRegistry_ParallelReads/parallel_read_284
=== PAUSE TestRegistry_ParallelReads/parallel_read_284
=== RUN   TestRegistry_ParallelReads/parallel_read_285
=== PAUSE TestRegistry_ParallelReads/parallel_read_285
=== RUN   TestRegistry_ParallelReads/parallel_read_286
=== PAUSE TestRegistry_ParallelReads/parallel_read_286
=== RUN   TestRegistry_ParallelReads/parallel_read_287
=== PAUSE TestRegistry_ParallelReads/parallel_read_287
=== RUN   TestRegistry_ParallelReads/parallel_read_288
=== PAUSE TestRegistry_ParallelReads/parallel_read_288
=== RUN   TestRegistry_ParallelReads/parallel_read_289
=== PAUSE TestRegistry_ParallelReads/parallel_read_289
=== RUN   TestRegistry_ParallelReads/parallel_read_290
=== PAUSE TestRegistry_ParallelReads/parallel_read_290
=== RUN   TestRegistry_ParallelReads/parallel_read_291
=== PAUSE TestRegistry_ParallelReads/parallel_read_291
=== RUN   TestRegistry_ParallelReads/parallel_read_292
=== PAUSE TestRegistry_ParallelReads/parallel_read_292
=== RUN   TestRegistry_ParallelReads/parallel_read_293
=== PAUSE TestRegistry_ParallelReads/parallel_read_293
=== RUN   TestRegistry_ParallelReads/parallel_read_294
=== PAUSE TestRegistry_ParallelReads/parallel_read_294
=== RUN   TestRegistry_ParallelReads/parallel_read_295
=== PAUSE TestRegistry_ParallelReads/parallel_read_295
=== RUN   TestRegistry_ParallelReads/parallel_read_296
=== PAUSE TestRegistry_ParallelReads/parallel_read_296
=== RUN   TestRegistry_ParallelReads/parallel_read_297
=== PAUSE TestRegistry_ParallelReads/parallel_read_297
=== RUN   TestRegistry_ParallelReads/parallel_read_298
=== PAUSE TestRegistry_ParallelReads/parallel_read_298
=== RUN   TestRegistry_ParallelReads/parallel_read_299
=== PAUSE TestRegistry_ParallelReads/parallel_read_299
=== RUN   TestRegistry_ParallelReads/parallel_read_300
=== PAUSE TestRegistry_ParallelReads/parallel_read_300
=== RUN   TestRegistry_ParallelReads/parallel_read_301
=== PAUSE TestRegistry_ParallelReads/parallel_read_301
=== RUN   TestRegistry_ParallelReads/parallel_read_302
=== PAUSE TestRegistry_ParallelReads/parallel_read_302
=== RUN   TestRegistry_ParallelReads/parallel_read_303
=== PAUSE TestRegistry_ParallelReads/parallel_read_303
=== RUN   TestRegistry_ParallelReads/parallel_read_304
=== PAUSE TestRegistry_ParallelReads/parallel_read_304
=== RUN   TestRegistry_ParallelReads/parallel_read_305
=== PAUSE TestRegistry_ParallelReads/parallel_read_305
=== RUN   TestRegistry_ParallelReads/parallel_read_306
=== PAUSE TestRegistry_ParallelReads/parallel_read_306
=== RUN   TestRegistry_ParallelReads/parallel_read_307
=== PAUSE TestRegistry_ParallelReads/parallel_read_307
=== RUN   TestRegistry_ParallelReads/parallel_read_308
=== PAUSE TestRegistry_ParallelReads/parallel_read_308
=== RUN   TestRegistry_ParallelReads/parallel_read_309
=== PAUSE TestRegistry_ParallelReads/parallel_read_309
=== RUN   TestRegistry_ParallelReads/parallel_read_310
=== PAUSE TestRegistry_ParallelReads/parallel_read_310
=== RUN   TestRegistry_ParallelReads/parallel_read_311
=== PAUSE TestRegistry_ParallelReads/parallel_read_311
=== RUN   TestRegistry_ParallelReads/parallel_read_312
=== PAUSE TestRegistry_ParallelReads/parallel_read_312
=== RUN   TestRegistry_ParallelReads/parallel_read_313
=== PAUSE TestRegistry_ParallelReads/parallel_read_313
=== RUN   TestRegistry_ParallelReads/parallel_read_314
=== PAUSE TestRegistry_ParallelReads/parallel_read_314
=== RUN   TestRegistry_ParallelReads/parallel_read_315
=== PAUSE TestRegistry_ParallelReads/parallel_read_315
=== RUN   TestRegistry_ParallelReads/parallel_read_316
=== PAUSE TestRegistry_ParallelReads/parallel_read_316
=== RUN   TestRegistry_ParallelReads/parallel_read_317
=== PAUSE TestRegistry_ParallelReads/parallel_read_317
=== RUN   TestRegistry_ParallelReads/parallel_read_318
=== PAUSE TestRegistry_ParallelReads/parallel_read_318
=== RUN   TestRegistry_ParallelReads/parallel_read_319
=== PAUSE TestRegistry_ParallelReads/parallel_read_319
=== RUN   TestRegistry_ParallelReads/parallel_read_320
=== PAUSE TestRegistry_ParallelReads/parallel_read_320
=== RUN   TestRegistry_ParallelReads/parallel_read_321
=== PAUSE TestRegistry_ParallelReads/parallel_read_321
=== RUN   TestRegistry_ParallelReads/parallel_read_322
=== PAUSE TestRegistry_ParallelReads/parallel_read_322
=== RUN   TestRegistry_ParallelReads/parallel_read_323
=== PAUSE TestRegistry_ParallelReads/parallel_read_323
=== RUN   TestRegistry_ParallelReads/parallel_read_324
=== PAUSE TestRegistry_ParallelReads/parallel_read_324
=== RUN   TestRegistry_ParallelReads/parallel_read_325
=== PAUSE TestRegistry_ParallelReads/parallel_read_325
=== RUN   TestRegistry_ParallelReads/parallel_read_326
=== PAUSE TestRegistry_ParallelReads/parallel_read_326
=== RUN   TestRegistry_ParallelReads/parallel_read_327
=== PAUSE TestRegistry_ParallelReads/parallel_read_327
=== RUN   TestRegistry_ParallelReads/parallel_read_328
=== PAUSE TestRegistry_ParallelReads/parallel_read_328
=== RUN   TestRegistry_ParallelReads/parallel_read_329
=== PAUSE TestRegistry_ParallelReads/parallel_read_329
=== RUN   TestRegistry_ParallelReads/parallel_read_330
=== PAUSE TestRegistry_ParallelReads/parallel_read_330
=== RUN   TestRegistry_ParallelReads/parallel_read_331
=== PAUSE TestRegistry_ParallelReads/parallel_read_331
=== RUN   TestRegistry_ParallelReads/parallel_read_332
=== PAUSE TestRegistry_ParallelReads/parallel_read_332
=== RUN   TestRegistry_ParallelReads/parallel_read_333
=== PAUSE TestRegistry_ParallelReads/parallel_read_333
=== RUN   TestRegistry_ParallelReads/parallel_read_334
=== PAUSE TestRegistry_ParallelReads/parallel_read_334
=== RUN   TestRegistry_ParallelReads/parallel_read_335
=== PAUSE TestRegistry_ParallelReads/parallel_read_335
=== RUN   TestRegistry_ParallelReads/parallel_read_336
=== PAUSE TestRegistry_ParallelReads/parallel_read_336
=== RUN   TestRegistry_ParallelReads/parallel_read_337
=== PAUSE TestRegistry_ParallelReads/parallel_read_337
=== RUN   TestRegistry_ParallelReads/parallel_read_338
=== PAUSE TestRegistry_ParallelReads/parallel_read_338
=== RUN   TestRegistry_ParallelReads/parallel_read_339
=== PAUSE TestRegistry_ParallelReads/parallel_read_339
=== RUN   TestRegistry_ParallelReads/parallel_read_340
=== PAUSE TestRegistry_ParallelReads/parallel_read_340
=== RUN   TestRegistry_ParallelReads/parallel_read_341
=== PAUSE TestRegistry_ParallelReads/parallel_read_341
=== RUN   TestRegistry_ParallelReads/parallel_read_342
=== PAUSE TestRegistry_ParallelReads/parallel_read_342
=== RUN   TestRegistry_ParallelReads/parallel_read_343
=== PAUSE TestRegistry_ParallelReads/parallel_read_343
=== RUN   TestRegistry_ParallelReads/parallel_read_344
=== PAUSE TestRegistry_ParallelReads/parallel_read_344
=== RUN   TestRegistry_ParallelReads/parallel_read_345
=== PAUSE TestRegistry_ParallelReads/parallel_read_345
=== RUN   TestRegistry_ParallelReads/parallel_read_346
=== PAUSE TestRegistry_ParallelReads/parallel_read_346
=== RUN   TestRegistry_ParallelReads/parallel_read_347
=== PAUSE TestRegistry_ParallelReads/parallel_read_347
=== RUN   TestRegistry_ParallelReads/parallel_read_348
=== PAUSE TestRegistry_ParallelReads/parallel_read_348
=== RUN   TestRegistry_ParallelReads/parallel_read_349
=== PAUSE TestRegistry_ParallelReads/parallel_read_349
=== RUN   TestRegistry_ParallelReads/parallel_read_350
=== PAUSE TestRegistry_ParallelReads/parallel_read_350
=== RUN   TestRegistry_ParallelReads/parallel_read_351
=== PAUSE TestRegistry_ParallelReads/parallel_read_351
=== RUN   TestRegistry_ParallelReads/parallel_read_352
=== PAUSE TestRegistry_ParallelReads/parallel_read_352
=== RUN   TestRegistry_ParallelReads/parallel_read_353
=== PAUSE TestRegistry_ParallelReads/parallel_read_353
=== RUN   TestRegistry_ParallelReads/parallel_read_354
=== PAUSE TestRegistry_ParallelReads/parallel_read_354
=== RUN   TestRegistry_ParallelReads/parallel_read_355
=== PAUSE TestRegistry_ParallelReads/parallel_read_355
=== RUN   TestRegistry_ParallelReads/parallel_read_356
=== PAUSE TestRegistry_ParallelReads/parallel_read_356
=== RUN   TestRegistry_ParallelReads/parallel_read_357
=== PAUSE TestRegistry_ParallelReads/parallel_read_357
=== RUN   TestRegistry_ParallelReads/parallel_read_358
=== PAUSE TestRegistry_ParallelReads/parallel_read_358
=== RUN   TestRegistry_ParallelReads/parallel_read_359
=== PAUSE TestRegistry_ParallelReads/parallel_read_359
=== RUN   TestRegistry_ParallelReads/parallel_read_360
=== PAUSE TestRegistry_ParallelReads/parallel_read_360
=== RUN   TestRegistry_ParallelReads/parallel_read_361
=== PAUSE TestRegistry_ParallelReads/parallel_read_361
=== RUN   TestRegistry_ParallelReads/parallel_read_362
=== PAUSE TestRegistry_ParallelReads/parallel_read_362
=== RUN   TestRegistry_ParallelReads/parallel_read_363
=== PAUSE TestRegistry_ParallelReads/parallel_read_363
=== RUN   TestRegistry_ParallelReads/parallel_read_364
=== PAUSE TestRegistry_ParallelReads/parallel_read_364
=== RUN   TestRegistry_ParallelReads/parallel_read_365
=== PAUSE TestRegistry_ParallelReads/parallel_read_365
=== RUN   TestRegistry_ParallelReads/parallel_read_366
=== PAUSE TestRegistry_ParallelReads/parallel_read_366
=== RUN   TestRegistry_ParallelReads/parallel_read_367
=== PAUSE TestRegistry_ParallelReads/parallel_read_367
=== RUN   TestRegistry_ParallelReads/parallel_read_368
=== PAUSE TestRegistry_ParallelReads/parallel_read_368
=== RUN   TestRegistry_ParallelReads/parallel_read_369
=== PAUSE TestRegistry_ParallelReads/parallel_read_369
=== RUN   TestRegistry_ParallelReads/parallel_read_370
=== PAUSE TestRegistry_ParallelReads/parallel_read_370
=== RUN   TestRegistry_ParallelReads/parallel_read_371
=== PAUSE TestRegistry_ParallelReads/parallel_read_371
=== RUN   TestRegistry_ParallelReads/parallel_read_372
=== PAUSE TestRegistry_ParallelReads/parallel_read_372
=== RUN   TestRegistry_ParallelReads/parallel_read_373
=== PAUSE TestRegistry_ParallelReads/parallel_read_373
=== RUN   TestRegistry_ParallelReads/parallel_read_374
=== PAUSE TestRegistry_ParallelReads/parallel_read_374
=== RUN   TestRegistry_ParallelReads/parallel_read_375
=== PAUSE TestRegistry_ParallelReads/parallel_read_375
=== RUN   TestRegistry_ParallelReads/parallel_read_376
=== PAUSE TestRegistry_ParallelReads/parallel_read_376
=== RUN   TestRegistry_ParallelReads/parallel_read_377
=== PAUSE TestRegistry_ParallelReads/parallel_read_377
=== RUN   TestRegistry_ParallelReads/parallel_read_378
=== PAUSE TestRegistry_ParallelReads/parallel_read_378
=== RUN   TestRegistry_ParallelReads/parallel_read_379
=== PAUSE TestRegistry_ParallelReads/parallel_read_379
=== RUN   TestRegistry_ParallelReads/parallel_read_380
=== PAUSE TestRegistry_ParallelReads/parallel_read_380
=== RUN   TestRegistry_ParallelReads/parallel_read_381
=== PAUSE TestRegistry_ParallelReads/parallel_read_381
=== RUN   TestRegistry_ParallelReads/parallel_read_382
=== PAUSE TestRegistry_ParallelReads/parallel_read_382
=== RUN   TestRegistry_ParallelReads/parallel_read_383
=== PAUSE TestRegistry_ParallelReads/parallel_read_383
=== RUN   TestRegistry_ParallelReads/parallel_read_384
=== PAUSE TestRegistry_ParallelReads/parallel_read_384
=== RUN   TestRegistry_ParallelReads/parallel_read_385
=== PAUSE TestRegistry_ParallelReads/parallel_read_385
=== RUN   TestRegistry_ParallelReads/parallel_read_386
=== PAUSE TestRegistry_ParallelReads/parallel_read_386
=== RUN   TestRegistry_ParallelReads/parallel_read_387
=== PAUSE TestRegistry_ParallelReads/parallel_read_387
=== RUN   TestRegistry_ParallelReads/parallel_read_388
=== PAUSE TestRegistry_ParallelReads/parallel_read_388
=== RUN   TestRegistry_ParallelReads/parallel_read_389
=== PAUSE TestRegistry_ParallelReads/parallel_read_389
=== RUN   TestRegistry_ParallelReads/parallel_read_390
=== PAUSE TestRegistry_ParallelReads/parallel_read_390
=== RUN   TestRegistry_ParallelReads/parallel_read_391
=== PAUSE TestRegistry_ParallelReads/parallel_read_391
=== RUN   TestRegistry_ParallelReads/parallel_read_392
=== PAUSE TestRegistry_ParallelReads/parallel_read_392
=== RUN   TestRegistry_ParallelReads/parallel_read_393
=== PAUSE TestRegistry_ParallelReads/parallel_read_393
=== RUN   TestRegistry_ParallelReads/parallel_read_394
=== PAUSE TestRegistry_ParallelReads/parallel_read_394
=== RUN   TestRegistry_ParallelReads/parallel_read_395
=== PAUSE TestRegistry_ParallelReads/parallel_read_395
=== RUN   TestRegistry_ParallelReads/parallel_read_396
=== PAUSE TestRegistry_ParallelReads/parallel_read_396
=== RUN   TestRegistry_ParallelReads/parallel_read_397
=== PAUSE TestRegistry_ParallelReads/parallel_read_397
=== RUN   TestRegistry_ParallelReads/parallel_read_398
=== PAUSE TestRegistry_ParallelReads/parallel_read_398
=== RUN   TestRegistry_ParallelReads/parallel_read_399
=== PAUSE TestRegistry_ParallelReads/parallel_read_399
=== RUN   TestRegistry_ParallelReads/parallel_read_400
=== PAUSE TestRegistry_ParallelReads/parallel_read_400
=== RUN   TestRegistry_ParallelReads/parallel_read_401
=== PAUSE TestRegistry_ParallelReads/parallel_read_401
=== RUN   TestRegistry_ParallelReads/parallel_read_402
=== PAUSE TestRegistry_ParallelReads/parallel_read_402
=== RUN   TestRegistry_ParallelReads/parallel_read_403
=== PAUSE TestRegistry_ParallelReads/parallel_read_403
=== RUN   TestRegistry_ParallelReads/parallel_read_404
=== PAUSE TestRegistry_ParallelReads/parallel_read_404
=== RUN   TestRegistry_ParallelReads/parallel_read_405
=== PAUSE TestRegistry_ParallelReads/parallel_read_405
=== RUN   TestRegistry_ParallelReads/parallel_read_406
=== PAUSE TestRegistry_ParallelReads/parallel_read_406
=== RUN   TestRegistry_ParallelReads/parallel_read_407
=== PAUSE TestRegistry_ParallelReads/parallel_read_407
=== RUN   TestRegistry_ParallelReads/parallel_read_408
=== PAUSE TestRegistry_ParallelReads/parallel_read_408
=== RUN   TestRegistry_ParallelReads/parallel_read_409
=== PAUSE TestRegistry_ParallelReads/parallel_read_409
=== RUN   TestRegistry_ParallelReads/parallel_read_410
=== PAUSE TestRegistry_ParallelReads/parallel_read_410
=== RUN   TestRegistry_ParallelReads/parallel_read_411
=== PAUSE TestRegistry_ParallelReads/parallel_read_411
=== RUN   TestRegistry_ParallelReads/parallel_read_412
=== PAUSE TestRegistry_ParallelReads/parallel_read_412
=== RUN   TestRegistry_ParallelReads/parallel_read_413
=== PAUSE TestRegistry_ParallelReads/parallel_read_413
=== RUN   TestRegistry_ParallelReads/parallel_read_414
=== PAUSE TestRegistry_ParallelReads/parallel_read_414
=== RUN   TestRegistry_ParallelReads/parallel_read_415
=== PAUSE TestRegistry_ParallelReads/parallel_read_415
=== RUN   TestRegistry_ParallelReads/parallel_read_416
=== PAUSE TestRegistry_ParallelReads/parallel_read_416
=== RUN   TestRegistry_ParallelReads/parallel_read_417
=== PAUSE TestRegistry_ParallelReads/parallel_read_417
=== RUN   TestRegistry_ParallelReads/parallel_read_418
=== PAUSE TestRegistry_ParallelReads/parallel_read_418
=== RUN   TestRegistry_ParallelReads/parallel_read_419
=== PAUSE TestRegistry_ParallelReads/parallel_read_419
=== RUN   TestRegistry_ParallelReads/parallel_read_420
=== PAUSE TestRegistry_ParallelReads/parallel_read_420
=== RUN   TestRegistry_ParallelReads/parallel_read_421
=== PAUSE TestRegistry_ParallelReads/parallel_read_421
=== RUN   TestRegistry_ParallelReads/parallel_read_422
=== PAUSE TestRegistry_ParallelReads/parallel_read_422
=== RUN   TestRegistry_ParallelReads/parallel_read_423
=== PAUSE TestRegistry_ParallelReads/parallel_read_423
=== RUN   TestRegistry_ParallelReads/parallel_read_424
=== PAUSE TestRegistry_ParallelReads/parallel_read_424
=== RUN   TestRegistry_ParallelReads/parallel_read_425
=== PAUSE TestRegistry_ParallelReads/parallel_read_425
=== RUN   TestRegistry_ParallelReads/parallel_read_426
=== PAUSE TestRegistry_ParallelReads/parallel_read_426
=== RUN   TestRegistry_ParallelReads/parallel_read_427
=== PAUSE TestRegistry_ParallelReads/parallel_read_427
=== RUN   TestRegistry_ParallelReads/parallel_read_428
=== PAUSE TestRegistry_ParallelReads/parallel_read_428
=== RUN   TestRegistry_ParallelReads/parallel_read_429
=== PAUSE TestRegistry_ParallelReads/parallel_read_429
=== RUN   TestRegistry_ParallelReads/parallel_read_430
=== PAUSE TestRegistry_ParallelReads/parallel_read_430
=== RUN   TestRegistry_ParallelReads/parallel_read_431
=== PAUSE TestRegistry_ParallelReads/parallel_read_431
=== RUN   TestRegistry_ParallelReads/parallel_read_432
=== PAUSE TestRegistry_ParallelReads/parallel_read_432
=== RUN   TestRegistry_ParallelReads/parallel_read_433
=== PAUSE TestRegistry_ParallelReads/parallel_read_433
=== RUN   TestRegistry_ParallelReads/parallel_read_434
=== PAUSE TestRegistry_ParallelReads/parallel_read_434
=== RUN   TestRegistry_ParallelReads/parallel_read_435
=== PAUSE TestRegistry_ParallelReads/parallel_read_435
=== RUN   TestRegistry_ParallelReads/parallel_read_436
=== PAUSE TestRegistry_ParallelReads/parallel_read_436
=== RUN   TestRegistry_ParallelReads/parallel_read_437
=== PAUSE TestRegistry_ParallelReads/parallel_read_437
=== RUN   TestRegistry_ParallelReads/parallel_read_438
=== PAUSE TestRegistry_ParallelReads/parallel_read_438
=== RUN   TestRegistry_ParallelReads/parallel_read_439
=== PAUSE TestRegistry_ParallelReads/parallel_read_439
=== RUN   TestRegistry_ParallelReads/parallel_read_440
=== PAUSE TestRegistry_ParallelReads/parallel_read_440
=== RUN   TestRegistry_ParallelReads/parallel_read_441
=== PAUSE TestRegistry_ParallelReads/parallel_read_441
=== RUN   TestRegistry_ParallelReads/parallel_read_442
=== PAUSE TestRegistry_ParallelReads/parallel_read_442
=== RUN   TestRegistry_ParallelReads/parallel_read_443
=== PAUSE TestRegistry_ParallelReads/parallel_read_443
=== RUN   TestRegistry_ParallelReads/parallel_read_444
=== PAUSE TestRegistry_ParallelReads/parallel_read_444
=== RUN   TestRegistry_ParallelReads/parallel_read_445
=== PAUSE TestRegistry_ParallelReads/parallel_read_445
=== RUN   TestRegistry_ParallelReads/parallel_read_446
=== PAUSE TestRegistry_ParallelReads/parallel_read_446
=== RUN   TestRegistry_ParallelReads/parallel_read_447
=== PAUSE TestRegistry_ParallelReads/parallel_read_447
=== RUN   TestRegistry_ParallelReads/parallel_read_448
=== PAUSE TestRegistry_ParallelReads/parallel_read_448
=== RUN   TestRegistry_ParallelReads/parallel_read_449
=== PAUSE TestRegistry_ParallelReads/parallel_read_449
=== RUN   TestRegistry_ParallelReads/parallel_read_450
=== PAUSE TestRegistry_ParallelReads/parallel_read_450
=== RUN   TestRegistry_ParallelReads/parallel_read_451
=== PAUSE TestRegistry_ParallelReads/parallel_read_451
=== RUN   TestRegistry_ParallelReads/parallel_read_452
=== PAUSE TestRegistry_ParallelReads/parallel_read_452
=== RUN   TestRegistry_ParallelReads/parallel_read_453
=== PAUSE TestRegistry_ParallelReads/parallel_read_453
=== RUN   TestRegistry_ParallelReads/parallel_read_454
=== PAUSE TestRegistry_ParallelReads/parallel_read_454
=== RUN   TestRegistry_ParallelReads/parallel_read_455
=== PAUSE TestRegistry_ParallelReads/parallel_read_455
=== RUN   TestRegistry_ParallelReads/parallel_read_456
=== PAUSE TestRegistry_ParallelReads/parallel_read_456
=== RUN   TestRegistry_ParallelReads/parallel_read_457
=== PAUSE TestRegistry_ParallelReads/parallel_read_457
=== RUN   TestRegistry_ParallelReads/parallel_read_458
=== PAUSE TestRegistry_ParallelReads/parallel_read_458
=== RUN   TestRegistry_ParallelReads/parallel_read_459
=== PAUSE TestRegistry_ParallelReads/parallel_read_459
=== RUN   TestRegistry_ParallelReads/parallel_read_460
=== PAUSE TestRegistry_ParallelReads/parallel_read_460
=== RUN   TestRegistry_ParallelReads/parallel_read_461
=== PAUSE TestRegistry_ParallelReads/parallel_read_461
=== RUN   TestRegistry_ParallelReads/parallel_read_462
=== PAUSE TestRegistry_ParallelReads/parallel_read_462
=== RUN   TestRegistry_ParallelReads/parallel_read_463
=== PAUSE TestRegistry_ParallelReads/parallel_read_463
=== RUN   TestRegistry_ParallelReads/parallel_read_464
=== PAUSE TestRegistry_ParallelReads/parallel_read_464
=== RUN   TestRegistry_ParallelReads/parallel_read_465
=== PAUSE TestRegistry_ParallelReads/parallel_read_465
=== RUN   TestRegistry_ParallelReads/parallel_read_466
=== PAUSE TestRegistry_ParallelReads/parallel_read_466
=== RUN   TestRegistry_ParallelReads/parallel_read_467
=== PAUSE TestRegistry_ParallelReads/parallel_read_467
=== RUN   TestRegistry_ParallelReads/parallel_read_468
=== PAUSE TestRegistry_ParallelReads/parallel_read_468
=== RUN   TestRegistry_ParallelReads/parallel_read_469
=== PAUSE TestRegistry_ParallelReads/parallel_read_469
=== RUN   TestRegistry_ParallelReads/parallel_read_470
=== PAUSE TestRegistry_ParallelReads/parallel_read_470
=== RUN   TestRegistry_ParallelReads/parallel_read_471
=== PAUSE TestRegistry_ParallelReads/parallel_read_471
=== RUN   TestRegistry_ParallelReads/parallel_read_472
=== PAUSE TestRegistry_ParallelReads/parallel_read_472
=== RUN   TestRegistry_ParallelReads/parallel_read_473
=== PAUSE TestRegistry_ParallelReads/parallel_read_473
=== RUN   TestRegistry_ParallelReads/parallel_read_474
=== PAUSE TestRegistry_ParallelReads/parallel_read_474
=== RUN   TestRegistry_ParallelReads/parallel_read_475
=== PAUSE TestRegistry_ParallelReads/parallel_read_475
=== RUN   TestRegistry_ParallelReads/parallel_read_476
=== PAUSE TestRegistry_ParallelReads/parallel_read_476
=== RUN   TestRegistry_ParallelReads/parallel_read_477
=== PAUSE TestRegistry_ParallelReads/parallel_read_477
=== RUN   TestRegistry_ParallelReads/parallel_read_478
=== PAUSE TestRegistry_ParallelReads/parallel_read_478
=== RUN   TestRegistry_ParallelReads/parallel_read_479
=== PAUSE TestRegistry_ParallelReads/parallel_read_479
=== RUN   TestRegistry_ParallelReads/parallel_read_480
=== PAUSE TestRegistry_ParallelReads/parallel_read_480
=== RUN   TestRegistry_ParallelReads/parallel_read_481
=== PAUSE TestRegistry_ParallelReads/parallel_read_481
=== RUN   TestRegistry_ParallelReads/parallel_read_482
=== PAUSE TestRegistry_ParallelReads/parallel_read_482
=== RUN   TestRegistry_ParallelReads/parallel_read_483
=== PAUSE TestRegistry_ParallelReads/parallel_read_483
=== RUN   TestRegistry_ParallelReads/parallel_read_484
=== PAUSE TestRegistry_ParallelReads/parallel_read_484
=== RUN   TestRegistry_ParallelReads/parallel_read_485
=== PAUSE TestRegistry_ParallelReads/parallel_read_485
=== RUN   TestRegistry_ParallelReads/parallel_read_486
=== PAUSE TestRegistry_ParallelReads/parallel_read_486
=== RUN   TestRegistry_ParallelReads/parallel_read_487
=== PAUSE TestRegistry_ParallelReads/parallel_read_487
=== RUN   TestRegistry_ParallelReads/parallel_read_488
=== PAUSE TestRegistry_ParallelReads/parallel_read_488
=== RUN   TestRegistry_ParallelReads/parallel_read_489
=== PAUSE TestRegistry_ParallelReads/parallel_read_489
=== RUN   TestRegistry_ParallelReads/parallel_read_490
=== PAUSE TestRegistry_ParallelReads/parallel_read_490
=== RUN   TestRegistry_ParallelReads/parallel_read_491
=== PAUSE TestRegistry_ParallelReads/parallel_read_491
=== RUN   TestRegistry_ParallelReads/parallel_read_492
=== PAUSE TestRegistry_ParallelReads/parallel_read_492
=== RUN   TestRegistry_ParallelReads/parallel_read_493
=== PAUSE TestRegistry_ParallelReads/parallel_read_493
=== RUN   TestRegistry_ParallelReads/parallel_read_494
=== PAUSE TestRegistry_ParallelReads/parallel_read_494
=== RUN   TestRegistry_ParallelReads/parallel_read_495
=== PAUSE TestRegistry_ParallelReads/parallel_read_495
=== RUN   TestRegistry_ParallelReads/parallel_read_496
=== PAUSE TestRegistry_ParallelReads/parallel_read_496
=== RUN   TestRegistry_ParallelReads/parallel_read_497
=== PAUSE TestRegistry_ParallelReads/parallel_read_497
=== RUN   TestRegistry_ParallelReads/parallel_read_498
=== PAUSE TestRegistry_ParallelReads/parallel_read_498
=== RUN   TestRegistry_ParallelReads/parallel_read_499
=== PAUSE TestRegistry_ParallelReads/parallel_read_499
=== RUN   TestRegistry_ParallelReads/parallel_read_500
=== PAUSE TestRegistry_ParallelReads/parallel_read_500
=== RUN   TestRegistry_ParallelReads/parallel_read_501
=== PAUSE TestRegistry_ParallelReads/parallel_read_501
=== RUN   TestRegistry_ParallelReads/parallel_read_502
=== PAUSE TestRegistry_ParallelReads/parallel_read_502
=== RUN   TestRegistry_ParallelReads/parallel_read_503
=== PAUSE TestRegistry_ParallelReads/parallel_read_503
=== RUN   TestRegistry_ParallelReads/parallel_read_504
=== PAUSE TestRegistry_ParallelReads/parallel_read_504
=== RUN   TestRegistry_ParallelReads/parallel_read_505
=== PAUSE TestRegistry_ParallelReads/parallel_read_505
=== RUN   TestRegistry_ParallelReads/parallel_read_506
=== PAUSE TestRegistry_ParallelReads/parallel_read_506
=== RUN   TestRegistry_ParallelReads/parallel_read_507
=== PAUSE TestRegistry_ParallelReads/parallel_read_507
=== RUN   TestRegistry_ParallelReads/parallel_read_508
=== PAUSE TestRegistry_ParallelReads/parallel_read_508
=== RUN   TestRegistry_ParallelReads/parallel_read_509
=== PAUSE TestRegistry_ParallelReads/parallel_read_509
=== RUN   TestRegistry_ParallelReads/parallel_read_510
=== PAUSE TestRegistry_ParallelReads/parallel_read_510
=== RUN   TestRegistry_ParallelReads/parallel_read_511
=== PAUSE TestRegistry_ParallelReads/parallel_read_511
=== RUN   TestRegistry_ParallelReads/parallel_read_512
=== PAUSE TestRegistry_ParallelReads/parallel_read_512
=== RUN   TestRegistry_ParallelReads/parallel_read_513
=== PAUSE TestRegistry_ParallelReads/parallel_read_513
=== RUN   TestRegistry_ParallelReads/parallel_read_514
=== PAUSE TestRegistry_ParallelReads/parallel_read_514
=== RUN   TestRegistry_ParallelReads/parallel_read_515
=== PAUSE TestRegistry_ParallelReads/parallel_read_515
=== RUN   TestRegistry_ParallelReads/parallel_read_516
=== PAUSE TestRegistry_ParallelReads/parallel_read_516
=== RUN   TestRegistry_ParallelReads/parallel_read_517
=== PAUSE TestRegistry_ParallelReads/parallel_read_517
=== RUN   TestRegistry_ParallelReads/parallel_read_518
=== PAUSE TestRegistry_ParallelReads/parallel_read_518
=== RUN   TestRegistry_ParallelReads/parallel_read_519
=== PAUSE TestRegistry_ParallelReads/parallel_read_519
=== RUN   TestRegistry_ParallelReads/parallel_read_520
=== PAUSE TestRegistry_ParallelReads/parallel_read_520
=== RUN   TestRegistry_ParallelReads/parallel_read_521
=== PAUSE TestRegistry_ParallelReads/parallel_read_521
=== RUN   TestRegistry_ParallelReads/parallel_read_522
=== PAUSE TestRegistry_ParallelReads/parallel_read_522
=== RUN   TestRegistry_ParallelReads/parallel_read_523
=== PAUSE TestRegistry_ParallelReads/parallel_read_523
=== RUN   TestRegistry_ParallelReads/parallel_read_524
=== PAUSE TestRegistry_ParallelReads/parallel_read_524
=== RUN   TestRegistry_ParallelReads/parallel_read_525
=== PAUSE TestRegistry_ParallelReads/parallel_read_525
=== RUN   TestRegistry_ParallelReads/parallel_read_526
=== PAUSE TestRegistry_ParallelReads/parallel_read_526
=== RUN   TestRegistry_ParallelReads/parallel_read_527
=== PAUSE TestRegistry_ParallelReads/parallel_read_527
=== RUN   TestRegistry_ParallelReads/parallel_read_528
=== PAUSE TestRegistry_ParallelReads/parallel_read_528
=== RUN   TestRegistry_ParallelReads/parallel_read_529
=== PAUSE TestRegistry_ParallelReads/parallel_read_529
=== RUN   TestRegistry_ParallelReads/parallel_read_530
=== PAUSE TestRegistry_ParallelReads/parallel_read_530
=== RUN   TestRegistry_ParallelReads/parallel_read_531
=== PAUSE TestRegistry_ParallelReads/parallel_read_531
=== RUN   TestRegistry_ParallelReads/parallel_read_532
=== PAUSE TestRegistry_ParallelReads/parallel_read_532
=== RUN   TestRegistry_ParallelReads/parallel_read_533
=== PAUSE TestRegistry_ParallelReads/parallel_read_533
=== RUN   TestRegistry_ParallelReads/parallel_read_534
=== PAUSE TestRegistry_ParallelReads/parallel_read_534
=== RUN   TestRegistry_ParallelReads/parallel_read_535
=== PAUSE TestRegistry_ParallelReads/parallel_read_535
=== RUN   TestRegistry_ParallelReads/parallel_read_536
=== PAUSE TestRegistry_ParallelReads/parallel_read_536
=== RUN   TestRegistry_ParallelReads/parallel_read_537
=== PAUSE TestRegistry_ParallelReads/parallel_read_537
=== RUN   TestRegistry_ParallelReads/parallel_read_538
=== PAUSE TestRegistry_ParallelReads/parallel_read_538
=== RUN   TestRegistry_ParallelReads/parallel_read_539
=== PAUSE TestRegistry_ParallelReads/parallel_read_539
=== RUN   TestRegistry_ParallelReads/parallel_read_540
=== PAUSE TestRegistry_ParallelReads/parallel_read_540
=== RUN   TestRegistry_ParallelReads/parallel_read_541
=== PAUSE TestRegistry_ParallelReads/parallel_read_541
=== RUN   TestRegistry_ParallelReads/parallel_read_542
=== PAUSE TestRegistry_ParallelReads/parallel_read_542
=== RUN   TestRegistry_ParallelReads/parallel_read_543
=== PAUSE TestRegistry_ParallelReads/parallel_read_543
=== RUN   TestRegistry_ParallelReads/parallel_read_544
=== PAUSE TestRegistry_ParallelReads/parallel_read_544
=== RUN   TestRegistry_ParallelReads/parallel_read_545
=== PAUSE TestRegistry_ParallelReads/parallel_read_545
=== RUN   TestRegistry_ParallelReads/parallel_read_546
=== PAUSE TestRegistry_ParallelReads/parallel_read_546
=== RUN   TestRegistry_ParallelReads/parallel_read_547
=== PAUSE TestRegistry_ParallelReads/parallel_read_547
=== RUN   TestRegistry_ParallelReads/parallel_read_548
=== PAUSE TestRegistry_ParallelReads/parallel_read_548
=== RUN   TestRegistry_ParallelReads/parallel_read_549
=== PAUSE TestRegistry_ParallelReads/parallel_read_549
=== RUN   TestRegistry_ParallelReads/parallel_read_550
=== PAUSE TestRegistry_ParallelReads/parallel_read_550
=== RUN   TestRegistry_ParallelReads/parallel_read_551
=== PAUSE TestRegistry_ParallelReads/parallel_read_551
=== RUN   TestRegistry_ParallelReads/parallel_read_552
=== PAUSE TestRegistry_ParallelReads/parallel_read_552
=== RUN   TestRegistry_ParallelReads/parallel_read_553
=== PAUSE TestRegistry_ParallelReads/parallel_read_553
=== RUN   TestRegistry_ParallelReads/parallel_read_554
=== PAUSE TestRegistry_ParallelReads/parallel_read_554
=== RUN   TestRegistry_ParallelReads/parallel_read_555
=== PAUSE TestRegistry_ParallelReads/parallel_read_555
=== RUN   TestRegistry_ParallelReads/parallel_read_556
=== PAUSE TestRegistry_ParallelReads/parallel_read_556
=== RUN   TestRegistry_ParallelReads/parallel_read_557
=== PAUSE TestRegistry_ParallelReads/parallel_read_557
=== RUN   TestRegistry_ParallelReads/parallel_read_558
=== PAUSE TestRegistry_ParallelReads/parallel_read_558
=== RUN   TestRegistry_ParallelReads/parallel_read_559
=== PAUSE TestRegistry_ParallelReads/parallel_read_559
=== RUN   TestRegistry_ParallelReads/parallel_read_560
=== PAUSE TestRegistry_ParallelReads/parallel_read_560
=== RUN   TestRegistry_ParallelReads/parallel_read_561
=== PAUSE TestRegistry_ParallelReads/parallel_read_561
=== RUN   TestRegistry_ParallelReads/parallel_read_562
=== PAUSE TestRegistry_ParallelReads/parallel_read_562
=== RUN   TestRegistry_ParallelReads/parallel_read_563
=== PAUSE TestRegistry_ParallelReads/parallel_read_563
=== RUN   TestRegistry_ParallelReads/parallel_read_564
=== PAUSE TestRegistry_ParallelReads/parallel_read_564
=== RUN   TestRegistry_ParallelReads/parallel_read_565
=== PAUSE TestRegistry_ParallelReads/parallel_read_565
=== RUN   TestRegistry_ParallelReads/parallel_read_566
=== PAUSE TestRegistry_ParallelReads/parallel_read_566
=== RUN   TestRegistry_ParallelReads/parallel_read_567
=== PAUSE TestRegistry_ParallelReads/parallel_read_567
=== RUN   TestRegistry_ParallelReads/parallel_read_568
=== PAUSE TestRegistry_ParallelReads/parallel_read_568
=== RUN   TestRegistry_ParallelReads/parallel_read_569
=== PAUSE TestRegistry_ParallelReads/parallel_read_569
=== RUN   TestRegistry_ParallelReads/parallel_read_570
=== PAUSE TestRegistry_ParallelReads/parallel_read_570
=== RUN   TestRegistry_ParallelReads/parallel_read_571
=== PAUSE TestRegistry_ParallelReads/parallel_read_571
=== RUN   TestRegistry_ParallelReads/parallel_read_572
=== PAUSE TestRegistry_ParallelReads/parallel_read_572
=== RUN   TestRegistry_ParallelReads/parallel_read_573
=== PAUSE TestRegistry_ParallelReads/parallel_read_573
=== RUN   TestRegistry_ParallelReads/parallel_read_574
=== PAUSE TestRegistry_ParallelReads/parallel_read_574
=== RUN   TestRegistry_ParallelReads/parallel_read_575
=== PAUSE TestRegistry_ParallelReads/parallel_read_575
=== RUN   TestRegistry_ParallelReads/parallel_read_576
=== PAUSE TestRegistry_ParallelReads/parallel_read_576
=== RUN   TestRegistry_ParallelReads/parallel_read_577
=== PAUSE TestRegistry_ParallelReads/parallel_read_577
=== RUN   TestRegistry_ParallelReads/parallel_read_578
=== PAUSE TestRegistry_ParallelReads/parallel_read_578
=== RUN   TestRegistry_ParallelReads/parallel_read_579
=== PAUSE TestRegistry_ParallelReads/parallel_read_579
=== RUN   TestRegistry_ParallelReads/parallel_read_580
=== PAUSE TestRegistry_ParallelReads/parallel_read_580
=== RUN   TestRegistry_ParallelReads/parallel_read_581
=== PAUSE TestRegistry_ParallelReads/parallel_read_581
=== RUN   TestRegistry_ParallelReads/parallel_read_582
=== PAUSE TestRegistry_ParallelReads/parallel_read_582
=== RUN   TestRegistry_ParallelReads/parallel_read_583
=== PAUSE TestRegistry_ParallelReads/parallel_read_583
=== RUN   TestRegistry_ParallelReads/parallel_read_584
=== PAUSE TestRegistry_ParallelReads/parallel_read_584
=== RUN   TestRegistry_ParallelReads/parallel_read_585
=== PAUSE TestRegistry_ParallelReads/parallel_read_585
=== RUN   TestRegistry_ParallelReads/parallel_read_586
=== PAUSE TestRegistry_ParallelReads/parallel_read_586
=== RUN   TestRegistry_ParallelReads/parallel_read_587
=== PAUSE TestRegistry_ParallelReads/parallel_read_587
=== RUN   TestRegistry_ParallelReads/parallel_read_588
=== PAUSE TestRegistry_ParallelReads/parallel_read_588
=== RUN   TestRegistry_ParallelReads/parallel_read_589
=== PAUSE TestRegistry_ParallelReads/parallel_read_589
=== RUN   TestRegistry_ParallelReads/parallel_read_590
=== PAUSE TestRegistry_ParallelReads/parallel_read_590
=== RUN   TestRegistry_ParallelReads/parallel_read_591
=== PAUSE TestRegistry_ParallelReads/parallel_read_591
=== RUN   TestRegistry_ParallelReads/parallel_read_592
=== PAUSE TestRegistry_ParallelReads/parallel_read_592
=== RUN   TestRegistry_ParallelReads/parallel_read_593
=== PAUSE TestRegistry_ParallelReads/parallel_read_593
=== RUN   TestRegistry_ParallelReads/parallel_read_594
=== PAUSE TestRegistry_ParallelReads/parallel_read_594
=== RUN   TestRegistry_ParallelReads/parallel_read_595
=== PAUSE TestRegistry_ParallelReads/parallel_read_595
=== RUN   TestRegistry_ParallelReads/parallel_read_596
=== PAUSE TestRegistry_ParallelReads/parallel_read_596
=== RUN   TestRegistry_ParallelReads/parallel_read_597
=== PAUSE TestRegistry_ParallelReads/parallel_read_597
=== RUN   TestRegistry_ParallelReads/parallel_read_598
=== PAUSE TestRegistry_ParallelReads/parallel_read_598
=== RUN   TestRegistry_ParallelReads/parallel_read_599
=== PAUSE TestRegistry_ParallelReads/parallel_read_599
=== RUN   TestRegistry_ParallelReads/parallel_read_600
=== PAUSE TestRegistry_ParallelReads/parallel_read_600
=== RUN   TestRegistry_ParallelReads/parallel_read_601
=== PAUSE TestRegistry_ParallelReads/parallel_read_601
=== RUN   TestRegistry_ParallelReads/parallel_read_602
=== PAUSE TestRegistry_ParallelReads/parallel_read_602
=== RUN   TestRegistry_ParallelReads/parallel_read_603
=== PAUSE TestRegistry_ParallelReads/parallel_read_603
=== RUN   TestRegistry_ParallelReads/parallel_read_604
=== PAUSE TestRegistry_ParallelReads/parallel_read_604
=== RUN   TestRegistry_ParallelReads/parallel_read_605
=== PAUSE TestRegistry_ParallelReads/parallel_read_605
=== RUN   TestRegistry_ParallelReads/parallel_read_606
=== PAUSE TestRegistry_ParallelReads/parallel_read_606
=== RUN   TestRegistry_ParallelReads/parallel_read_607
=== PAUSE TestRegistry_ParallelReads/parallel_read_607
=== RUN   TestRegistry_ParallelReads/parallel_read_608
=== PAUSE TestRegistry_ParallelReads/parallel_read_608
=== RUN   TestRegistry_ParallelReads/parallel_read_609
=== PAUSE TestRegistry_ParallelReads/parallel_read_609
=== RUN   TestRegistry_ParallelReads/parallel_read_610
=== PAUSE TestRegistry_ParallelReads/parallel_read_610
=== RUN   TestRegistry_ParallelReads/parallel_read_611
=== PAUSE TestRegistry_ParallelReads/parallel_read_611
=== RUN   TestRegistry_ParallelReads/parallel_read_612
=== PAUSE TestRegistry_ParallelReads/parallel_read_612
=== RUN   TestRegistry_ParallelReads/parallel_read_613
=== PAUSE TestRegistry_ParallelReads/parallel_read_613
=== RUN   TestRegistry_ParallelReads/parallel_read_614
=== PAUSE TestRegistry_ParallelReads/parallel_read_614
=== RUN   TestRegistry_ParallelReads/parallel_read_615
=== PAUSE TestRegistry_ParallelReads/parallel_read_615
=== RUN   TestRegistry_ParallelReads/parallel_read_616
=== PAUSE TestRegistry_ParallelReads/parallel_read_616
=== RUN   TestRegistry_ParallelReads/parallel_read_617
=== PAUSE TestRegistry_ParallelReads/parallel_read_617
=== RUN   TestRegistry_ParallelReads/parallel_read_618
=== PAUSE TestRegistry_ParallelReads/parallel_read_618
=== RUN   TestRegistry_ParallelReads/parallel_read_619
=== PAUSE TestRegistry_ParallelReads/parallel_read_619
=== RUN   TestRegistry_ParallelReads/parallel_read_620
=== PAUSE TestRegistry_ParallelReads/parallel_read_620
=== RUN   TestRegistry_ParallelReads/parallel_read_621
=== PAUSE TestRegistry_ParallelReads/parallel_read_621
=== RUN   TestRegistry_ParallelReads/parallel_read_622
=== PAUSE TestRegistry_ParallelReads/parallel_read_622
=== RUN   TestRegistry_ParallelReads/parallel_read_623
=== PAUSE TestRegistry_ParallelReads/parallel_read_623
=== RUN   TestRegistry_ParallelReads/parallel_read_624
=== PAUSE TestRegistry_ParallelReads/parallel_read_624
=== RUN   TestRegistry_ParallelReads/parallel_read_625
=== PAUSE TestRegistry_ParallelReads/parallel_read_625
=== RUN   TestRegistry_ParallelReads/parallel_read_626
=== PAUSE TestRegistry_ParallelReads/parallel_read_626
=== RUN   TestRegistry_ParallelReads/parallel_read_627
=== PAUSE TestRegistry_ParallelReads/parallel_read_627
=== RUN   TestRegistry_ParallelReads/parallel_read_628
=== PAUSE TestRegistry_ParallelReads/parallel_read_628
=== RUN   TestRegistry_ParallelReads/parallel_read_629
=== PAUSE TestRegistry_ParallelReads/parallel_read_629
=== RUN   TestRegistry_ParallelReads/parallel_read_630
=== PAUSE TestRegistry_ParallelReads/parallel_read_630
=== RUN   TestRegistry_ParallelReads/parallel_read_631
=== PAUSE TestRegistry_ParallelReads/parallel_read_631
=== RUN   TestRegistry_ParallelReads/parallel_read_632
=== PAUSE TestRegistry_ParallelReads/parallel_read_632
=== RUN   TestRegistry_ParallelReads/parallel_read_633
=== PAUSE TestRegistry_ParallelReads/parallel_read_633
=== RUN   TestRegistry_ParallelReads/parallel_read_634
=== PAUSE TestRegistry_ParallelReads/parallel_read_634
=== RUN   TestRegistry_ParallelReads/parallel_read_635
=== PAUSE TestRegistry_ParallelReads/parallel_read_635
=== RUN   TestRegistry_ParallelReads/parallel_read_636
=== PAUSE TestRegistry_ParallelReads/parallel_read_636
=== RUN   TestRegistry_ParallelReads/parallel_read_637
=== PAUSE TestRegistry_ParallelReads/parallel_read_637
=== RUN   TestRegistry_ParallelReads/parallel_read_638
=== PAUSE TestRegistry_ParallelReads/parallel_read_638
=== RUN   TestRegistry_ParallelReads/parallel_read_639
=== PAUSE TestRegistry_ParallelReads/parallel_read_639
=== RUN   TestRegistry_ParallelReads/parallel_read_640
=== PAUSE TestRegistry_ParallelReads/parallel_read_640
=== RUN   TestRegistry_ParallelReads/parallel_read_641
=== PAUSE TestRegistry_ParallelReads/parallel_read_641
=== RUN   TestRegistry_ParallelReads/parallel_read_642
=== PAUSE TestRegistry_ParallelReads/parallel_read_642
=== RUN   TestRegistry_ParallelReads/parallel_read_643
=== PAUSE TestRegistry_ParallelReads/parallel_read_643
=== RUN   TestRegistry_ParallelReads/parallel_read_644
=== PAUSE TestRegistry_ParallelReads/parallel_read_644
=== RUN   TestRegistry_ParallelReads/parallel_read_645
=== PAUSE TestRegistry_ParallelReads/parallel_read_645
=== RUN   TestRegistry_ParallelReads/parallel_read_646
=== PAUSE TestRegistry_ParallelReads/parallel_read_646
=== RUN   TestRegistry_ParallelReads/parallel_read_647
=== PAUSE TestRegistry_ParallelReads/parallel_read_647
=== RUN   TestRegistry_ParallelReads/parallel_read_648
=== PAUSE TestRegistry_ParallelReads/parallel_read_648
=== RUN   TestRegistry_ParallelReads/parallel_read_649
=== PAUSE TestRegistry_ParallelReads/parallel_read_649
=== RUN   TestRegistry_ParallelReads/parallel_read_650
=== PAUSE TestRegistry_ParallelReads/parallel_read_650
=== RUN   TestRegistry_ParallelReads/parallel_read_651
=== PAUSE TestRegistry_ParallelReads/parallel_read_651
=== RUN   TestRegistry_ParallelReads/parallel_read_652
=== PAUSE TestRegistry_ParallelReads/parallel_read_652
=== RUN   TestRegistry_ParallelReads/parallel_read_653
=== PAUSE TestRegistry_ParallelReads/parallel_read_653
=== RUN   TestRegistry_ParallelReads/parallel_read_654
=== PAUSE TestRegistry_ParallelReads/parallel_read_654
=== RUN   TestRegistry_ParallelReads/parallel_read_655
=== PAUSE TestRegistry_ParallelReads/parallel_read_655
=== RUN   TestRegistry_ParallelReads/parallel_read_656
=== PAUSE TestRegistry_ParallelReads/parallel_read_656
=== RUN   TestRegistry_ParallelReads/parallel_read_657
=== PAUSE TestRegistry_ParallelReads/parallel_read_657
=== RUN   TestRegistry_ParallelReads/parallel_read_658
=== PAUSE TestRegistry_ParallelReads/parallel_read_658
=== RUN   TestRegistry_ParallelReads/parallel_read_659
=== PAUSE TestRegistry_ParallelReads/parallel_read_659
=== RUN   TestRegistry_ParallelReads/parallel_read_660
=== PAUSE TestRegistry_ParallelReads/parallel_read_660
=== RUN   TestRegistry_ParallelReads/parallel_read_661
=== PAUSE TestRegistry_ParallelReads/parallel_read_661
=== RUN   TestRegistry_ParallelReads/parallel_read_662
=== PAUSE TestRegistry_ParallelReads/parallel_read_662
=== RUN   TestRegistry_ParallelReads/parallel_read_663
=== PAUSE TestRegistry_ParallelReads/parallel_read_663
=== RUN   TestRegistry_ParallelReads/parallel_read_664
=== PAUSE TestRegistry_ParallelReads/parallel_read_664
=== RUN   TestRegistry_ParallelReads/parallel_read_665
=== PAUSE TestRegistry_ParallelReads/parallel_read_665
=== RUN   TestRegistry_ParallelReads/parallel_read_666
=== PAUSE TestRegistry_ParallelReads/parallel_read_666
=== RUN   TestRegistry_ParallelReads/parallel_read_667
=== PAUSE TestRegistry_ParallelReads/parallel_read_667
=== RUN   TestRegistry_ParallelReads/parallel_read_668
=== PAUSE TestRegistry_ParallelReads/parallel_read_668
=== RUN   TestRegistry_ParallelReads/parallel_read_669
=== PAUSE TestRegistry_ParallelReads/parallel_read_669
=== RUN   TestRegistry_ParallelReads/parallel_read_670
=== PAUSE TestRegistry_ParallelReads/parallel_read_670
=== RUN   TestRegistry_ParallelReads/parallel_read_671
=== PAUSE TestRegistry_ParallelReads/parallel_read_671
=== RUN   TestRegistry_ParallelReads/parallel_read_672
=== PAUSE TestRegistry_ParallelReads/parallel_read_672
=== RUN   TestRegistry_ParallelReads/parallel_read_673
=== PAUSE TestRegistry_ParallelReads/parallel_read_673
=== RUN   TestRegistry_ParallelReads/parallel_read_674
=== PAUSE TestRegistry_ParallelReads/parallel_read_674
=== RUN   TestRegistry_ParallelReads/parallel_read_675
=== PAUSE TestRegistry_ParallelReads/parallel_read_675
=== RUN   TestRegistry_ParallelReads/parallel_read_676
=== PAUSE TestRegistry_ParallelReads/parallel_read_676
=== RUN   TestRegistry_ParallelReads/parallel_read_677
=== PAUSE TestRegistry_ParallelReads/parallel_read_677
=== RUN   TestRegistry_ParallelReads/parallel_read_678
=== PAUSE TestRegistry_ParallelReads/parallel_read_678
=== RUN   TestRegistry_ParallelReads/parallel_read_679
=== PAUSE TestRegistry_ParallelReads/parallel_read_679
=== RUN   TestRegistry_ParallelReads/parallel_read_680
=== PAUSE TestRegistry_ParallelReads/parallel_read_680
=== RUN   TestRegistry_ParallelReads/parallel_read_681
=== PAUSE TestRegistry_ParallelReads/parallel_read_681
=== RUN   TestRegistry_ParallelReads/parallel_read_682
=== PAUSE TestRegistry_ParallelReads/parallel_read_682
=== RUN   TestRegistry_ParallelReads/parallel_read_683
=== PAUSE TestRegistry_ParallelReads/parallel_read_683
=== RUN   TestRegistry_ParallelReads/parallel_read_684
=== PAUSE TestRegistry_ParallelReads/parallel_read_684
=== RUN   TestRegistry_ParallelReads/parallel_read_685
=== PAUSE TestRegistry_ParallelReads/parallel_read_685
=== RUN   TestRegistry_ParallelReads/parallel_read_686
=== PAUSE TestRegistry_ParallelReads/parallel_read_686
=== RUN   TestRegistry_ParallelReads/parallel_read_687
=== PAUSE TestRegistry_ParallelReads/parallel_read_687
=== RUN   TestRegistry_ParallelReads/parallel_read_688
=== PAUSE TestRegistry_ParallelReads/parallel_read_688
=== RUN   TestRegistry_ParallelReads/parallel_read_689
=== PAUSE TestRegistry_ParallelReads/parallel_read_689
=== RUN   TestRegistry_ParallelReads/parallel_read_690
=== PAUSE TestRegistry_ParallelReads/parallel_read_690
=== RUN   TestRegistry_ParallelReads/parallel_read_691
=== PAUSE TestRegistry_ParallelReads/parallel_read_691
=== RUN   TestRegistry_ParallelReads/parallel_read_692
=== PAUSE TestRegistry_ParallelReads/parallel_read_692
=== RUN   TestRegistry_ParallelReads/parallel_read_693
=== PAUSE TestRegistry_ParallelReads/parallel_read_693
=== RUN   TestRegistry_ParallelReads/parallel_read_694
=== PAUSE TestRegistry_ParallelReads/parallel_read_694
=== RUN   TestRegistry_ParallelReads/parallel_read_695
=== PAUSE TestRegistry_ParallelReads/parallel_read_695
=== RUN   TestRegistry_ParallelReads/parallel_read_696
=== PAUSE TestRegistry_ParallelReads/parallel_read_696
=== RUN   TestRegistry_ParallelReads/parallel_read_697
=== PAUSE TestRegistry_ParallelReads/parallel_read_697
=== RUN   TestRegistry_ParallelReads/parallel_read_698
=== PAUSE TestRegistry_ParallelReads/parallel_read_698
=== RUN   TestRegistry_ParallelReads/parallel_read_699
=== PAUSE TestRegistry_ParallelReads/parallel_read_699
=== RUN   TestRegistry_ParallelReads/parallel_read_700
=== PAUSE TestRegistry_ParallelReads/parallel_read_700
=== RUN   TestRegistry_ParallelReads/parallel_read_701
=== PAUSE TestRegistry_ParallelReads/parallel_read_701
=== RUN   TestRegistry_ParallelReads/parallel_read_702
=== PAUSE TestRegistry_ParallelReads/parallel_read_702
=== RUN   TestRegistry_ParallelReads/parallel_read_703
=== PAUSE TestRegistry_ParallelReads/parallel_read_703
=== RUN   TestRegistry_ParallelReads/parallel_read_704
=== PAUSE TestRegistry_ParallelReads/parallel_read_704
=== RUN   TestRegistry_ParallelReads/parallel_read_705
=== PAUSE TestRegistry_ParallelReads/parallel_read_705
=== RUN   TestRegistry_ParallelReads/parallel_read_706
=== PAUSE TestRegistry_ParallelReads/parallel_read_706
=== RUN   TestRegistry_ParallelReads/parallel_read_707
=== PAUSE TestRegistry_ParallelReads/parallel_read_707
=== RUN   TestRegistry_ParallelReads/parallel_read_708
=== PAUSE TestRegistry_ParallelReads/parallel_read_708
=== RUN   TestRegistry_ParallelReads/parallel_read_709
=== PAUSE TestRegistry_ParallelReads/parallel_read_709
=== RUN   TestRegistry_ParallelReads/parallel_read_710
=== PAUSE TestRegistry_ParallelReads/parallel_read_710
=== RUN   TestRegistry_ParallelReads/parallel_read_711
=== PAUSE TestRegistry_ParallelReads/parallel_read_711
=== RUN   TestRegistry_ParallelReads/parallel_read_712
=== PAUSE TestRegistry_ParallelReads/parallel_read_712
=== RUN   TestRegistry_ParallelReads/parallel_read_713
=== PAUSE TestRegistry_ParallelReads/parallel_read_713
=== RUN   TestRegistry_ParallelReads/parallel_read_714
=== PAUSE TestRegistry_ParallelReads/parallel_read_714
=== RUN   TestRegistry_ParallelReads/parallel_read_715
=== PAUSE TestRegistry_ParallelReads/parallel_read_715
=== RUN   TestRegistry_ParallelReads/parallel_read_716
=== PAUSE TestRegistry_ParallelReads/parallel_read_716
=== RUN   TestRegistry_ParallelReads/parallel_read_717
=== PAUSE TestRegistry_ParallelReads/parallel_read_717
=== RUN   TestRegistry_ParallelReads/parallel_read_718
=== PAUSE TestRegistry_ParallelReads/parallel_read_718
=== RUN   TestRegistry_ParallelReads/parallel_read_719
=== PAUSE TestRegistry_ParallelReads/parallel_read_719
=== RUN   TestRegistry_ParallelReads/parallel_read_720
=== PAUSE TestRegistry_ParallelReads/parallel_read_720
=== RUN   TestRegistry_ParallelReads/parallel_read_721
=== PAUSE TestRegistry_ParallelReads/parallel_read_721
=== RUN   TestRegistry_ParallelReads/parallel_read_722
=== PAUSE TestRegistry_ParallelReads/parallel_read_722
=== RUN   TestRegistry_ParallelReads/parallel_read_723
=== PAUSE TestRegistry_ParallelReads/parallel_read_723
=== RUN   TestRegistry_ParallelReads/parallel_read_724
=== PAUSE TestRegistry_ParallelReads/parallel_read_724
=== RUN   TestRegistry_ParallelReads/parallel_read_725
=== PAUSE TestRegistry_ParallelReads/parallel_read_725
=== RUN   TestRegistry_ParallelReads/parallel_read_726
=== PAUSE TestRegistry_ParallelReads/parallel_read_726
=== RUN   TestRegistry_ParallelReads/parallel_read_727
=== PAUSE TestRegistry_ParallelReads/parallel_read_727
=== RUN   TestRegistry_ParallelReads/parallel_read_728
=== PAUSE TestRegistry_ParallelReads/parallel_read_728
=== RUN   TestRegistry_ParallelReads/parallel_read_729
=== PAUSE TestRegistry_ParallelReads/parallel_read_729
=== RUN   TestRegistry_ParallelReads/parallel_read_730
=== PAUSE TestRegistry_ParallelReads/parallel_read_730
=== RUN   TestRegistry_ParallelReads/parallel_read_731
=== PAUSE TestRegistry_ParallelReads/parallel_read_731
=== RUN   TestRegistry_ParallelReads/parallel_read_732
=== PAUSE TestRegistry_ParallelReads/parallel_read_732
=== RUN   TestRegistry_ParallelReads/parallel_read_733
=== PAUSE TestRegistry_ParallelReads/parallel_read_733
=== RUN   TestRegistry_ParallelReads/parallel_read_734
=== PAUSE TestRegistry_ParallelReads/parallel_read_734
=== RUN   TestRegistry_ParallelReads/parallel_read_735
=== PAUSE TestRegistry_ParallelReads/parallel_read_735
=== RUN   TestRegistry_ParallelReads/parallel_read_736
=== PAUSE TestRegistry_ParallelReads/parallel_read_736
=== RUN   TestRegistry_ParallelReads/parallel_read_737
=== PAUSE TestRegistry_ParallelReads/parallel_read_737
=== RUN   TestRegistry_ParallelReads/parallel_read_738
=== PAUSE TestRegistry_ParallelReads/parallel_read_738
=== RUN   TestRegistry_ParallelReads/parallel_read_739
=== PAUSE TestRegistry_ParallelReads/parallel_read_739
=== RUN   TestRegistry_ParallelReads/parallel_read_740
=== PAUSE TestRegistry_ParallelReads/parallel_read_740
=== RUN   TestRegistry_ParallelReads/parallel_read_741
=== PAUSE TestRegistry_ParallelReads/parallel_read_741
=== RUN   TestRegistry_ParallelReads/parallel_read_742
=== PAUSE TestRegistry_ParallelReads/parallel_read_742
=== RUN   TestRegistry_ParallelReads/parallel_read_743
=== PAUSE TestRegistry_ParallelReads/parallel_read_743
=== RUN   TestRegistry_ParallelReads/parallel_read_744
=== PAUSE TestRegistry_ParallelReads/parallel_read_744
=== RUN   TestRegistry_ParallelReads/parallel_read_745
=== PAUSE TestRegistry_ParallelReads/parallel_read_745
=== RUN   TestRegistry_ParallelReads/parallel_read_746
=== PAUSE TestRegistry_ParallelReads/parallel_read_746
=== RUN   TestRegistry_ParallelReads/parallel_read_747
=== PAUSE TestRegistry_ParallelReads/parallel_read_747
=== RUN   TestRegistry_ParallelReads/parallel_read_748
=== PAUSE TestRegistry_ParallelReads/parallel_read_748
=== RUN   TestRegistry_ParallelReads/parallel_read_749
=== PAUSE TestRegistry_ParallelReads/parallel_read_749
=== RUN   TestRegistry_ParallelReads/parallel_read_750
=== PAUSE TestRegistry_ParallelReads/parallel_read_750
=== RUN   TestRegistry_ParallelReads/parallel_read_751
=== PAUSE TestRegistry_ParallelReads/parallel_read_751
=== RUN   TestRegistry_ParallelReads/parallel_read_752
=== PAUSE TestRegistry_ParallelReads/parallel_read_752
=== RUN   TestRegistry_ParallelReads/parallel_read_753
=== PAUSE TestRegistry_ParallelReads/parallel_read_753
=== RUN   TestRegistry_ParallelReads/parallel_read_754
=== PAUSE TestRegistry_ParallelReads/parallel_read_754
=== RUN   TestRegistry_ParallelReads/parallel_read_755
=== PAUSE TestRegistry_ParallelReads/parallel_read_755
=== RUN   TestRegistry_ParallelReads/parallel_read_756
=== PAUSE TestRegistry_ParallelReads/parallel_read_756
=== RUN   TestRegistry_ParallelReads/parallel_read_757
=== PAUSE TestRegistry_ParallelReads/parallel_read_757
=== RUN   TestRegistry_ParallelReads/parallel_read_758
=== PAUSE TestRegistry_ParallelReads/parallel_read_758
=== RUN   TestRegistry_ParallelReads/parallel_read_759
=== PAUSE TestRegistry_ParallelReads/parallel_read_759
=== RUN   TestRegistry_ParallelReads/parallel_read_760
=== PAUSE TestRegistry_ParallelReads/parallel_read_760
=== RUN   TestRegistry_ParallelReads/parallel_read_761
=== PAUSE TestRegistry_ParallelReads/parallel_read_761
=== RUN   TestRegistry_ParallelReads/parallel_read_762
=== PAUSE TestRegistry_ParallelReads/parallel_read_762
=== RUN   TestRegistry_ParallelReads/parallel_read_763
=== PAUSE TestRegistry_ParallelReads/parallel_read_763
=== RUN   TestRegistry_ParallelReads/parallel_read_764
=== PAUSE TestRegistry_ParallelReads/parallel_read_764
=== RUN   TestRegistry_ParallelReads/parallel_read_765
=== PAUSE TestRegistry_ParallelReads/parallel_read_765
=== RUN   TestRegistry_ParallelReads/parallel_read_766
=== PAUSE TestRegistry_ParallelReads/parallel_read_766
=== RUN   TestRegistry_ParallelReads/parallel_read_767
=== PAUSE TestRegistry_ParallelReads/parallel_read_767
=== RUN   TestRegistry_ParallelReads/parallel_read_768
=== PAUSE TestRegistry_ParallelReads/parallel_read_768
=== RUN   TestRegistry_ParallelReads/parallel_read_769
=== PAUSE TestRegistry_ParallelReads/parallel_read_769
=== RUN   TestRegistry_ParallelReads/parallel_read_770
=== PAUSE TestRegistry_ParallelReads/parallel_read_770
=== RUN   TestRegistry_ParallelReads/parallel_read_771
=== PAUSE TestRegistry_ParallelReads/parallel_read_771
=== RUN   TestRegistry_ParallelReads/parallel_read_772
=== PAUSE TestRegistry_ParallelReads/parallel_read_772
=== RUN   TestRegistry_ParallelReads/parallel_read_773
=== PAUSE TestRegistry_ParallelReads/parallel_read_773
=== RUN   TestRegistry_ParallelReads/parallel_read_774
=== PAUSE TestRegistry_ParallelReads/parallel_read_774
=== RUN   TestRegistry_ParallelReads/parallel_read_775
=== PAUSE TestRegistry_ParallelReads/parallel_read_775
=== RUN   TestRegistry_ParallelReads/parallel_read_776
=== PAUSE TestRegistry_ParallelReads/parallel_read_776
=== RUN   TestRegistry_ParallelReads/parallel_read_777
=== PAUSE TestRegistry_ParallelReads/parallel_read_777
=== RUN   TestRegistry_ParallelReads/parallel_read_778
=== PAUSE TestRegistry_ParallelReads/parallel_read_778
=== RUN   TestRegistry_ParallelReads/parallel_read_779
=== PAUSE TestRegistry_ParallelReads/parallel_read_779
=== RUN   TestRegistry_ParallelReads/parallel_read_780
=== PAUSE TestRegistry_ParallelReads/parallel_read_780
=== RUN   TestRegistry_ParallelReads/parallel_read_781
=== PAUSE TestRegistry_ParallelReads/parallel_read_781
=== RUN   TestRegistry_ParallelReads/parallel_read_782
=== PAUSE TestRegistry_ParallelReads/parallel_read_782
=== RUN   TestRegistry_ParallelReads/parallel_read_783
=== PAUSE TestRegistry_ParallelReads/parallel_read_783
=== RUN   TestRegistry_ParallelReads/parallel_read_784
=== PAUSE TestRegistry_ParallelReads/parallel_read_784
=== RUN   TestRegistry_ParallelReads/parallel_read_785
=== PAUSE TestRegistry_ParallelReads/parallel_read_785
=== RUN   TestRegistry_ParallelReads/parallel_read_786
=== PAUSE TestRegistry_ParallelReads/parallel_read_786
=== RUN   TestRegistry_ParallelReads/parallel_read_787
=== PAUSE TestRegistry_ParallelReads/parallel_read_787
=== RUN   TestRegistry_ParallelReads/parallel_read_788
=== PAUSE TestRegistry_ParallelReads/parallel_read_788
=== RUN   TestRegistry_ParallelReads/parallel_read_789
=== PAUSE TestRegistry_ParallelReads/parallel_read_789
=== RUN   TestRegistry_ParallelReads/parallel_read_790
=== PAUSE TestRegistry_ParallelReads/parallel_read_790
=== RUN   TestRegistry_ParallelReads/parallel_read_791
=== PAUSE TestRegistry_ParallelReads/parallel_read_791
=== RUN   TestRegistry_ParallelReads/parallel_read_792
=== PAUSE TestRegistry_ParallelReads/parallel_read_792
=== RUN   TestRegistry_ParallelReads/parallel_read_793
=== PAUSE TestRegistry_ParallelReads/parallel_read_793
=== RUN   TestRegistry_ParallelReads/parallel_read_794
=== PAUSE TestRegistry_ParallelReads/parallel_read_794
=== RUN   TestRegistry_ParallelReads/parallel_read_795
=== PAUSE TestRegistry_ParallelReads/parallel_read_795
=== RUN   TestRegistry_ParallelReads/parallel_read_796
=== PAUSE TestRegistry_ParallelReads/parallel_read_796
=== RUN   TestRegistry_ParallelReads/parallel_read_797
=== PAUSE TestRegistry_ParallelReads/parallel_read_797
=== RUN   TestRegistry_ParallelReads/parallel_read_798
=== PAUSE TestRegistry_ParallelReads/parallel_read_798
=== RUN   TestRegistry_ParallelReads/parallel_read_799
=== PAUSE TestRegistry_ParallelReads/parallel_read_799
=== RUN   TestRegistry_ParallelReads/parallel_read_800
=== PAUSE TestRegistry_ParallelReads/parallel_read_800
=== RUN   TestRegistry_ParallelReads/parallel_read_801
=== PAUSE TestRegistry_ParallelReads/parallel_read_801
=== RUN   TestRegistry_ParallelReads/parallel_read_802
=== PAUSE TestRegistry_ParallelReads/parallel_read_802
=== RUN   TestRegistry_ParallelReads/parallel_read_803
=== PAUSE TestRegistry_ParallelReads/parallel_read_803
=== RUN   TestRegistry_ParallelReads/parallel_read_804
=== PAUSE TestRegistry_ParallelReads/parallel_read_804
=== RUN   TestRegistry_ParallelReads/parallel_read_805
=== PAUSE TestRegistry_ParallelReads/parallel_read_805
=== RUN   TestRegistry_ParallelReads/parallel_read_806
=== PAUSE TestRegistry_ParallelReads/parallel_read_806
=== RUN   TestRegistry_ParallelReads/parallel_read_807
=== PAUSE TestRegistry_ParallelReads/parallel_read_807
=== RUN   TestRegistry_ParallelReads/parallel_read_808
=== PAUSE TestRegistry_ParallelReads/parallel_read_808
=== RUN   TestRegistry_ParallelReads/parallel_read_809
=== PAUSE TestRegistry_ParallelReads/parallel_read_809
=== RUN   TestRegistry_ParallelReads/parallel_read_810
=== PAUSE TestRegistry_ParallelReads/parallel_read_810
=== RUN   TestRegistry_ParallelReads/parallel_read_811
=== PAUSE TestRegistry_ParallelReads/parallel_read_811
=== RUN   TestRegistry_ParallelReads/parallel_read_812
=== PAUSE TestRegistry_ParallelReads/parallel_read_812
=== RUN   TestRegistry_ParallelReads/parallel_read_813
=== PAUSE TestRegistry_ParallelReads/parallel_read_813
=== RUN   TestRegistry_ParallelReads/parallel_read_814
=== PAUSE TestRegistry_ParallelReads/parallel_read_814
=== RUN   TestRegistry_ParallelReads/parallel_read_815
=== PAUSE TestRegistry_ParallelReads/parallel_read_815
=== RUN   TestRegistry_ParallelReads/parallel_read_816
=== PAUSE TestRegistry_ParallelReads/parallel_read_816
=== RUN   TestRegistry_ParallelReads/parallel_read_817
=== PAUSE TestRegistry_ParallelReads/parallel_read_817
=== RUN   TestRegistry_ParallelReads/parallel_read_818
=== PAUSE TestRegistry_ParallelReads/parallel_read_818
=== RUN   TestRegistry_ParallelReads/parallel_read_819
=== PAUSE TestRegistry_ParallelReads/parallel_read_819
=== RUN   TestRegistry_ParallelReads/parallel_read_820
=== PAUSE TestRegistry_ParallelReads/parallel_read_820
=== RUN   TestRegistry_ParallelReads/parallel_read_821
=== PAUSE TestRegistry_ParallelReads/parallel_read_821
=== RUN   TestRegistry_ParallelReads/parallel_read_822
=== PAUSE TestRegistry_ParallelReads/parallel_read_822
=== RUN   TestRegistry_ParallelReads/parallel_read_823
=== PAUSE TestRegistry_ParallelReads/parallel_read_823
=== RUN   TestRegistry_ParallelReads/parallel_read_824
=== PAUSE TestRegistry_ParallelReads/parallel_read_824
=== RUN   TestRegistry_ParallelReads/parallel_read_825
=== PAUSE TestRegistry_ParallelReads/parallel_read_825
=== RUN   TestRegistry_ParallelReads/parallel_read_826
=== PAUSE TestRegistry_ParallelReads/parallel_read_826
=== RUN   TestRegistry_ParallelReads/parallel_read_827
=== PAUSE TestRegistry_ParallelReads/parallel_read_827
=== RUN   TestRegistry_ParallelReads/parallel_read_828
=== PAUSE TestRegistry_ParallelReads/parallel_read_828
=== RUN   TestRegistry_ParallelReads/parallel_read_829
=== PAUSE TestRegistry_ParallelReads/parallel_read_829
=== RUN   TestRegistry_ParallelReads/parallel_read_830
=== PAUSE TestRegistry_ParallelReads/parallel_read_830
=== RUN   TestRegistry_ParallelReads/parallel_read_831
=== PAUSE TestRegistry_ParallelReads/parallel_read_831
=== RUN   TestRegistry_ParallelReads/parallel_read_832
=== PAUSE TestRegistry_ParallelReads/parallel_read_832
=== RUN   TestRegistry_ParallelReads/parallel_read_833
=== PAUSE TestRegistry_ParallelReads/parallel_read_833
=== RUN   TestRegistry_ParallelReads/parallel_read_834
=== PAUSE TestRegistry_ParallelReads/parallel_read_834
=== RUN   TestRegistry_ParallelReads/parallel_read_835
=== PAUSE TestRegistry_ParallelReads/parallel_read_835
=== RUN   TestRegistry_ParallelReads/parallel_read_836
=== PAUSE TestRegistry_ParallelReads/parallel_read_836
=== RUN   TestRegistry_ParallelReads/parallel_read_837
=== PAUSE TestRegistry_ParallelReads/parallel_read_837
=== RUN   TestRegistry_ParallelReads/parallel_read_838
=== PAUSE TestRegistry_ParallelReads/parallel_read_838
=== RUN   TestRegistry_ParallelReads/parallel_read_839
=== PAUSE TestRegistry_ParallelReads/parallel_read_839
=== RUN   TestRegistry_ParallelReads/parallel_read_840
=== PAUSE TestRegistry_ParallelReads/parallel_read_840
=== RUN   TestRegistry_ParallelReads/parallel_read_841
=== PAUSE TestRegistry_ParallelReads/parallel_read_841
=== RUN   TestRegistry_ParallelReads/parallel_read_842
=== PAUSE TestRegistry_ParallelReads/parallel_read_842
=== RUN   TestRegistry_ParallelReads/parallel_read_843
=== PAUSE TestRegistry_ParallelReads/parallel_read_843
=== RUN   TestRegistry_ParallelReads/parallel_read_844
=== PAUSE TestRegistry_ParallelReads/parallel_read_844
=== RUN   TestRegistry_ParallelReads/parallel_read_845
=== PAUSE TestRegistry_ParallelReads/parallel_read_845
=== RUN   TestRegistry_ParallelReads/parallel_read_846
=== PAUSE TestRegistry_ParallelReads/parallel_read_846
=== RUN   TestRegistry_ParallelReads/parallel_read_847
=== PAUSE TestRegistry_ParallelReads/parallel_read_847
=== RUN   TestRegistry_ParallelReads/parallel_read_848
=== PAUSE TestRegistry_ParallelReads/parallel_read_848
=== RUN   TestRegistry_ParallelReads/parallel_read_849
=== PAUSE TestRegistry_ParallelReads/parallel_read_849
=== RUN   TestRegistry_ParallelReads/parallel_read_850
=== PAUSE TestRegistry_ParallelReads/parallel_read_850
=== RUN   TestRegistry_ParallelReads/parallel_read_851
=== PAUSE TestRegistry_ParallelReads/parallel_read_851
=== RUN   TestRegistry_ParallelReads/parallel_read_852
=== PAUSE TestRegistry_ParallelReads/parallel_read_852
=== RUN   TestRegistry_ParallelReads/parallel_read_853
=== PAUSE TestRegistry_ParallelReads/parallel_read_853
=== RUN   TestRegistry_ParallelReads/parallel_read_854
=== PAUSE TestRegistry_ParallelReads/parallel_read_854
=== RUN   TestRegistry_ParallelReads/parallel_read_855
=== PAUSE TestRegistry_ParallelReads/parallel_read_855
=== RUN   TestRegistry_ParallelReads/parallel_read_856
=== PAUSE TestRegistry_ParallelReads/parallel_read_856
=== RUN   TestRegistry_ParallelReads/parallel_read_857
=== PAUSE TestRegistry_ParallelReads/parallel_read_857
=== RUN   TestRegistry_ParallelReads/parallel_read_858
=== PAUSE TestRegistry_ParallelReads/parallel_read_858
=== RUN   TestRegistry_ParallelReads/parallel_read_859
=== PAUSE TestRegistry_ParallelReads/parallel_read_859
=== RUN   TestRegistry_ParallelReads/parallel_read_860
=== PAUSE TestRegistry_ParallelReads/parallel_read_860
=== RUN   TestRegistry_ParallelReads/parallel_read_861
=== PAUSE TestRegistry_ParallelReads/parallel_read_861
=== RUN   TestRegistry_ParallelReads/parallel_read_862
=== PAUSE TestRegistry_ParallelReads/parallel_read_862
=== RUN   TestRegistry_ParallelReads/parallel_read_863
=== PAUSE TestRegistry_ParallelReads/parallel_read_863
=== RUN   TestRegistry_ParallelReads/parallel_read_864
=== PAUSE TestRegistry_ParallelReads/parallel_read_864
=== RUN   TestRegistry_ParallelReads/parallel_read_865
=== PAUSE TestRegistry_ParallelReads/parallel_read_865
=== RUN   TestRegistry_ParallelReads/parallel_read_866
=== PAUSE TestRegistry_ParallelReads/parallel_read_866
=== RUN   TestRegistry_ParallelReads/parallel_read_867
=== PAUSE TestRegistry_ParallelReads/parallel_read_867
=== RUN   TestRegistry_ParallelReads/parallel_read_868
=== PAUSE TestRegistry_ParallelReads/parallel_read_868
=== RUN   TestRegistry_ParallelReads/parallel_read_869
=== PAUSE TestRegistry_ParallelReads/parallel_read_869
=== RUN   TestRegistry_ParallelReads/parallel_read_870
=== PAUSE TestRegistry_ParallelReads/parallel_read_870
=== RUN   TestRegistry_ParallelReads/parallel_read_871
=== PAUSE TestRegistry_ParallelReads/parallel_read_871
=== RUN   TestRegistry_ParallelReads/parallel_read_872
=== PAUSE TestRegistry_ParallelReads/parallel_read_872
=== RUN   TestRegistry_ParallelReads/parallel_read_873
=== PAUSE TestRegistry_ParallelReads/parallel_read_873
=== RUN   TestRegistry_ParallelReads/parallel_read_874
=== PAUSE TestRegistry_ParallelReads/parallel_read_874
=== RUN   TestRegistry_ParallelReads/parallel_read_875
=== PAUSE TestRegistry_ParallelReads/parallel_read_875
=== RUN   TestRegistry_ParallelReads/parallel_read_876
=== PAUSE TestRegistry_ParallelReads/parallel_read_876
=== RUN   TestRegistry_ParallelReads/parallel_read_877
=== PAUSE TestRegistry_ParallelReads/parallel_read_877
=== RUN   TestRegistry_ParallelReads/parallel_read_878
=== PAUSE TestRegistry_ParallelReads/parallel_read_878
=== RUN   TestRegistry_ParallelReads/parallel_read_879
=== PAUSE TestRegistry_ParallelReads/parallel_read_879
=== RUN   TestRegistry_ParallelReads/parallel_read_880
=== PAUSE TestRegistry_ParallelReads/parallel_read_880
=== RUN   TestRegistry_ParallelReads/parallel_read_881
=== PAUSE TestRegistry_ParallelReads/parallel_read_881
=== RUN   TestRegistry_ParallelReads/parallel_read_882
=== PAUSE TestRegistry_ParallelReads/parallel_read_882
=== RUN   TestRegistry_ParallelReads/parallel_read_883
=== PAUSE TestRegistry_ParallelReads/parallel_read_883
=== RUN   TestRegistry_ParallelReads/parallel_read_884
=== PAUSE TestRegistry_ParallelReads/parallel_read_884
=== RUN   TestRegistry_ParallelReads/parallel_read_885
=== PAUSE TestRegistry_ParallelReads/parallel_read_885
=== RUN   TestRegistry_ParallelReads/parallel_read_886
=== PAUSE TestRegistry_ParallelReads/parallel_read_886
=== RUN   TestRegistry_ParallelReads/parallel_read_887
=== PAUSE TestRegistry_ParallelReads/parallel_read_887
=== RUN   TestRegistry_ParallelReads/parallel_read_888
=== PAUSE TestRegistry_ParallelReads/parallel_read_888
=== RUN   TestRegistry_ParallelReads/parallel_read_889
=== PAUSE TestRegistry_ParallelReads/parallel_read_889
=== RUN   TestRegistry_ParallelReads/parallel_read_890
=== PAUSE TestRegistry_ParallelReads/parallel_read_890
=== RUN   TestRegistry_ParallelReads/parallel_read_891
=== PAUSE TestRegistry_ParallelReads/parallel_read_891
=== RUN   TestRegistry_ParallelReads/parallel_read_892
=== PAUSE TestRegistry_ParallelReads/parallel_read_892
=== RUN   TestRegistry_ParallelReads/parallel_read_893
=== PAUSE TestRegistry_ParallelReads/parallel_read_893
=== RUN   TestRegistry_ParallelReads/parallel_read_894
=== PAUSE TestRegistry_ParallelReads/parallel_read_894
=== RUN   TestRegistry_ParallelReads/parallel_read_895
=== PAUSE TestRegistry_ParallelReads/parallel_read_895
=== RUN   TestRegistry_ParallelReads/parallel_read_896
=== PAUSE TestRegistry_ParallelReads/parallel_read_896
=== RUN   TestRegistry_ParallelReads/parallel_read_897
=== PAUSE TestRegistry_ParallelReads/parallel_read_897
=== RUN   TestRegistry_ParallelReads/parallel_read_898
=== PAUSE TestRegistry_ParallelReads/parallel_read_898
=== RUN   TestRegistry_ParallelReads/parallel_read_899
=== PAUSE TestRegistry_ParallelReads/parallel_read_899
=== RUN   TestRegistry_ParallelReads/parallel_read_900
=== PAUSE TestRegistry_ParallelReads/parallel_read_900
=== RUN   TestRegistry_ParallelReads/parallel_read_901
=== PAUSE TestRegistry_ParallelReads/parallel_read_901
=== RUN   TestRegistry_ParallelReads/parallel_read_902
=== PAUSE TestRegistry_ParallelReads/parallel_read_902
=== RUN   TestRegistry_ParallelReads/parallel_read_903
=== PAUSE TestRegistry_ParallelReads/parallel_read_903
=== RUN   TestRegistry_ParallelReads/parallel_read_904
=== PAUSE TestRegistry_ParallelReads/parallel_read_904
=== RUN   TestRegistry_ParallelReads/parallel_read_905
=== PAUSE TestRegistry_ParallelReads/parallel_read_905
=== RUN   TestRegistry_ParallelReads/parallel_read_906
=== PAUSE TestRegistry_ParallelReads/parallel_read_906
=== RUN   TestRegistry_ParallelReads/parallel_read_907
=== PAUSE TestRegistry_ParallelReads/parallel_read_907
=== RUN   TestRegistry_ParallelReads/parallel_read_908
=== PAUSE TestRegistry_ParallelReads/parallel_read_908
=== RUN   TestRegistry_ParallelReads/parallel_read_909
=== PAUSE TestRegistry_ParallelReads/parallel_read_909
=== RUN   TestRegistry_ParallelReads/parallel_read_910
=== PAUSE TestRegistry_ParallelReads/parallel_read_910
=== RUN   TestRegistry_ParallelReads/parallel_read_911
=== PAUSE TestRegistry_ParallelReads/parallel_read_911
=== RUN   TestRegistry_ParallelReads/parallel_read_912
=== PAUSE TestRegistry_ParallelReads/parallel_read_912
=== RUN   TestRegistry_ParallelReads/parallel_read_913
=== PAUSE TestRegistry_ParallelReads/parallel_read_913
=== RUN   TestRegistry_ParallelReads/parallel_read_914
=== PAUSE TestRegistry_ParallelReads/parallel_read_914
=== RUN   TestRegistry_ParallelReads/parallel_read_915
=== PAUSE TestRegistry_ParallelReads/parallel_read_915
=== RUN   TestRegistry_ParallelReads/parallel_read_916
=== PAUSE TestRegistry_ParallelReads/parallel_read_916
=== RUN   TestRegistry_ParallelReads/parallel_read_917
=== PAUSE TestRegistry_ParallelReads/parallel_read_917
=== RUN   TestRegistry_ParallelReads/parallel_read_918
=== PAUSE TestRegistry_ParallelReads/parallel_read_918
=== RUN   TestRegistry_ParallelReads/parallel_read_919
=== PAUSE TestRegistry_ParallelReads/parallel_read_919
=== RUN   TestRegistry_ParallelReads/parallel_read_920
=== PAUSE TestRegistry_ParallelReads/parallel_read_920
=== RUN   TestRegistry_ParallelReads/parallel_read_921
=== PAUSE TestRegistry_ParallelReads/parallel_read_921
=== RUN   TestRegistry_ParallelReads/parallel_read_922
=== PAUSE TestRegistry_ParallelReads/parallel_read_922
=== RUN   TestRegistry_ParallelReads/parallel_read_923
=== PAUSE TestRegistry_ParallelReads/parallel_read_923
=== RUN   TestRegistry_ParallelReads/parallel_read_924
=== PAUSE TestRegistry_ParallelReads/parallel_read_924
=== RUN   TestRegistry_ParallelReads/parallel_read_925
=== PAUSE TestRegistry_ParallelReads/parallel_read_925
=== RUN   TestRegistry_ParallelReads/parallel_read_926
=== PAUSE TestRegistry_ParallelReads/parallel_read_926
=== RUN   TestRegistry_ParallelReads/parallel_read_927
=== PAUSE TestRegistry_ParallelReads/parallel_read_927
=== RUN   TestRegistry_ParallelReads/parallel_read_928
=== PAUSE TestRegistry_ParallelReads/parallel_read_928
=== RUN   TestRegistry_ParallelReads/parallel_read_929
=== PAUSE TestRegistry_ParallelReads/parallel_read_929
=== RUN   TestRegistry_ParallelReads/parallel_read_930
=== PAUSE TestRegistry_ParallelReads/parallel_read_930
=== RUN   TestRegistry_ParallelReads/parallel_read_931
=== PAUSE TestRegistry_ParallelReads/parallel_read_931
=== RUN   TestRegistry_ParallelReads/parallel_read_932
=== PAUSE TestRegistry_ParallelReads/parallel_read_932
=== RUN   TestRegistry_ParallelReads/parallel_read_933
=== PAUSE TestRegistry_ParallelReads/parallel_read_933
=== RUN   TestRegistry_ParallelReads/parallel_read_934
=== PAUSE TestRegistry_ParallelReads/parallel_read_934
=== RUN   TestRegistry_ParallelReads/parallel_read_935
=== PAUSE TestRegistry_ParallelReads/parallel_read_935
=== RUN   TestRegistry_ParallelReads/parallel_read_936
=== PAUSE TestRegistry_ParallelReads/parallel_read_936
=== RUN   TestRegistry_ParallelReads/parallel_read_937
=== PAUSE TestRegistry_ParallelReads/parallel_read_937
=== RUN   TestRegistry_ParallelReads/parallel_read_938
=== PAUSE TestRegistry_ParallelReads/parallel_read_938
=== RUN   TestRegistry_ParallelReads/parallel_read_939
=== PAUSE TestRegistry_ParallelReads/parallel_read_939
=== RUN   TestRegistry_ParallelReads/parallel_read_940
=== PAUSE TestRegistry_ParallelReads/parallel_read_940
=== RUN   TestRegistry_ParallelReads/parallel_read_941
=== PAUSE TestRegistry_ParallelReads/parallel_read_941
=== RUN   TestRegistry_ParallelReads/parallel_read_942
=== PAUSE TestRegistry_ParallelReads/parallel_read_942
=== RUN   TestRegistry_ParallelReads/parallel_read_943
=== PAUSE TestRegistry_ParallelReads/parallel_read_943
=== RUN   TestRegistry_ParallelReads/parallel_read_944
=== PAUSE TestRegistry_ParallelReads/parallel_read_944
=== RUN   TestRegistry_ParallelReads/parallel_read_945
=== PAUSE TestRegistry_ParallelReads/parallel_read_945
=== RUN   TestRegistry_ParallelReads/parallel_read_946
=== PAUSE TestRegistry_ParallelReads/parallel_read_946
=== RUN   TestRegistry_ParallelReads/parallel_read_947
=== PAUSE TestRegistry_ParallelReads/parallel_read_947
=== RUN   TestRegistry_ParallelReads/parallel_read_948
=== PAUSE TestRegistry_ParallelReads/parallel_read_948
=== RUN   TestRegistry_ParallelReads/parallel_read_949
=== PAUSE TestRegistry_ParallelReads/parallel_read_949
=== RUN   TestRegistry_ParallelReads/parallel_read_950
=== PAUSE TestRegistry_ParallelReads/parallel_read_950
=== RUN   TestRegistry_ParallelReads/parallel_read_951
=== PAUSE TestRegistry_ParallelReads/parallel_read_951
=== RUN   TestRegistry_ParallelReads/parallel_read_952
=== PAUSE TestRegistry_ParallelReads/parallel_read_952
=== RUN   TestRegistry_ParallelReads/parallel_read_953
=== PAUSE TestRegistry_ParallelReads/parallel_read_953
=== RUN   TestRegistry_ParallelReads/parallel_read_954
=== PAUSE TestRegistry_ParallelReads/parallel_read_954
=== RUN   TestRegistry_ParallelReads/parallel_read_955
=== PAUSE TestRegistry_ParallelReads/parallel_read_955
=== RUN   TestRegistry_ParallelReads/parallel_read_956
=== PAUSE TestRegistry_ParallelReads/parallel_read_956
=== RUN   TestRegistry_ParallelReads/parallel_read_957
=== PAUSE TestRegistry_ParallelReads/parallel_read_957
=== RUN   TestRegistry_ParallelReads/parallel_read_958
=== PAUSE TestRegistry_ParallelReads/parallel_read_958
=== RUN   TestRegistry_ParallelReads/parallel_read_959
=== PAUSE TestRegistry_ParallelReads/parallel_read_959
=== RUN   TestRegistry_ParallelReads/parallel_read_960
=== PAUSE TestRegistry_ParallelReads/parallel_read_960
=== RUN   TestRegistry_ParallelReads/parallel_read_961
=== PAUSE TestRegistry_ParallelReads/parallel_read_961
=== RUN   TestRegistry_ParallelReads/parallel_read_962
=== PAUSE TestRegistry_ParallelReads/parallel_read_962
=== RUN   TestRegistry_ParallelReads/parallel_read_963
=== PAUSE TestRegistry_ParallelReads/parallel_read_963
=== RUN   TestRegistry_ParallelReads/parallel_read_964
=== PAUSE TestRegistry_ParallelReads/parallel_read_964
=== RUN   TestRegistry_ParallelReads/parallel_read_965
=== PAUSE TestRegistry_ParallelReads/parallel_read_965
=== RUN   TestRegistry_ParallelReads/parallel_read_966
=== PAUSE TestRegistry_ParallelReads/parallel_read_966
=== RUN   TestRegistry_ParallelReads/parallel_read_967
=== PAUSE TestRegistry_ParallelReads/parallel_read_967
=== RUN   TestRegistry_ParallelReads/parallel_read_968
=== PAUSE TestRegistry_ParallelReads/parallel_read_968
=== RUN   TestRegistry_ParallelReads/parallel_read_969
=== PAUSE TestRegistry_ParallelReads/parallel_read_969
=== RUN   TestRegistry_ParallelReads/parallel_read_970
=== PAUSE TestRegistry_ParallelReads/parallel_read_970
=== RUN   TestRegistry_ParallelReads/parallel_read_971
=== PAUSE TestRegistry_ParallelReads/parallel_read_971
=== RUN   TestRegistry_ParallelReads/parallel_read_972
=== PAUSE TestRegistry_ParallelReads/parallel_read_972
=== RUN   TestRegistry_ParallelReads/parallel_read_973
=== PAUSE TestRegistry_ParallelReads/parallel_read_973
=== RUN   TestRegistry_ParallelReads/parallel_read_974
=== PAUSE TestRegistry_ParallelReads/parallel_read_974
=== RUN   TestRegistry_ParallelReads/parallel_read_975
=== PAUSE TestRegistry_ParallelReads/parallel_read_975
=== RUN   TestRegistry_ParallelReads/parallel_read_976
=== PAUSE TestRegistry_ParallelReads/parallel_read_976
=== RUN   TestRegistry_ParallelReads/parallel_read_977
=== PAUSE TestRegistry_ParallelReads/parallel_read_977
=== RUN   TestRegistry_ParallelReads/parallel_read_978
=== PAUSE TestRegistry_ParallelReads/parallel_read_978
=== RUN   TestRegistry_ParallelReads/parallel_read_979
=== PAUSE TestRegistry_ParallelReads/parallel_read_979
=== RUN   TestRegistry_ParallelReads/parallel_read_980
=== PAUSE TestRegistry_ParallelReads/parallel_read_980
=== RUN   TestRegistry_ParallelReads/parallel_read_981
=== PAUSE TestRegistry_ParallelReads/parallel_read_981
=== RUN   TestRegistry_ParallelReads/parallel_read_982
=== PAUSE TestRegistry_ParallelReads/parallel_read_982
=== RUN   TestRegistry_ParallelReads/parallel_read_983
=== PAUSE TestRegistry_ParallelReads/parallel_read_983
=== RUN   TestRegistry_ParallelReads/parallel_read_984
=== PAUSE TestRegistry_ParallelReads/parallel_read_984
=== RUN   TestRegistry_ParallelReads/parallel_read_985
=== PAUSE TestRegistry_ParallelReads/parallel_read_985
=== RUN   TestRegistry_ParallelReads/parallel_read_986
=== PAUSE TestRegistry_ParallelReads/parallel_read_986
=== RUN   TestRegistry_ParallelReads/parallel_read_987
=== PAUSE TestRegistry_ParallelReads/parallel_read_987
=== RUN   TestRegistry_ParallelReads/parallel_read_988
=== PAUSE TestRegistry_ParallelReads/parallel_read_988
=== RUN   TestRegistry_ParallelReads/parallel_read_989
=== PAUSE TestRegistry_ParallelReads/parallel_read_989
=== RUN   TestRegistry_ParallelReads/parallel_read_990
=== PAUSE TestRegistry_ParallelReads/parallel_read_990
=== RUN   TestRegistry_ParallelReads/parallel_read_991
=== PAUSE TestRegistry_ParallelReads/parallel_read_991
=== RUN   TestRegistry_ParallelReads/parallel_read_992
=== PAUSE TestRegistry_ParallelReads/parallel_read_992
=== RUN   TestRegistry_ParallelReads/parallel_read_993
=== PAUSE TestRegistry_ParallelReads/parallel_read_993
=== RUN   TestRegistry_ParallelReads/parallel_read_994
=== PAUSE TestRegistry_ParallelReads/parallel_read_994
=== RUN   TestRegistry_ParallelReads/parallel_read_995
=== PAUSE TestRegistry_ParallelReads/parallel_read_995
=== RUN   TestRegistry_ParallelReads/parallel_read_996
=== PAUSE TestRegistry_ParallelReads/parallel_read_996
=== RUN   TestRegistry_ParallelReads/parallel_read_997
=== PAUSE TestRegistry_ParallelReads/parallel_read_997
=== RUN   TestRegistry_ParallelReads/parallel_read_998
=== PAUSE TestRegistry_ParallelReads/parallel_read_998
=== RUN   TestRegistry_ParallelReads/parallel_read_999
=== PAUSE TestRegistry_ParallelReads/parallel_read_999
=== CONT  TestRegistry_ParallelReads/parallel_read_0
=== CONT  TestRegistry_ParallelReads/parallel_read_984
=== CONT  TestRegistry_ParallelReads/parallel_read_986
=== CONT  TestRegistry_ParallelReads/parallel_read_964
=== CONT  TestRegistry_ParallelReads/parallel_read_939
=== CONT  TestRegistry_ParallelReads/parallel_read_965
=== CONT  TestRegistry_ParallelReads/parallel_read_993
=== CONT  TestRegistry_ParallelReads/parallel_read_898
=== CONT  TestRegistry_ParallelReads/parallel_read_990
=== CONT  TestRegistry_ParallelReads/parallel_read_878
=== CONT  TestRegistry_ParallelReads/parallel_read_937
=== CONT  TestRegistry_ParallelReads/parallel_read_851
=== CONT  TestRegistry_ParallelReads/parallel_read_822
=== CONT  TestRegistry_ParallelReads/parallel_read_949
=== CONT  TestRegistry_ParallelReads/parallel_read_884
=== CONT  TestRegistry_ParallelReads/parallel_read_943
=== CONT  TestRegistry_ParallelReads/parallel_read_863
=== CONT  TestRegistry_ParallelReads/parallel_read_754
=== CONT  TestRegistry_ParallelReads/parallel_read_953
=== CONT  TestRegistry_ParallelReads/parallel_read_961
=== CONT  TestRegistry_ParallelReads/parallel_read_762
=== CONT  TestRegistry_ParallelReads/parallel_read_905
=== CONT  TestRegistry_ParallelReads/parallel_read_959
=== CONT  TestRegistry_ParallelReads/parallel_read_776
=== CONT  TestRegistry_ParallelReads/parallel_read_828
=== CONT  TestRegistry_ParallelReads/parallel_read_794
=== CONT  TestRegistry_ParallelReads/parallel_read_684
=== CONT  TestRegistry_ParallelReads/parallel_read_870
=== CONT  TestRegistry_ParallelReads/parallel_read_836
=== CONT  TestRegistry_ParallelReads/parallel_read_673
=== CONT  TestRegistry_ParallelReads/parallel_read_946
=== CONT  TestRegistry_ParallelReads/parallel_read_908
=== CONT  TestRegistry_ParallelReads/parallel_read_743
=== CONT  TestRegistry_ParallelReads/parallel_read_849
=== CONT  TestRegistry_ParallelReads/parallel_read_938
=== CONT  TestRegistry_ParallelReads/parallel_read_720
=== CONT  TestRegistry_ParallelReads/parallel_read_648
=== CONT  TestRegistry_ParallelReads/parallel_read_792
=== CONT  TestRegistry_ParallelReads/parallel_read_696
=== CONT  TestRegistry_ParallelReads/parallel_read_790
=== CONT  TestRegistry_ParallelReads/parallel_read_771
=== CONT  TestRegistry_ParallelReads/parallel_read_734
=== CONT  TestRegistry_ParallelReads/parallel_read_640
=== CONT  TestRegistry_ParallelReads/parallel_read_751
=== CONT  TestRegistry_ParallelReads/parallel_read_783
=== CONT  TestRegistry_ParallelReads/parallel_read_638
=== CONT  TestRegistry_ParallelReads/parallel_read_781
=== CONT  TestRegistry_ParallelReads/parallel_read_906
=== CONT  TestRegistry_ParallelReads/parallel_read_856
=== CONT  TestRegistry_ParallelReads/parallel_read_791
=== CONT  TestRegistry_ParallelReads/parallel_read_677
=== CONT  TestRegistry_ParallelReads/parallel_read_688
=== CONT  TestRegistry_ParallelReads/parallel_read_954
=== CONT  TestRegistry_ParallelReads/parallel_read_992
=== CONT  TestRegistry_ParallelReads/parallel_read_977
=== CONT  TestRegistry_ParallelReads/parallel_read_942
=== CONT  TestRegistry_ParallelReads/parallel_read_838
=== CONT  TestRegistry_ParallelReads/parallel_read_901
=== CONT  TestRegistry_ParallelReads/parallel_read_710
=== CONT  TestRegistry_ParallelReads/parallel_read_745
=== CONT  TestRegistry_ParallelReads/parallel_read_883
=== CONT  TestRegistry_ParallelReads/parallel_read_727
=== CONT  TestRegistry_ParallelReads/parallel_read_697
=== CONT  TestRegistry_ParallelReads/parallel_read_933
=== CONT  TestRegistry_ParallelReads/parallel_read_616
=== CONT  TestRegistry_ParallelReads/parallel_read_666
=== CONT  TestRegistry_ParallelReads/parallel_read_922
=== CONT  TestRegistry_ParallelReads/parallel_read_829
=== CONT  TestRegistry_ParallelReads/parallel_read_874
=== CONT  TestRegistry_ParallelReads/parallel_read_721
=== CONT  TestRegistry_ParallelReads/parallel_read_969
=== CONT  TestRegistry_ParallelReads/parallel_read_665
=== CONT  TestRegistry_ParallelReads/parallel_read_722
=== CONT  TestRegistry_ParallelReads/parallel_read_694
=== CONT  TestRegistry_ParallelReads/parallel_read_896
=== CONT  TestRegistry_ParallelReads/parallel_read_625
=== CONT  TestRegistry_ParallelReads/parallel_read_698
=== CONT  TestRegistry_ParallelReads/parallel_read_662
=== CONT  TestRegistry_ParallelReads/parallel_read_733
=== CONT  TestRegistry_ParallelReads/parallel_read_983
=== CONT  TestRegistry_ParallelReads/parallel_read_736
=== CONT  TestRegistry_ParallelReads/parallel_read_728
=== CONT  TestRegistry_ParallelReads/parallel_read_768
=== CONT  TestRegistry_ParallelReads/parallel_read_691
=== CONT  TestRegistry_ParallelReads/parallel_read_687
=== CONT  TestRegistry_ParallelReads/parallel_read_941
=== CONT  TestRegistry_ParallelReads/parallel_read_695
=== CONT  TestRegistry_ParallelReads/parallel_read_889
=== CONT  TestRegistry_ParallelReads/parallel_read_224
=== CONT  TestRegistry_ParallelReads/parallel_read_647
=== CONT  TestRegistry_ParallelReads/parallel_read_746
=== CONT  TestRegistry_ParallelReads/parallel_read_775
=== CONT  TestRegistry_ParallelReads/parallel_read_778
=== CONT  TestRegistry_ParallelReads/parallel_read_659
=== CONT  TestRegistry_ParallelReads/parallel_read_699
=== CONT  TestRegistry_ParallelReads/parallel_read_947
=== CONT  TestRegistry_ParallelReads/parallel_read_692
=== CONT  TestRegistry_ParallelReads/parallel_read_617
=== CONT  TestRegistry_ParallelReads/parallel_read_814
=== CONT  TestRegistry_ParallelReads/parallel_read_873
=== CONT  TestRegistry_ParallelReads/parallel_read_221
=== CONT  TestRegistry_ParallelReads/parallel_read_858
=== CONT  TestRegistry_ParallelReads/parallel_read_709
=== CONT  TestRegistry_ParallelReads/parallel_read_211
=== CONT  TestRegistry_ParallelReads/parallel_read_875
=== CONT  TestRegistry_ParallelReads/parallel_read_824
=== CONT  TestRegistry_ParallelReads/parallel_read_882
=== CONT  TestRegistry_ParallelReads/parallel_read_634
=== CONT  TestRegistry_ParallelReads/parallel_read_327
=== CONT  TestRegistry_ParallelReads/parallel_read_395
=== CONT  TestRegistry_ParallelReads/parallel_read_244
=== CONT  TestRegistry_ParallelReads/parallel_read_210
=== CONT  TestRegistry_ParallelReads/parallel_read_251
=== CONT  TestRegistry_ParallelReads/parallel_read_459
=== CONT  TestRegistry_ParallelReads/parallel_read_358
=== CONT  TestRegistry_ParallelReads/parallel_read_212
=== CONT  TestRegistry_ParallelReads/parallel_read_323
=== CONT  TestRegistry_ParallelReads/parallel_read_382
=== CONT  TestRegistry_ParallelReads/parallel_read_872
=== CONT  TestRegistry_ParallelReads/parallel_read_415
=== CONT  TestRegistry_ParallelReads/parallel_read_486
=== CONT  TestRegistry_ParallelReads/parallel_read_334
=== CONT  TestRegistry_ParallelReads/parallel_read_280
=== CONT  TestRegistry_ParallelReads/parallel_read_407
=== CONT  TestRegistry_ParallelReads/parallel_read_363
=== CONT  TestRegistry_ParallelReads/parallel_read_369
=== CONT  TestRegistry_ParallelReads/parallel_read_422
=== CONT  TestRegistry_ParallelReads/parallel_read_385
=== CONT  TestRegistry_ParallelReads/parallel_read_456
=== CONT  TestRegistry_ParallelReads/parallel_read_669
=== CONT  TestRegistry_ParallelReads/parallel_read_464
=== CONT  TestRegistry_ParallelReads/parallel_read_465
=== CONT  TestRegistry_ParallelReads/parallel_read_389
=== CONT  TestRegistry_ParallelReads/parallel_read_380
=== CONT  TestRegistry_ParallelReads/parallel_read_466
=== CONT  TestRegistry_ParallelReads/parallel_read_272
=== CONT  TestRegistry_ParallelReads/parallel_read_393
=== CONT  TestRegistry_ParallelReads/parallel_read_450
=== CONT  TestRegistry_ParallelReads/parallel_read_396
=== CONT  TestRegistry_ParallelReads/parallel_read_399
=== CONT  TestRegistry_ParallelReads/parallel_read_432
=== CONT  TestRegistry_ParallelReads/parallel_read_461
=== CONT  TestRegistry_ParallelReads/parallel_read_364
=== CONT  TestRegistry_ParallelReads/parallel_read_284
=== CONT  TestRegistry_ParallelReads/parallel_read_204
=== CONT  TestRegistry_ParallelReads/parallel_read_277
=== CONT  TestRegistry_ParallelReads/parallel_read_202
=== CONT  TestRegistry_ParallelReads/parallel_read_424
=== CONT  TestRegistry_ParallelReads/parallel_read_412
=== CONT  TestRegistry_ParallelReads/parallel_read_258
=== CONT  TestRegistry_ParallelReads/parallel_read_420
=== CONT  TestRegistry_ParallelReads/parallel_read_302
=== CONT  TestRegistry_ParallelReads/parallel_read_232
=== CONT  TestRegistry_ParallelReads/parallel_read_480
=== CONT  TestRegistry_ParallelReads/parallel_read_349
=== CONT  TestRegistry_ParallelReads/parallel_read_197
=== CONT  TestRegistry_ParallelReads/parallel_read_670
=== CONT  TestRegistry_ParallelReads/parallel_read_429
=== CONT  TestRegistry_ParallelReads/parallel_read_205
=== CONT  TestRegistry_ParallelReads/parallel_read_271
=== CONT  TestRegistry_ParallelReads/parallel_read_201
=== CONT  TestRegistry_ParallelReads/parallel_read_190
=== CONT  TestRegistry_ParallelReads/parallel_read_230
=== CONT  TestRegistry_ParallelReads/parallel_read_449
=== CONT  TestRegistry_ParallelReads/parallel_read_188
=== CONT  TestRegistry_ParallelReads/parallel_read_255
=== CONT  TestRegistry_ParallelReads/parallel_read_257
=== CONT  TestRegistry_ParallelReads/parallel_read_263
=== CONT  TestRegistry_ParallelReads/parallel_read_344
=== CONT  TestRegistry_ParallelReads/parallel_read_352
=== CONT  TestRegistry_ParallelReads/parallel_read_195
=== CONT  TestRegistry_ParallelReads/parallel_read_290
=== CONT  TestRegistry_ParallelReads/parallel_read_184
=== CONT  TestRegistry_ParallelReads/parallel_read_281
=== CONT  TestRegistry_ParallelReads/parallel_read_317
=== CONT  TestRegistry_ParallelReads/parallel_read_240
=== CONT  TestRegistry_ParallelReads/parallel_read_245
=== CONT  TestRegistry_ParallelReads/parallel_read_265
=== CONT  TestRegistry_ParallelReads/parallel_read_187
=== CONT  TestRegistry_ParallelReads/parallel_read_182
=== CONT  TestRegistry_ParallelReads/parallel_read_242
=== CONT  TestRegistry_ParallelReads/parallel_read_318
=== CONT  TestRegistry_ParallelReads/parallel_read_437
=== CONT  TestRegistry_ParallelReads/parallel_read_310
=== CONT  TestRegistry_ParallelReads/parallel_read_291
=== CONT  TestRegistry_ParallelReads/parallel_read_183
=== CONT  TestRegistry_ParallelReads/parallel_read_285
=== CONT  TestRegistry_ParallelReads/parallel_read_264
=== CONT  TestRegistry_ParallelReads/parallel_read_288
=== CONT  TestRegistry_ParallelReads/parallel_read_181
=== CONT  TestRegistry_ParallelReads/parallel_read_339
=== CONT  TestRegistry_ParallelReads/parallel_read_261
=== CONT  TestRegistry_ParallelReads/parallel_read_343
=== CONT  TestRegistry_ParallelReads/parallel_read_178
=== CONT  TestRegistry_ParallelReads/parallel_read_267
=== CONT  TestRegistry_ParallelReads/parallel_read_304
=== CONT  TestRegistry_ParallelReads/parallel_read_176
=== CONT  TestRegistry_ParallelReads/parallel_read_174
=== CONT  TestRegistry_ParallelReads/parallel_read_681
=== CONT  TestRegistry_ParallelReads/parallel_read_163
=== CONT  TestRegistry_ParallelReads/parallel_read_345
=== CONT  TestRegistry_ParallelReads/parallel_read_34
=== CONT  TestRegistry_ParallelReads/parallel_read_260
=== CONT  TestRegistry_ParallelReads/parallel_read_671
=== CONT  TestRegistry_ParallelReads/parallel_read_341
=== CONT  TestRegistry_ParallelReads/parallel_read_31
=== CONT  TestRegistry_ParallelReads/parallel_read_119
=== CONT  TestRegistry_ParallelReads/parallel_read_316
=== CONT  TestRegistry_ParallelReads/parallel_read_333
=== CONT  TestRegistry_ParallelReads/parallel_read_157
=== CONT  TestRegistry_ParallelReads/parallel_read_167
=== CONT  TestRegistry_ParallelReads/parallel_read_162
=== CONT  TestRegistry_ParallelReads/parallel_read_164
=== CONT  TestRegistry_ParallelReads/parallel_read_161
=== CONT  TestRegistry_ParallelReads/parallel_read_49
=== CONT  TestRegistry_ParallelReads/parallel_read_66
=== CONT  TestRegistry_ParallelReads/parallel_read_47
=== CONT  TestRegistry_ParallelReads/parallel_read_21
=== CONT  TestRegistry_ParallelReads/parallel_read_110
=== CONT  TestRegistry_ParallelReads/parallel_read_151
=== CONT  TestRegistry_ParallelReads/parallel_read_111
=== CONT  TestRegistry_ParallelReads/parallel_read_71
=== CONT  TestRegistry_ParallelReads/parallel_read_118
=== CONT  TestRegistry_ParallelReads/parallel_read_70
=== CONT  TestRegistry_ParallelReads/parallel_read_149
=== CONT  TestRegistry_ParallelReads/parallel_read_24
=== CONT  TestRegistry_ParallelReads/parallel_read_106
=== CONT  TestRegistry_ParallelReads/parallel_read_51
=== CONT  TestRegistry_ParallelReads/parallel_read_154
=== CONT  TestRegistry_ParallelReads/parallel_read_153
=== CONT  TestRegistry_ParallelReads/parallel_read_38
=== CONT  TestRegistry_ParallelReads/parallel_read_16
=== CONT  TestRegistry_ParallelReads/parallel_read_11
=== CONT  TestRegistry_ParallelReads/parallel_read_37
=== CONT  TestRegistry_ParallelReads/parallel_read_52
=== CONT  TestRegistry_ParallelReads/parallel_read_7
=== CONT  TestRegistry_ParallelReads/parallel_read_60
=== CONT  TestRegistry_ParallelReads/parallel_read_35
=== CONT  TestRegistry_ParallelReads/parallel_read_58
=== CONT  TestRegistry_ParallelReads/parallel_read_17
=== CONT  TestRegistry_ParallelReads/parallel_read_57
=== CONT  TestRegistry_ParallelReads/parallel_read_145
=== CONT  TestRegistry_ParallelReads/parallel_read_148
=== CONT  TestRegistry_ParallelReads/parallel_read_147
=== CONT  TestRegistry_ParallelReads/parallel_read_88
=== CONT  TestRegistry_ParallelReads/parallel_read_2
=== CONT  TestRegistry_ParallelReads/parallel_read_84
=== CONT  TestRegistry_ParallelReads/parallel_read_739
=== CONT  TestRegistry_ParallelReads/parallel_read_5
=== CONT  TestRegistry_ParallelReads/parallel_read_82
=== CONT  TestRegistry_ParallelReads/parallel_read_3
=== CONT  TestRegistry_ParallelReads/parallel_read_101
=== CONT  TestRegistry_ParallelReads/parallel_read_233
=== CONT  TestRegistry_ParallelReads/parallel_read_90
=== CONT  TestRegistry_ParallelReads/parallel_read_138
=== CONT  TestRegistry_ParallelReads/parallel_read_248
=== CONT  TestRegistry_ParallelReads/parallel_read_77
=== CONT  TestRegistry_ParallelReads/parallel_read_133
=== CONT  TestRegistry_ParallelReads/parallel_read_335
=== CONT  TestRegistry_ParallelReads/parallel_read_320
=== CONT  TestRegistry_ParallelReads/parallel_read_122
=== CONT  TestRegistry_ParallelReads/parallel_read_813
=== CONT  TestRegistry_ParallelReads/parallel_read_129
=== CONT  TestRegistry_ParallelReads/parallel_read_551
=== CONT  TestRegistry_ParallelReads/parallel_read_536
=== CONT  TestRegistry_ParallelReads/parallel_read_127
=== CONT  TestRegistry_ParallelReads/parallel_read_558
=== CONT  TestRegistry_ParallelReads/parallel_read_137
=== CONT  TestRegistry_ParallelReads/parallel_read_562
=== CONT  TestRegistry_ParallelReads/parallel_read_86
=== CONT  TestRegistry_ParallelReads/parallel_read_981
=== CONT  TestRegistry_ParallelReads/parallel_read_599
=== CONT  TestRegistry_ParallelReads/parallel_read_499
=== CONT  TestRegistry_ParallelReads/parallel_read_590
=== CONT  TestRegistry_ParallelReads/parallel_read_575
=== CONT  TestRegistry_ParallelReads/parallel_read_128
=== CONT  TestRegistry_ParallelReads/parallel_read_540
=== CONT  TestRegistry_ParallelReads/parallel_read_653
=== CONT  TestRegistry_ParallelReads/parallel_read_41
=== CONT  TestRegistry_ParallelReads/parallel_read_30
=== CONT  TestRegistry_ParallelReads/parallel_read_250
=== CONT  TestRegistry_ParallelReads/parallel_read_527
=== CONT  TestRegistry_ParallelReads/parallel_read_56
=== CONT  TestRegistry_ParallelReads/parallel_read_835
=== CONT  TestRegistry_ParallelReads/parallel_read_591
=== CONT  TestRegistry_ParallelReads/parallel_read_504
=== CONT  TestRegistry_ParallelReads/parallel_read_332
=== CONT  TestRegistry_ParallelReads/parallel_read_577
=== CONT  TestRegistry_ParallelReads/parallel_read_439
=== CONT  TestRegistry_ParallelReads/parallel_read_818
=== CONT  TestRegistry_ParallelReads/parallel_read_737
=== CONT  TestRegistry_ParallelReads/parallel_read_150
=== CONT  TestRegistry_ParallelReads/parallel_read_55
=== CONT  TestRegistry_ParallelReads/parallel_read_862
=== CONT  TestRegistry_ParallelReads/parallel_read_336
=== CONT  TestRegistry_ParallelReads/parallel_read_891
=== CONT  TestRegistry_ParallelReads/parallel_read_270
=== CONT  TestRegistry_ParallelReads/parallel_read_348
=== CONT  TestRegistry_ParallelReads/parallel_read_936
=== CONT  TestRegistry_ParallelReads/parallel_read_911
=== CONT  TestRegistry_ParallelReads/parallel_read_657
=== CONT  TestRegistry_ParallelReads/parallel_read_520
=== CONT  TestRegistry_ParallelReads/parallel_read_112
=== CONT  TestRegistry_ParallelReads/parallel_read_975
=== CONT  TestRegistry_ParallelReads/parallel_read_892
=== CONT  TestRegistry_ParallelReads/parallel_read_770
=== CONT  TestRegistry_ParallelReads/parallel_read_116
=== CONT  TestRegistry_ParallelReads/parallel_read_910
=== CONT  TestRegistry_ParallelReads/parallel_read_701
=== CONT  TestRegistry_ParallelReads/parallel_read_427
=== CONT  TestRegistry_ParallelReads/parallel_read_643
=== CONT  TestRegistry_ParallelReads/parallel_read_749
=== CONT  TestRegistry_ParallelReads/parallel_read_985
=== CONT  TestRegistry_ParallelReads/parallel_read_871
=== CONT  TestRegistry_ParallelReads/parallel_read_976
=== CONT  TestRegistry_ParallelReads/parallel_read_832
=== CONT  TestRegistry_ParallelReads/parallel_read_989
=== CONT  TestRegistry_ParallelReads/parallel_read_632
=== CONT  TestRegistry_ParallelReads/parallel_read_966
=== CONT  TestRegistry_ParallelReads/parallel_read_823
=== CONT  TestRegistry_ParallelReads/parallel_read_626
=== CONT  TestRegistry_ParallelReads/parallel_read_852
=== CONT  TestRegistry_ParallelReads/parallel_read_866
=== CONT  TestRegistry_ParallelReads/parallel_read_923
=== CONT  TestRegistry_ParallelReads/parallel_read_672
=== CONT  TestRegistry_ParallelReads/parallel_read_789
=== CONT  TestRegistry_ParallelReads/parallel_read_668
=== CONT  TestRegistry_ParallelReads/parallel_read_758
=== CONT  TestRegistry_ParallelReads/parallel_read_847
=== CONT  TestRegistry_ParallelReads/parallel_read_914
=== CONT  TestRegistry_ParallelReads/parallel_read_686
=== CONT  TestRegistry_ParallelReads/parallel_read_713
=== CONT  TestRegistry_ParallelReads/parallel_read_772
=== CONT  TestRegistry_ParallelReads/parallel_read_654
=== CONT  TestRegistry_ParallelReads/parallel_read_724
=== CONT  TestRegistry_ParallelReads/parallel_read_816
=== CONT  TestRegistry_ParallelReads/parallel_read_800
=== CONT  TestRegistry_ParallelReads/parallel_read_850
=== CONT  TestRegistry_ParallelReads/parallel_read_797
=== CONT  TestRegistry_ParallelReads/parallel_read_971
=== CONT  TestRegistry_ParallelReads/parallel_read_742
=== CONT  TestRegistry_ParallelReads/parallel_read_706
=== CONT  TestRegistry_ParallelReads/parallel_read_622
=== CONT  TestRegistry_ParallelReads/parallel_read_628
=== CONT  TestRegistry_ParallelReads/parallel_read_685
=== CONT  TestRegistry_ParallelReads/parallel_read_973
=== CONT  TestRegistry_ParallelReads/parallel_read_786
=== CONT  TestRegistry_ParallelReads/parallel_read_750
=== CONT  TestRegistry_ParallelReads/parallel_read_621
=== CONT  TestRegistry_ParallelReads/parallel_read_885
=== CONT  TestRegistry_ParallelReads/parallel_read_633
=== CONT  TestRegistry_ParallelReads/parallel_read_718
=== CONT  TestRegistry_ParallelReads/parallel_read_893
=== CONT  TestRegistry_ParallelReads/parallel_read_929
=== CONT  TestRegistry_ParallelReads/parallel_read_970
=== CONT  TestRegistry_ParallelReads/parallel_read_615
=== CONT  TestRegistry_ParallelReads/parallel_read_837
=== CONT  TestRegistry_ParallelReads/parallel_read_982
=== CONT  TestRegistry_ParallelReads/parallel_read_747
=== CONT  TestRegistry_ParallelReads/parallel_read_920
=== CONT  TestRegistry_ParallelReads/parallel_read_651
=== CONT  TestRegistry_ParallelReads/parallel_read_726
=== CONT  TestRegistry_ParallelReads/parallel_read_994
=== CONT  TestRegistry_ParallelReads/parallel_read_774
=== CONT  TestRegistry_ParallelReads/parallel_read_708
=== CONT  TestRegistry_ParallelReads/parallel_read_649
=== CONT  TestRegistry_ParallelReads/parallel_read_735
=== CONT  TestRegistry_ParallelReads/parallel_read_680
=== CONT  TestRegistry_ParallelReads/parallel_read_894
=== CONT  TestRegistry_ParallelReads/parallel_read_676
=== CONT  TestRegistry_ParallelReads/parallel_read_664
=== CONT  TestRegistry_ParallelReads/parallel_read_782
=== CONT  TestRegistry_ParallelReads/parallel_read_630
=== CONT  TestRegistry_ParallelReads/parallel_read_854
=== CONT  TestRegistry_ParallelReads/parallel_read_711
=== CONT  TestRegistry_ParallelReads/parallel_read_888
=== CONT  TestRegistry_ParallelReads/parallel_read_731
=== CONT  TestRegistry_ParallelReads/parallel_read_719
=== CONT  TestRegistry_ParallelReads/parallel_read_661
=== CONT  TestRegistry_ParallelReads/parallel_read_963
=== CONT  TestRegistry_ParallelReads/parallel_read_868
=== CONT  TestRegistry_ParallelReads/parallel_read_808
=== CONT  TestRegistry_ParallelReads/parallel_read_730
=== CONT  TestRegistry_ParallelReads/parallel_read_958
=== CONT  TestRegistry_ParallelReads/parallel_read_853
=== CONT  TestRegistry_ParallelReads/parallel_read_785
=== CONT  TestRegistry_ParallelReads/parallel_read_637
=== CONT  TestRegistry_ParallelReads/parallel_read_793
=== CONT  TestRegistry_ParallelReads/parallel_read_798
=== CONT  TestRegistry_ParallelReads/parallel_read_890
=== CONT  TestRegistry_ParallelReads/parallel_read_848
=== CONT  TestRegistry_ParallelReads/parallel_read_614
=== CONT  TestRegistry_ParallelReads/parallel_read_991
=== CONT  TestRegistry_ParallelReads/parallel_read_678
=== CONT  TestRegistry_ParallelReads/parallel_read_629
=== CONT  TestRegistry_ParallelReads/parallel_read_810
=== CONT  TestRegistry_ParallelReads/parallel_read_950
=== CONT  TestRegistry_ParallelReads/parallel_read_741
=== CONT  TestRegistry_ParallelReads/parallel_read_865
=== CONT  TestRegistry_ParallelReads/parallel_read_972
=== CONT  TestRegistry_ParallelReads/parallel_read_815
=== CONT  TestRegistry_ParallelReads/parallel_read_821
=== CONT  TestRegistry_ParallelReads/parallel_read_935
=== CONT  TestRegistry_ParallelReads/parallel_read_948
=== CONT  TestRegistry_ParallelReads/parallel_read_732
=== CONT  TestRegistry_ParallelReads/parallel_read_689
=== CONT  TestRegistry_ParallelReads/parallel_read_812
=== CONT  TestRegistry_ParallelReads/parallel_read_645
=== CONT  TestRegistry_ParallelReads/parallel_read_999
=== CONT  TestRegistry_ParallelReads/parallel_read_795
=== CONT  TestRegistry_ParallelReads/parallel_read_899
=== CONT  TestRegistry_ParallelReads/parallel_read_820
=== CONT  TestRegistry_ParallelReads/parallel_read_968
=== CONT  TestRegistry_ParallelReads/parallel_read_753
=== CONT  TestRegistry_ParallelReads/parallel_read_921
=== CONT  TestRegistry_ParallelReads/parallel_read_635
=== CONT  TestRegistry_ParallelReads/parallel_read_705
=== CONT  TestRegistry_ParallelReads/parallel_read_663
=== CONT  TestRegistry_ParallelReads/parallel_read_980
=== CONT  TestRegistry_ParallelReads/parallel_read_646
=== CONT  TestRegistry_ParallelReads/parallel_read_780
=== CONT  TestRegistry_ParallelReads/parallel_read_806
=== CONT  TestRegistry_ParallelReads/parallel_read_962
=== CONT  TestRegistry_ParallelReads/parallel_read_881
=== CONT  TestRegistry_ParallelReads/parallel_read_956
=== CONT  TestRegistry_ParallelReads/parallel_read_704
=== CONT  TestRegistry_ParallelReads/parallel_read_748
=== CONT  TestRegistry_ParallelReads/parallel_read_761
=== CONT  TestRegistry_ParallelReads/parallel_read_773
=== CONT  TestRegistry_ParallelReads/parallel_read_930
=== CONT  TestRegistry_ParallelReads/parallel_read_787
=== CONT  TestRegistry_ParallelReads/parallel_read_995
=== CONT  TestRegistry_ParallelReads/parallel_read_744
=== CONT  TestRegistry_ParallelReads/parallel_read_723
=== CONT  TestRegistry_ParallelReads/parallel_read_900
=== CONT  TestRegistry_ParallelReads/parallel_read_895
=== CONT  TestRegistry_ParallelReads/parallel_read_880
=== CONT  TestRegistry_ParallelReads/parallel_read_738
=== CONT  TestRegistry_ParallelReads/parallel_read_639
=== CONT  TestRegistry_ParallelReads/parallel_read_682
=== CONT  TestRegistry_ParallelReads/parallel_read_642
=== CONT  TestRegistry_ParallelReads/parallel_read_996
=== CONT  TestRegistry_ParallelReads/parallel_read_656
=== CONT  TestRegistry_ParallelReads/parallel_read_714
=== CONT  TestRegistry_ParallelReads/parallel_read_841
=== CONT  TestRegistry_ParallelReads/parallel_read_225
=== CONT  TestRegistry_ParallelReads/parallel_read_658
=== CONT  TestRegistry_ParallelReads/parallel_read_675
=== CONT  TestRegistry_ParallelReads/parallel_read_952
=== CONT  TestRegistry_ParallelReads/parallel_read_974
=== CONT  TestRegistry_ParallelReads/parallel_read_809
=== CONT  TestRegistry_ParallelReads/parallel_read_683
=== CONT  TestRegistry_ParallelReads/parallel_read_916
=== CONT  TestRegistry_ParallelReads/parallel_read_945
=== CONT  TestRegistry_ParallelReads/parallel_read_702
=== CONT  TestRegistry_ParallelReads/parallel_read_902
=== CONT  TestRegistry_ParallelReads/parallel_read_903
=== CONT  TestRegistry_ParallelReads/parallel_read_864
=== CONT  TestRegistry_ParallelReads/parallel_read_826
=== CONT  TestRegistry_ParallelReads/parallel_read_679
=== CONT  TestRegistry_ParallelReads/parallel_read_997
=== CONT  TestRegistry_ParallelReads/parallel_read_931
=== CONT  TestRegistry_ParallelReads/parallel_read_861
=== CONT  TestRegistry_ParallelReads/parallel_read_226
=== CONT  TestRegistry_ParallelReads/parallel_read_955
=== CONT  TestRegistry_ParallelReads/parallel_read_712
=== CONT  TestRegistry_ParallelReads/parallel_read_650
=== CONT  TestRegistry_ParallelReads/parallel_read_223
=== CONT  TestRegistry_ParallelReads/parallel_read_620
=== CONT  TestRegistry_ParallelReads/parallel_read_222
=== CONT  TestRegistry_ParallelReads/parallel_read_644
=== CONT  TestRegistry_ParallelReads/parallel_read_624
=== CONT  TestRegistry_ParallelReads/parallel_read_655
=== CONT  TestRegistry_ParallelReads/parallel_read_218
=== CONT  TestRegistry_ParallelReads/parallel_read_707
=== CONT  TestRegistry_ParallelReads/parallel_read_752
=== CONT  TestRegistry_ParallelReads/parallel_read_219
=== CONT  TestRegistry_ParallelReads/parallel_read_636
=== CONT  TestRegistry_ParallelReads/parallel_read_216
=== CONT  TestRegistry_ParallelReads/parallel_read_886
=== CONT  TestRegistry_ParallelReads/parallel_read_703
=== CONT  TestRegistry_ParallelReads/parallel_read_907
=== CONT  TestRegistry_ParallelReads/parallel_read_725
=== CONT  TestRegistry_ParallelReads/parallel_read_912
=== CONT  TestRegistry_ParallelReads/parallel_read_716
=== CONT  TestRegistry_ParallelReads/parallel_read_214
=== CONT  TestRegistry_ParallelReads/parallel_read_623
=== CONT  TestRegistry_ParallelReads/parallel_read_220
=== CONT  TestRegistry_ParallelReads/parallel_read_755
=== CONT  TestRegistry_ParallelReads/parallel_read_627
=== CONT  TestRegistry_ParallelReads/parallel_read_805
=== CONT  TestRegistry_ParallelReads/parallel_read_631
=== CONT  TestRegistry_ParallelReads/parallel_read_926
=== CONT  TestRegistry_ParallelReads/parallel_read_217
=== CONT  TestRegistry_ParallelReads/parallel_read_846
=== CONT  TestRegistry_ParallelReads/parallel_read_652
=== CONT  TestRegistry_ParallelReads/parallel_read_763
=== CONT  TestRegistry_ParallelReads/parallel_read_215
=== CONT  TestRegistry_ParallelReads/parallel_read_927
=== CONT  TestRegistry_ParallelReads/parallel_read_919
=== CONT  TestRegistry_ParallelReads/parallel_read_765
=== CONT  TestRegistry_ParallelReads/parallel_read_932
=== CONT  TestRegistry_ParallelReads/parallel_read_796
=== CONT  TestRegistry_ParallelReads/parallel_read_988
=== CONT  TestRegistry_ParallelReads/parallel_read_667
=== CONT  TestRegistry_ParallelReads/parallel_read_843
=== CONT  TestRegistry_ParallelReads/parallel_read_660
=== CONT  TestRegistry_ParallelReads/parallel_read_904
=== CONT  TestRegistry_ParallelReads/parallel_read_917
=== CONT  TestRegistry_ParallelReads/parallel_read_610
=== CONT  TestRegistry_ParallelReads/parallel_read_951
=== CONT  TestRegistry_ParallelReads/parallel_read_529
=== CONT  TestRegistry_ParallelReads/parallel_read_612
=== CONT  TestRegistry_ParallelReads/parallel_read_819
=== CONT  TestRegistry_ParallelReads/parallel_read_978
=== CONT  TestRegistry_ParallelReads/parallel_read_827
=== CONT  TestRegistry_ParallelReads/parallel_read_523
=== CONT  TestRegistry_ParallelReads/parallel_read_928
=== CONT  TestRegistry_ParallelReads/parallel_read_690
=== CONT  TestRegistry_ParallelReads/parallel_read_760
=== CONT  TestRegistry_ParallelReads/parallel_read_877
=== CONT  TestRegistry_ParallelReads/parallel_read_967
=== CONT  TestRegistry_ParallelReads/parallel_read_979
=== CONT  TestRegistry_ParallelReads/parallel_read_887
=== CONT  TestRegistry_ParallelReads/parallel_read_833
=== CONT  TestRegistry_ParallelReads/parallel_read_807
=== CONT  TestRegistry_ParallelReads/parallel_read_595
=== CONT  TestRegistry_ParallelReads/parallel_read_759
=== CONT  TestRegistry_ParallelReads/parallel_read_764
=== CONT  TestRegistry_ParallelReads/parallel_read_876
=== CONT  TestRegistry_ParallelReads/parallel_read_913
=== CONT  TestRegistry_ParallelReads/parallel_read_496
=== CONT  TestRegistry_ParallelReads/parallel_read_569
=== CONT  TestRegistry_ParallelReads/parallel_read_552
=== CONT  TestRegistry_ParallelReads/parallel_read_756
=== CONT  TestRegistry_ParallelReads/parallel_read_960
=== CONT  TestRegistry_ParallelReads/parallel_read_503
=== CONT  TestRegistry_ParallelReads/parallel_read_842
=== CONT  TestRegistry_ParallelReads/parallel_read_987
=== CONT  TestRegistry_ParallelReads/parallel_read_539
=== CONT  TestRegistry_ParallelReads/parallel_read_784
=== CONT  TestRegistry_ParallelReads/parallel_read_512
=== CONT  TestRegistry_ParallelReads/parallel_read_998
=== CONT  TestRegistry_ParallelReads/parallel_read_834
=== CONT  TestRegistry_ParallelReads/parallel_read_560
=== CONT  TestRegistry_ParallelReads/parallel_read_804
=== CONT  TestRegistry_ParallelReads/parallel_read_586
=== CONT  TestRegistry_ParallelReads/parallel_read_491
=== CONT  TestRegistry_ParallelReads/parallel_read_611
=== CONT  TestRegistry_ParallelReads/parallel_read_603
=== CONT  TestRegistry_ParallelReads/parallel_read_580
=== CONT  TestRegistry_ParallelReads/parallel_read_497
=== CONT  TestRegistry_ParallelReads/parallel_read_924
=== CONT  TestRegistry_ParallelReads/parallel_read_543
=== CONT  TestRegistry_ParallelReads/parallel_read_515
=== CONT  TestRegistry_ParallelReads/parallel_read_401
=== CONT  TestRegistry_ParallelReads/parallel_read_859
=== CONT  TestRegistry_ParallelReads/parallel_read_519
=== CONT  TestRegistry_ParallelReads/parallel_read_489
=== CONT  TestRegistry_ParallelReads/parallel_read_869
=== CONT  TestRegistry_ParallelReads/parallel_read_490
=== CONT  TestRegistry_ParallelReads/parallel_read_498
=== CONT  TestRegistry_ParallelReads/parallel_read_555
=== CONT  TestRegistry_ParallelReads/parallel_read_857
=== CONT  TestRegistry_ParallelReads/parallel_read_376
=== CONT  TestRegistry_ParallelReads/parallel_read_570
=== CONT  TestRegistry_ParallelReads/parallel_read_453
=== CONT  TestRegistry_ParallelReads/parallel_read_507
=== CONT  TestRegistry_ParallelReads/parallel_read_788
=== CONT  TestRegistry_ParallelReads/parallel_read_510
=== CONT  TestRegistry_ParallelReads/parallel_read_360
=== CONT  TestRegistry_ParallelReads/parallel_read_579
=== CONT  TestRegistry_ParallelReads/parallel_read_915
=== CONT  TestRegistry_ParallelReads/parallel_read_436
=== CONT  TestRegistry_ParallelReads/parallel_read_546
=== CONT  TestRegistry_ParallelReads/parallel_read_581
=== CONT  TestRegistry_ParallelReads/parallel_read_584
=== CONT  TestRegistry_ParallelReads/parallel_read_417
=== CONT  TestRegistry_ParallelReads/parallel_read_578
=== CONT  TestRegistry_ParallelReads/parallel_read_379
=== CONT  TestRegistry_ParallelReads/parallel_read_918
=== CONT  TestRegistry_ParallelReads/parallel_read_589
=== CONT  TestRegistry_ParallelReads/parallel_read_367
=== CONT  TestRegistry_ParallelReads/parallel_read_394
=== CONT  TestRegistry_ParallelReads/parallel_read_550
=== CONT  TestRegistry_ParallelReads/parallel_read_879
=== CONT  TestRegistry_ParallelReads/parallel_read_957
=== CONT  TestRegistry_ParallelReads/parallel_read_362
=== CONT  TestRegistry_ParallelReads/parallel_read_544
=== CONT  TestRegistry_ParallelReads/parallel_read_596
=== CONT  TestRegistry_ParallelReads/parallel_read_516
=== CONT  TestRegistry_ParallelReads/parallel_read_416
=== CONT  TestRegistry_ParallelReads/parallel_read_757
=== CONT  TestRegistry_ParallelReads/parallel_read_909
=== CONT  TestRegistry_ParallelReads/parallel_read_566
=== CONT  TestRegistry_ParallelReads/parallel_read_598
=== CONT  TestRegistry_ParallelReads/parallel_read_860
=== CONT  TestRegistry_ParallelReads/parallel_read_867
=== CONT  TestRegistry_ParallelReads/parallel_read_488
=== CONT  TestRegistry_ParallelReads/parallel_read_447
=== CONT  TestRegistry_ParallelReads/parallel_read_574
=== CONT  TestRegistry_ParallelReads/parallel_read_587
=== CONT  TestRegistry_ParallelReads/parallel_read_534
=== CONT  TestRegistry_ParallelReads/parallel_read_452
=== CONT  TestRegistry_ParallelReads/parallel_read_383
=== CONT  TestRegistry_ParallelReads/parallel_read_925
=== CONT  TestRegistry_ParallelReads/parallel_read_467
=== CONT  TestRegistry_ParallelReads/parallel_read_535
=== CONT  TestRegistry_ParallelReads/parallel_read_940
=== CONT  TestRegistry_ParallelReads/parallel_read_435
=== CONT  TestRegistry_ParallelReads/parallel_read_572
=== CONT  TestRegistry_ParallelReads/parallel_read_934
=== CONT  TestRegistry_ParallelReads/parallel_read_602
=== CONT  TestRegistry_ParallelReads/parallel_read_531
=== CONT  TestRegistry_ParallelReads/parallel_read_532
=== CONT  TestRegistry_ParallelReads/parallel_read_944
=== CONT  TestRegistry_ParallelReads/parallel_read_593
=== CONT  TestRegistry_ParallelReads/parallel_read_601
=== CONT  TestRegistry_ParallelReads/parallel_read_553
=== CONT  TestRegistry_ParallelReads/parallel_read_454
=== CONT  TestRegistry_ParallelReads/parallel_read_517
=== CONT  TestRegistry_ParallelReads/parallel_read_533
=== CONT  TestRegistry_ParallelReads/parallel_read_831
=== CONT  TestRegistry_ParallelReads/parallel_read_509
=== CONT  TestRegistry_ParallelReads/parallel_read_406
=== CONT  TestRegistry_ParallelReads/parallel_read_573
=== CONT  TestRegistry_ParallelReads/parallel_read_571
=== CONT  TestRegistry_ParallelReads/parallel_read_897
=== CONT  TestRegistry_ParallelReads/parallel_read_549
=== CONT  TestRegistry_ParallelReads/parallel_read_567
=== CONT  TestRegistry_ParallelReads/parallel_read_428
=== CONT  TestRegistry_ParallelReads/parallel_read_547
=== CONT  TestRegistry_ParallelReads/parallel_read_576
=== CONT  TestRegistry_ParallelReads/parallel_read_365
=== CONT  TestRegistry_ParallelReads/parallel_read_500
=== CONT  TestRegistry_ParallelReads/parallel_read_592
=== CONT  TestRegistry_ParallelReads/parallel_read_554
=== CONT  TestRegistry_ParallelReads/parallel_read_609
=== CONT  TestRegistry_ParallelReads/parallel_read_556
=== CONT  TestRegistry_ParallelReads/parallel_read_410
=== CONT  TestRegistry_ParallelReads/parallel_read_548
=== CONT  TestRegistry_ParallelReads/parallel_read_522
=== CONT  TestRegistry_ParallelReads/parallel_read_418
=== CONT  TestRegistry_ParallelReads/parallel_read_524
=== CONT  TestRegistry_ParallelReads/parallel_read_378
=== CONT  TestRegistry_ParallelReads/parallel_read_594
=== CONT  TestRegistry_ParallelReads/parallel_read_485
=== CONT  TestRegistry_ParallelReads/parallel_read_356
=== CONT  TestRegistry_ParallelReads/parallel_read_366
=== CONT  TestRegistry_ParallelReads/parallel_read_502
=== CONT  TestRegistry_ParallelReads/parallel_read_608
=== CONT  TestRegistry_ParallelReads/parallel_read_613
=== CONT  TestRegistry_ParallelReads/parallel_read_472
=== CONT  TestRegistry_ParallelReads/parallel_read_582
=== CONT  TestRegistry_ParallelReads/parallel_read_446
=== CONT  TestRegistry_ParallelReads/parallel_read_588
=== CONT  TestRegistry_ParallelReads/parallel_read_481
=== CONT  TestRegistry_ParallelReads/parallel_read_506
=== CONT  TestRegistry_ParallelReads/parallel_read_392
=== CONT  TestRegistry_ParallelReads/parallel_read_460
=== CONT  TestRegistry_ParallelReads/parallel_read_487
=== CONT  TestRegistry_ParallelReads/parallel_read_443
=== CONT  TestRegistry_ParallelReads/parallel_read_545
=== CONT  TestRegistry_ParallelReads/parallel_read_563
=== CONT  TestRegistry_ParallelReads/parallel_read_434
=== CONT  TestRegistry_ParallelReads/parallel_read_409
=== CONT  TestRegistry_ParallelReads/parallel_read_374
=== CONT  TestRegistry_ParallelReads/parallel_read_521
=== CONT  TestRegistry_ParallelReads/parallel_read_438
=== CONT  TestRegistry_ParallelReads/parallel_read_479
=== CONT  TestRegistry_ParallelReads/parallel_read_482
=== CONT  TestRegistry_ParallelReads/parallel_read_370
=== CONT  TestRegistry_ParallelReads/parallel_read_476
=== CONT  TestRegistry_ParallelReads/parallel_read_404
=== CONT  TestRegistry_ParallelReads/parallel_read_448
=== CONT  TestRegistry_ParallelReads/parallel_read_227
=== CONT  TestRegistry_ParallelReads/parallel_read_494
=== CONT  TestRegistry_ParallelReads/parallel_read_414
=== CONT  TestRegistry_ParallelReads/parallel_read_421
=== CONT  TestRegistry_ParallelReads/parallel_read_830
=== CONT  TestRegistry_ParallelReads/parallel_read_457
=== CONT  TestRegistry_ParallelReads/parallel_read_440
=== CONT  TestRegistry_ParallelReads/parallel_read_475
=== CONT  TestRegistry_ParallelReads/parallel_read_618
=== CONT  TestRegistry_ParallelReads/parallel_read_397
=== CONT  TestRegistry_ParallelReads/parallel_read_419
=== CONT  TestRegistry_ParallelReads/parallel_read_767
=== CONT  TestRegistry_ParallelReads/parallel_read_387
=== CONT  TestRegistry_ParallelReads/parallel_read_779
=== CONT  TestRegistry_ParallelReads/parallel_read_855
=== CONT  TestRegistry_ParallelReads/parallel_read_388
=== CONT  TestRegistry_ParallelReads/parallel_read_845
=== CONT  TestRegistry_ParallelReads/parallel_read_583
=== CONT  TestRegistry_ParallelReads/parallel_read_802
=== CONT  TestRegistry_ParallelReads/parallel_read_803
=== CONT  TestRegistry_ParallelReads/parallel_read_801
=== CONT  TestRegistry_ParallelReads/parallel_read_825
=== CONT  TestRegistry_ParallelReads/parallel_read_542
=== CONT  TestRegistry_ParallelReads/parallel_read_766
=== CONT  TestRegistry_ParallelReads/parallel_read_840
=== CONT  TestRegistry_ParallelReads/parallel_read_413
=== CONT  TestRegistry_ParallelReads/parallel_read_817
=== CONT  TestRegistry_ParallelReads/parallel_read_514
=== CONT  TestRegistry_ParallelReads/parallel_read_513
=== CONT  TestRegistry_ParallelReads/parallel_read_811
=== CONT  TestRegistry_ParallelReads/parallel_read_541
=== CONT  TestRegistry_ParallelReads/parallel_read_839
=== CONT  TestRegistry_ParallelReads/parallel_read_559
=== CONT  TestRegistry_ParallelReads/parallel_read_473
=== CONT  TestRegistry_ParallelReads/parallel_read_769
=== CONT  TestRegistry_ParallelReads/parallel_read_411
=== CONT  TestRegistry_ParallelReads/parallel_read_403
=== CONT  TestRegistry_ParallelReads/parallel_read_844
=== CONT  TestRegistry_ParallelReads/parallel_read_585
=== CONT  TestRegistry_ParallelReads/parallel_read_607
=== CONT  TestRegistry_ParallelReads/parallel_read_398
=== CONT  TestRegistry_ParallelReads/parallel_read_471
=== CONT  TestRegistry_ParallelReads/parallel_read_511
=== CONT  TestRegistry_ParallelReads/parallel_read_386
=== CONT  TestRegistry_ParallelReads/parallel_read_538
=== CONT  TestRegistry_ParallelReads/parallel_read_458
=== CONT  TestRegistry_ParallelReads/parallel_read_361
=== CONT  TestRegistry_ParallelReads/parallel_read_402
=== CONT  TestRegistry_ParallelReads/parallel_read_799
=== CONT  TestRegistry_ParallelReads/parallel_read_259
=== CONT  TestRegistry_ParallelReads/parallel_read_430
=== CONT  TestRegistry_ParallelReads/parallel_read_442
=== CONT  TestRegistry_ParallelReads/parallel_read_463
=== CONT  TestRegistry_ParallelReads/parallel_read_495
=== CONT  TestRegistry_ParallelReads/parallel_read_295
=== CONT  TestRegistry_ParallelReads/parallel_read_381
=== CONT  TestRegistry_ParallelReads/parallel_read_330
=== CONT  TestRegistry_ParallelReads/parallel_read_372
=== CONT  TestRegistry_ParallelReads/parallel_read_505
=== CONT  TestRegistry_ParallelReads/parallel_read_477
=== CONT  TestRegistry_ParallelReads/parallel_read_715
=== CONT  TestRegistry_ParallelReads/parallel_read_324
=== CONT  TestRegistry_ParallelReads/parallel_read_600
=== CONT  TestRegistry_ParallelReads/parallel_read_433
=== CONT  TestRegistry_ParallelReads/parallel_read_300
=== CONT  TestRegistry_ParallelReads/parallel_read_518
=== CONT  TestRegistry_ParallelReads/parallel_read_470
=== CONT  TestRegistry_ParallelReads/parallel_read_468
=== CONT  TestRegistry_ParallelReads/parallel_read_354
=== CONT  TestRegistry_ParallelReads/parallel_read_604
=== CONT  TestRegistry_ParallelReads/parallel_read_97
=== CONT  TestRegistry_ParallelReads/parallel_read_408
=== CONT  TestRegistry_ParallelReads/parallel_read_256
=== CONT  TestRegistry_ParallelReads/parallel_read_357
=== CONT  TestRegistry_ParallelReads/parallel_read_237
=== CONT  TestRegistry_ParallelReads/parallel_read_483
=== CONT  TestRegistry_ParallelReads/parallel_read_526
=== CONT  TestRegistry_ParallelReads/parallel_read_469
=== CONT  TestRegistry_ParallelReads/parallel_read_371
=== CONT  TestRegistry_ParallelReads/parallel_read_568
=== CONT  TestRegistry_ParallelReads/parallel_read_462
=== CONT  TestRegistry_ParallelReads/parallel_read_384
=== CONT  TestRegistry_ParallelReads/parallel_read_606
=== CONT  TestRegistry_ParallelReads/parallel_read_431
=== CONT  TestRegistry_ParallelReads/parallel_read_605
=== CONT  TestRegistry_ParallelReads/parallel_read_293
=== CONT  TestRegistry_ParallelReads/parallel_read_246
=== CONT  TestRegistry_ParallelReads/parallel_read_375
=== CONT  TestRegistry_ParallelReads/parallel_read_303
=== CONT  TestRegistry_ParallelReads/parallel_read_235
=== CONT  TestRegistry_ParallelReads/parallel_read_390
=== CONT  TestRegistry_ParallelReads/parallel_read_493
=== CONT  TestRegistry_ParallelReads/parallel_read_368
=== CONT  TestRegistry_ParallelReads/parallel_read_309
=== CONT  TestRegistry_ParallelReads/parallel_read_478
=== CONT  TestRegistry_ParallelReads/parallel_read_474
=== CONT  TestRegistry_ParallelReads/parallel_read_359
=== CONT  TestRegistry_ParallelReads/parallel_read_213
=== CONT  TestRegistry_ParallelReads/parallel_read_353
=== CONT  TestRegistry_ParallelReads/parallel_read_561
=== CONT  TestRegistry_ParallelReads/parallel_read_426
=== CONT  TestRegistry_ParallelReads/parallel_read_287
=== CONT  TestRegistry_ParallelReads/parallel_read_508
=== CONT  TestRegistry_ParallelReads/parallel_read_207
=== CONT  TestRegistry_ParallelReads/parallel_read_425
=== CONT  TestRegistry_ParallelReads/parallel_read_206
=== CONT  TestRegistry_ParallelReads/parallel_read_444
=== CONT  TestRegistry_ParallelReads/parallel_read_423
=== CONT  TestRegistry_ParallelReads/parallel_read_641
=== CONT  TestRegistry_ParallelReads/parallel_read_451
=== CONT  TestRegistry_ParallelReads/parallel_read_391
=== CONT  TestRegistry_ParallelReads/parallel_read_441
=== CONT  TestRegistry_ParallelReads/parallel_read_314
=== CONT  TestRegistry_ParallelReads/parallel_read_209
=== CONT  TestRegistry_ParallelReads/parallel_read_208
=== CONT  TestRegistry_ParallelReads/parallel_read_400
=== CONT  TestRegistry_ParallelReads/parallel_read_445
=== CONT  TestRegistry_ParallelReads/parallel_read_322
=== CONT  TestRegistry_ParallelReads/parallel_read_377
=== CONT  TestRegistry_ParallelReads/parallel_read_203
=== CONT  TestRegistry_ParallelReads/parallel_read_373
=== CONT  TestRegistry_ParallelReads/parallel_read_266
=== CONT  TestRegistry_ParallelReads/parallel_read_325
=== CONT  TestRegistry_ParallelReads/parallel_read_253
=== CONT  TestRegistry_ParallelReads/parallel_read_199
=== CONT  TestRegistry_ParallelReads/parallel_read_228
=== CONT  TestRegistry_ParallelReads/parallel_read_198
=== CONT  TestRegistry_ParallelReads/parallel_read_299
=== CONT  TestRegistry_ParallelReads/parallel_read_307
=== CONT  TestRegistry_ParallelReads/parallel_read_484
=== CONT  TestRegistry_ParallelReads/parallel_read_346
=== CONT  TestRegistry_ParallelReads/parallel_read_247
=== CONT  TestRegistry_ParallelReads/parallel_read_321
=== CONT  TestRegistry_ParallelReads/parallel_read_200
=== CONT  TestRegistry_ParallelReads/parallel_read_405
=== CONT  TestRegistry_ParallelReads/parallel_read_301
=== CONT  TestRegistry_ParallelReads/parallel_read_455
=== CONT  TestRegistry_ParallelReads/parallel_read_313
=== CONT  TestRegistry_ParallelReads/parallel_read_252
=== CONT  TestRegistry_ParallelReads/parallel_read_315
=== CONT  TestRegistry_ParallelReads/parallel_read_276
=== CONT  TestRegistry_ParallelReads/parallel_read_239
=== CONT  TestRegistry_ParallelReads/parallel_read_193
=== CONT  TestRegistry_ParallelReads/parallel_read_340
=== CONT  TestRegistry_ParallelReads/parallel_read_196
=== CONT  TestRegistry_ParallelReads/parallel_read_292
=== CONT  TestRegistry_ParallelReads/parallel_read_194
=== CONT  TestRegistry_ParallelReads/parallel_read_262
=== CONT  TestRegistry_ParallelReads/parallel_read_234
=== CONT  TestRegistry_ParallelReads/parallel_read_286
=== CONT  TestRegistry_ParallelReads/parallel_read_229
=== CONT  TestRegistry_ParallelReads/parallel_read_269
=== CONT  TestRegistry_ParallelReads/parallel_read_192
=== CONT  TestRegistry_ParallelReads/parallel_read_347
=== CONT  TestRegistry_ParallelReads/parallel_read_311
=== CONT  TestRegistry_ParallelReads/parallel_read_351
=== CONT  TestRegistry_ParallelReads/parallel_read_283
=== CONT  TestRegistry_ParallelReads/parallel_read_275
=== CONT  TestRegistry_ParallelReads/parallel_read_342
=== CONT  TestRegistry_ParallelReads/parallel_read_289
=== CONT  TestRegistry_ParallelReads/parallel_read_294
=== CONT  TestRegistry_ParallelReads/parallel_read_189
=== CONT  TestRegistry_ParallelReads/parallel_read_243
=== CONT  TestRegistry_ParallelReads/parallel_read_297
=== CONT  TestRegistry_ParallelReads/parallel_read_186
=== CONT  TestRegistry_ParallelReads/parallel_read_231
=== CONT  TestRegistry_ParallelReads/parallel_read_306
=== CONT  TestRegistry_ParallelReads/parallel_read_326
=== CONT  TestRegistry_ParallelReads/parallel_read_236
=== CONT  TestRegistry_ParallelReads/parallel_read_185
=== CONT  TestRegistry_ParallelReads/parallel_read_191
=== CONT  TestRegistry_ParallelReads/parallel_read_268
=== CONT  TestRegistry_ParallelReads/parallel_read_254
=== CONT  TestRegistry_ParallelReads/parallel_read_249
=== CONT  TestRegistry_ParallelReads/parallel_read_305
=== CONT  TestRegistry_ParallelReads/parallel_read_331
=== CONT  TestRegistry_ParallelReads/parallel_read_350
=== CONT  TestRegistry_ParallelReads/parallel_read_282
=== CONT  TestRegistry_ParallelReads/parallel_read_238
=== CONT  TestRegistry_ParallelReads/parallel_read_279
=== CONT  TestRegistry_ParallelReads/parallel_read_337
=== CONT  TestRegistry_ParallelReads/parallel_read_298
=== CONT  TestRegistry_ParallelReads/parallel_read_329
=== CONT  TestRegistry_ParallelReads/parallel_read_173
=== CONT  TestRegistry_ParallelReads/parallel_read_180
=== CONT  TestRegistry_ParallelReads/parallel_read_296
=== CONT  TestRegistry_ParallelReads/parallel_read_273
=== CONT  TestRegistry_ParallelReads/parallel_read_308
=== CONT  TestRegistry_ParallelReads/parallel_read_172
=== CONT  TestRegistry_ParallelReads/parallel_read_179
=== CONT  TestRegistry_ParallelReads/parallel_read_177
=== CONT  TestRegistry_ParallelReads/parallel_read_328
=== CONT  TestRegistry_ParallelReads/parallel_read_175
=== CONT  TestRegistry_ParallelReads/parallel_read_171
=== CONT  TestRegistry_ParallelReads/parallel_read_80
=== CONT  TestRegistry_ParallelReads/parallel_read_319
=== CONT  TestRegistry_ParallelReads/parallel_read_168
=== CONT  TestRegistry_ParallelReads/parallel_read_166
=== CONT  TestRegistry_ParallelReads/parallel_read_169
=== CONT  TestRegistry_ParallelReads/parallel_read_165
=== CONT  TestRegistry_ParallelReads/parallel_read_74
=== CONT  TestRegistry_ParallelReads/parallel_read_121
=== CONT  TestRegistry_ParallelReads/parallel_read_33
=== CONT  TestRegistry_ParallelReads/parallel_read_338
=== CONT  TestRegistry_ParallelReads/parallel_read_170
=== CONT  TestRegistry_ParallelReads/parallel_read_312
=== CONT  TestRegistry_ParallelReads/parallel_read_32
=== CONT  TestRegistry_ParallelReads/parallel_read_75
=== CONT  TestRegistry_ParallelReads/parallel_read_674
=== CONT  TestRegistry_ParallelReads/parallel_read_76
=== CONT  TestRegistry_ParallelReads/parallel_read_159
=== CONT  TestRegistry_ParallelReads/parallel_read_29
=== CONT  TestRegistry_ParallelReads/parallel_read_73
=== CONT  TestRegistry_ParallelReads/parallel_read_729
=== CONT  TestRegistry_ParallelReads/parallel_read_160
=== CONT  TestRegistry_ParallelReads/parallel_read_72
=== CONT  TestRegistry_ParallelReads/parallel_read_740
=== CONT  TestRegistry_ParallelReads/parallel_read_27
=== CONT  TestRegistry_ParallelReads/parallel_read_114
=== CONT  TestRegistry_ParallelReads/parallel_read_158
=== CONT  TestRegistry_ParallelReads/parallel_read_25
=== CONT  TestRegistry_ParallelReads/parallel_read_67
=== CONT  TestRegistry_ParallelReads/parallel_read_26
=== CONT  TestRegistry_ParallelReads/parallel_read_700
=== CONT  TestRegistry_ParallelReads/parallel_read_50
=== CONT  TestRegistry_ParallelReads/parallel_read_48
=== CONT  TestRegistry_ParallelReads/parallel_read_113
=== CONT  TestRegistry_ParallelReads/parallel_read_68
=== CONT  TestRegistry_ParallelReads/parallel_read_117
=== CONT  TestRegistry_ParallelReads/parallel_read_65
=== CONT  TestRegistry_ParallelReads/parallel_read_156
=== CONT  TestRegistry_ParallelReads/parallel_read_619
=== CONT  TestRegistry_ParallelReads/parallel_read_152
=== CONT  TestRegistry_ParallelReads/parallel_read_23
=== CONT  TestRegistry_ParallelReads/parallel_read_63
=== CONT  TestRegistry_ParallelReads/parallel_read_28
=== CONT  TestRegistry_ParallelReads/parallel_read_46
=== CONT  TestRegistry_ParallelReads/parallel_read_115
=== CONT  TestRegistry_ParallelReads/parallel_read_155
=== CONT  TestRegistry_ParallelReads/parallel_read_18
=== CONT  TestRegistry_ParallelReads/parallel_read_22
=== CONT  TestRegistry_ParallelReads/parallel_read_69
=== CONT  TestRegistry_ParallelReads/parallel_read_109
=== CONT  TestRegistry_ParallelReads/parallel_read_43
=== CONT  TestRegistry_ParallelReads/parallel_read_107
=== CONT  TestRegistry_ParallelReads/parallel_read_64
=== CONT  TestRegistry_ParallelReads/parallel_read_15
=== CONT  TestRegistry_ParallelReads/parallel_read_108
=== CONT  TestRegistry_ParallelReads/parallel_read_42
=== CONT  TestRegistry_ParallelReads/parallel_read_39
=== CONT  TestRegistry_ParallelReads/parallel_read_105
=== CONT  TestRegistry_ParallelReads/parallel_read_62
=== CONT  TestRegistry_ParallelReads/parallel_read_44
=== CONT  TestRegistry_ParallelReads/parallel_read_14
=== CONT  TestRegistry_ParallelReads/parallel_read_61
=== CONT  TestRegistry_ParallelReads/parallel_read_20
=== CONT  TestRegistry_ParallelReads/parallel_read_36
=== CONT  TestRegistry_ParallelReads/parallel_read_19
=== CONT  TestRegistry_ParallelReads/parallel_read_45
=== CONT  TestRegistry_ParallelReads/parallel_read_13
=== CONT  TestRegistry_ParallelReads/parallel_read_40
=== CONT  TestRegistry_ParallelReads/parallel_read_9
=== CONT  TestRegistry_ParallelReads/parallel_read_12
=== CONT  TestRegistry_ParallelReads/parallel_read_59
=== CONT  TestRegistry_ParallelReads/parallel_read_54
=== CONT  TestRegistry_ParallelReads/parallel_read_10
=== CONT  TestRegistry_ParallelReads/parallel_read_89
=== CONT  TestRegistry_ParallelReads/parallel_read_85
=== CONT  TestRegistry_ParallelReads/parallel_read_144
=== CONT  TestRegistry_ParallelReads/parallel_read_103
=== CONT  TestRegistry_ParallelReads/parallel_read_53
=== CONT  TestRegistry_ParallelReads/parallel_read_6
=== CONT  TestRegistry_ParallelReads/parallel_read_104
=== CONT  TestRegistry_ParallelReads/parallel_read_146
=== CONT  TestRegistry_ParallelReads/parallel_read_99
=== CONT  TestRegistry_ParallelReads/parallel_read_4
=== CONT  TestRegistry_ParallelReads/parallel_read_98
=== CONT  TestRegistry_ParallelReads/parallel_read_96
=== CONT  TestRegistry_ParallelReads/parallel_read_87
=== CONT  TestRegistry_ParallelReads/parallel_read_95
=== CONT  TestRegistry_ParallelReads/parallel_read_8
=== CONT  TestRegistry_ParallelReads/parallel_read_100
=== CONT  TestRegistry_ParallelReads/parallel_read_94
=== CONT  TestRegistry_ParallelReads/parallel_read_83
=== CONT  TestRegistry_ParallelReads/parallel_read_1
=== CONT  TestRegistry_ParallelReads/parallel_read_143
=== CONT  TestRegistry_ParallelReads/parallel_read_278
=== CONT  TestRegistry_ParallelReads/parallel_read_102
=== CONT  TestRegistry_ParallelReads/parallel_read_142
=== CONT  TestRegistry_ParallelReads/parallel_read_81
=== CONT  TestRegistry_ParallelReads/parallel_read_140
=== CONT  TestRegistry_ParallelReads/parallel_read_91
=== CONT  TestRegistry_ParallelReads/parallel_read_274
=== CONT  TestRegistry_ParallelReads/parallel_read_141
=== CONT  TestRegistry_ParallelReads/parallel_read_139
=== CONT  TestRegistry_ParallelReads/parallel_read_126
=== CONT  TestRegistry_ParallelReads/parallel_read_79
=== CONT  TestRegistry_ParallelReads/parallel_read_134
=== CONT  TestRegistry_ParallelReads/parallel_read_131
=== CONT  TestRegistry_ParallelReads/parallel_read_93
=== CONT  TestRegistry_ParallelReads/parallel_read_78
=== CONT  TestRegistry_ParallelReads/parallel_read_92
=== CONT  TestRegistry_ParallelReads/parallel_read_125
=== CONT  TestRegistry_ParallelReads/parallel_read_241
=== CONT  TestRegistry_ParallelReads/parallel_read_123
=== CONT  TestRegistry_ParallelReads/parallel_read_132
=== CONT  TestRegistry_ParallelReads/parallel_read_135
=== CONT  TestRegistry_ParallelReads/parallel_read_492
=== CONT  TestRegistry_ParallelReads/parallel_read_717
=== CONT  TestRegistry_ParallelReads/parallel_read_120
=== CONT  TestRegistry_ParallelReads/parallel_read_136
=== CONT  TestRegistry_ParallelReads/parallel_read_124
=== CONT  TestRegistry_ParallelReads/parallel_read_530
=== CONT  TestRegistry_ParallelReads/parallel_read_501
=== CONT  TestRegistry_ParallelReads/parallel_read_597
=== CONT  TestRegistry_ParallelReads/parallel_read_130
=== CONT  TestRegistry_ParallelReads/parallel_read_525
=== CONT  TestRegistry_ParallelReads/parallel_read_355
=== CONT  TestRegistry_ParallelReads/parallel_read_777
=== CONT  TestRegistry_ParallelReads/parallel_read_528
=== CONT  TestRegistry_ParallelReads/parallel_read_565
=== CONT  TestRegistry_ParallelReads/parallel_read_537
=== CONT  TestRegistry_ParallelReads/parallel_read_693
=== CONT  TestRegistry_ParallelReads/parallel_read_564
=== CONT  TestRegistry_ParallelReads/parallel_read_557
--- PASS: TestRegistry_ParallelReads (0.08s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_965 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_898 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_990 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_878 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_937 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_984 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_851 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_986 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_822 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_964 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_939 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_949 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_884 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_943 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_754 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_953 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_961 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_762 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_993 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_905 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_794 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_959 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_828 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_870 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_684 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_776 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_836 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_673 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_743 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_908 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_938 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_720 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_648 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_792 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_790 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_771 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_734 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_640 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_751 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_783 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_638 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_906 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_856 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_791 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_677 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_688 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_954 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_992 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_942 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_977 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_838 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_901 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_710 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_745 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_883 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_727 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_933 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_697 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_0 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_616 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_829 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_874 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_721 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_863 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_696 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_922 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_781 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_946 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_849 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_666 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_896 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_665 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_694 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_698 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_969 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_625 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_662 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_733 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_722 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_983 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_768 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_691 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_728 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_687 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_736 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_941 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_889 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_695 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_224 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_647 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_746 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_775 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_778 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_659 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_699 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_947 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_692 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_617 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_814 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_873 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_221 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_858 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_709 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_211 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_882 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_327 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_875 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_395 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_824 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_634 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_244 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_210 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_251 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_459 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_358 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_212 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_323 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_382 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_872 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_415 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_486 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_334 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_280 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_407 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_363 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_369 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_422 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_385 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_456 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_669 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_464 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_465 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_389 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_380 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_466 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_272 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_393 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_450 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_396 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_432 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_399 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_461 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_284 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_364 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_204 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_277 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_202 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_424 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_412 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_258 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_420 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_302 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_232 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_480 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_349 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_197 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_670 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_429 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_205 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_271 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_201 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_190 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_230 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_449 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_188 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_255 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_257 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_263 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_344 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_352 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_195 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_290 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_184 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_281 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_317 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_240 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_245 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_265 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_187 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_182 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_242 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_318 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_437 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_310 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_291 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_183 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_264 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_285 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_288 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_181 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_339 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_261 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_343 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_178 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_267 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_304 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_176 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_174 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_681 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_163 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_345 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_34 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_260 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_671 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_341 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_31 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_119 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_316 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_333 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_157 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_167 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_162 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_164 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_161 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_49 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_66 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_21 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_47 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_110 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_151 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_111 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_71 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_118 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_70 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_149 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_24 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_106 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_51 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_154 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_153 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_38 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_16 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_11 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_37 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_52 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_7 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_60 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_35 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_58 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_17 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_57 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_145 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_148 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_147 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_88 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_2 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_84 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_739 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_5 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_82 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_3 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_101 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_233 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_90 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_248 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_138 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_77 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_133 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_335 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_320 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_122 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_813 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_129 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_551 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_536 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_127 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_558 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_137 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_562 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_86 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_981 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_599 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_499 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_590 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_575 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_128 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_540 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_653 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_41 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_30 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_250 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_527 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_56 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_835 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_591 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_504 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_332 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_577 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_439 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_818 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_737 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_150 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_55 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_862 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_336 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_891 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_270 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_348 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_936 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_911 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_657 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_520 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_112 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_975 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_892 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_770 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_116 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_910 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_701 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_427 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_643 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_749 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_985 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_871 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_976 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_832 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_989 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_632 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_966 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_823 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_626 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_852 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_866 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_923 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_672 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_789 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_758 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_668 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_847 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_914 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_686 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_713 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_772 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_654 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_724 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_816 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_800 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_850 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_797 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_971 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_742 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_706 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_622 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_628 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_685 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_973 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_786 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_750 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_621 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_885 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_633 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_837 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_982 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_718 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_747 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_893 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_651 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_726 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_994 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_929 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_970 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_708 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_774 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_615 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_649 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_735 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_920 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_680 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_894 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_676 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_664 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_782 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_630 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_854 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_711 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_888 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_731 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_661 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_868 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_808 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_730 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_719 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_963 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_853 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_958 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_785 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_637 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_793 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_798 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_890 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_848 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_614 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_991 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_678 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_629 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_810 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_950 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_741 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_865 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_815 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_935 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_972 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_821 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_948 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_732 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_689 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_812 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_645 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_999 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_795 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_899 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_820 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_968 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_921 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_753 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_635 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_705 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_663 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_980 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_646 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_780 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_806 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_962 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_881 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_956 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_704 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_748 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_995 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_787 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_895 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_930 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_761 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_744 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_723 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_682 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_642 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_639 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_900 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_773 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_996 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_738 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_656 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_880 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_714 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_225 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_841 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_658 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_675 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_952 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_974 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_809 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_683 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_916 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_945 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_702 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_902 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_903 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_864 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_826 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_679 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_997 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_931 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_861 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_226 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_955 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_712 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_650 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_223 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_620 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_222 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_644 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_624 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_655 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_707 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_218 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_752 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_219 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_636 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_216 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_886 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_703 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_725 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_907 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_716 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_214 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_912 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_623 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_220 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_755 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_627 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_805 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_631 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_926 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_217 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_846 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_763 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_927 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_919 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_652 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_215 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_765 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_932 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_796 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_988 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_667 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_843 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_660 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_904 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_610 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_917 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_951 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_529 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_612 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_819 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_978 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_827 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_523 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_928 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_760 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_690 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_877 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_967 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_979 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_887 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_833 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_807 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_595 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_759 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_764 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_876 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_913 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_496 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_569 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_552 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_756 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_960 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_503 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_842 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_987 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_539 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_784 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_512 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_998 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_834 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_560 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_804 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_586 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_491 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_611 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_603 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_580 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_497 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_924 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_543 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_515 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_401 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_859 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_519 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_489 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_869 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_490 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_498 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_555 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_857 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_376 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_570 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_453 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_507 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_510 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_788 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_360 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_579 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_915 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_436 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_546 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_581 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_584 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_417 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_578 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_379 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_918 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_394 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_589 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_550 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_367 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_879 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_362 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_957 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_544 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_596 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_516 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_416 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_757 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_909 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_566 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_598 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_860 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_867 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_488 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_447 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_574 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_587 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_534 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_452 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_383 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_925 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_467 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_940 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_535 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_435 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_572 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_934 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_531 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_532 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_602 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_944 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_593 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_601 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_553 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_454 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_517 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_533 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_831 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_509 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_406 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_571 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_573 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_897 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_549 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_567 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_428 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_547 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_576 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_365 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_500 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_592 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_554 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_609 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_556 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_410 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_548 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_522 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_418 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_524 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_378 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_594 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_485 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_356 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_366 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_502 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_608 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_613 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_472 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_582 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_446 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_588 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_481 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_506 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_392 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_460 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_487 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_443 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_545 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_563 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_434 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_409 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_374 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_521 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_438 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_479 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_482 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_370 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_476 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_404 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_448 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_227 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_494 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_414 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_421 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_830 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_457 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_440 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_475 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_618 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_397 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_419 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_767 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_387 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_779 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_855 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_388 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_845 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_583 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_802 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_803 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_801 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_825 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_542 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_766 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_840 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_413 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_817 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_514 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_513 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_811 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_541 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_839 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_559 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_473 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_769 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_411 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_403 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_844 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_585 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_607 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_398 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_471 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_511 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_386 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_538 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_458 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_361 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_402 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_799 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_259 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_430 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_442 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_463 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_495 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_295 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_381 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_330 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_372 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_505 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_477 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_715 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_324 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_600 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_433 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_300 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_518 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_470 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_468 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_354 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_604 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_97 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_408 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_256 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_357 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_237 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_483 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_526 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_469 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_371 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_568 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_462 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_384 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_606 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_431 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_605 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_293 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_246 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_375 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_303 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_235 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_390 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_493 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_368 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_309 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_478 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_474 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_359 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_213 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_353 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_561 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_426 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_287 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_508 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_207 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_425 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_206 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_444 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_423 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_641 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_451 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_391 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_441 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_314 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_209 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_208 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_400 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_445 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_322 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_377 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_203 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_373 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_266 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_325 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_253 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_199 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_228 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_198 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_299 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_307 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_484 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_346 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_247 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_321 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_200 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_405 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_301 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_455 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_313 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_252 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_315 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_276 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_239 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_193 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_340 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_196 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_292 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_194 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_262 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_234 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_286 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_229 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_269 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_347 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_192 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_311 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_351 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_283 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_275 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_342 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_289 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_294 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_189 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_243 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_297 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_186 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_231 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_306 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_326 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_236 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_185 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_191 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_268 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_254 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_249 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_305 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_331 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_350 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_282 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_238 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_279 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_337 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_298 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_329 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_173 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_180 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_296 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_273 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_308 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_172 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_179 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_177 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_328 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_175 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_171 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_80 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_319 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_168 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_166 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_169 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_165 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_74 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_121 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_33 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_338 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_170 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_312 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_75 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_32 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_674 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_76 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_159 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_29 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_73 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_729 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_160 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_72 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_740 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_27 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_114 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_158 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_25 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_67 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_26 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_700 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_50 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_48 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_113 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_68 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_117 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_65 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_156 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_619 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_152 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_23 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_63 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_28 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_46 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_115 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_155 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_18 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_22 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_69 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_109 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_43 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_107 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_64 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_15 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_108 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_42 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_39 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_105 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_62 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_44 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_14 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_61 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_20 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_36 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_19 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_45 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_13 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_40 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_9 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_12 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_59 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_54 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_10 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_89 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_85 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_103 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_144 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_53 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_6 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_104 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_146 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_99 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_4 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_98 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_96 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_87 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_95 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_8 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_100 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_94 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_83 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_143 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_1 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_278 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_102 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_142 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_81 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_140 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_91 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_274 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_141 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_139 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_126 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_79 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_134 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_131 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_93 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_78 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_92 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_125 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_241 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_123 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_132 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_135 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_492 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_717 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_120 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_136 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_124 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_530 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_501 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_597 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_130 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_525 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_355 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_777 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_528 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_565 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_537 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_693 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_564 (0.00s)
    --- PASS: TestRegistry_ParallelReads/parallel_read_557 (0.00s)
=== RUN   FuzzDispatcher_DispatchRaw
=== RUN   FuzzDispatcher_DispatchRaw/seed#0
=== RUN   FuzzDispatcher_DispatchRaw/seed#1
=== RUN   FuzzDispatcher_DispatchRaw/seed#2
2026/06/23 22:07:51 ERROR Failed to parse interaction payload operation=dispatch.parse_failed error="invalid character '}' looking for beginning of value" syntheticFailure=400
=== RUN   FuzzDispatcher_DispatchRaw/seed#3
--- PASS: FuzzDispatcher_DispatchRaw (0.00s)
    --- PASS: FuzzDispatcher_DispatchRaw/seed#0 (0.00s)
    --- PASS: FuzzDispatcher_DispatchRaw/seed#1 (0.00s)
    --- PASS: FuzzDispatcher_DispatchRaw/seed#2 (0.00s)
    --- PASS: FuzzDispatcher_DispatchRaw/seed#3 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/core	1.204s
=== RUN   TestEmbedCommands_ConcurrentMutation
=== PAUSE TestEmbedCommands_ConcurrentMutation
=== RUN   TestEmbedCommands_ObservabilityStructuralFaults
=== PAUSE TestEmbedCommands_ObservabilityStructuralFaults
=== RUN   TestEmbedCommands_RegisterCommands
=== PAUSE TestEmbedCommands_RegisterCommands
=== RUN   TestEmbedCommands_Post
=== PAUSE TestEmbedCommands_Post
=== RUN   TestEmbedCommands_Preview
=== PAUSE TestEmbedCommands_Preview
=== RUN   TestEmbedCommands_Set
=== PAUSE TestEmbedCommands_Set
=== RUN   TestEmbedCommands_Delete
=== PAUSE TestEmbedCommands_Delete
=== RUN   TestEmbedCommands_List
=== PAUSE TestEmbedCommands_List
=== RUN   TestEmbedCommands_Refresh
=== PAUSE TestEmbedCommands_Refresh
=== RUN   TestEmbedCommands_Unpost
=== PAUSE TestEmbedCommands_Unpost
=== RUN   TestEmbedCommands_Fields
=== PAUSE TestEmbedCommands_Fields
=== RUN   TestEmbedCommands_ImportExport
=== PAUSE TestEmbedCommands_ImportExport
=== RUN   TestEmbedCommands_ErrorAndEdgeCases
=== PAUSE TestEmbedCommands_ErrorAndEdgeCases
=== CONT  TestEmbedCommands_ConcurrentMutation
=== CONT  TestEmbedCommands_Refresh
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestEmbedCommands_Fields
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestEmbedCommands_Post
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestEmbedCommands_Unpost
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestEmbedCommands_RegisterCommands
2026/06/23 22:07:51 INFO Architectural state transition: Primary routines initialization component=EmbedCommands
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestEmbedCommands_RegisterCommands (0.00s)
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestEmbedCommands_Delete
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
=== CONT  TestEmbedCommands_ObservabilityStructuralFaults
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO Architectural state transition: Primary routines initialization component=EmbedCommands
2026/06/23 22:07:51 INFO Architectural state transition: Registering native command command_name=embed
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestEmbedCommands_Preview
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestEmbedCommands_Preview (0.00s)
=== CONT  TestEmbedCommands_Set
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
--- PASS: TestEmbedCommands_ObservabilityStructuralFaults (0.00s)
=== CONT  TestEmbedCommands_ErrorAndEdgeCases
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestEmbedCommands_Set (0.00s)
=== CONT  TestEmbedCommands_List
--- PASS: TestEmbedCommands_Delete (0.00s)
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestEmbedCommands_ImportExport
--- PASS: TestEmbedCommands_Unpost (0.01s)
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:51 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:51 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestEmbedCommands_Refresh (0.01s)
--- PASS: TestEmbedCommands_Post (0.01s)
--- PASS: TestEmbedCommands_Fields (0.01s)
--- PASS: TestEmbedCommands_List (0.00s)
--- PASS: TestEmbedCommands_ErrorAndEdgeCases (0.00s)
--- PASS: TestEmbedCommands_ImportExport (0.00s)
--- PASS: TestEmbedCommands_ConcurrentMutation (0.03s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/embeds	1.194s
=== RUN   TestLoggingCommands_RegisterCommands
=== PAUSE TestLoggingCommands_RegisterCommands
=== RUN   TestLoggingRootCommand_HandleSafety
=== PAUSE TestLoggingRootCommand_HandleSafety
=== RUN   TestLoggingRootCommand_Avatar
=== PAUSE TestLoggingRootCommand_Avatar
=== RUN   TestLoggingRootCommand_RoleUpdate
=== PAUSE TestLoggingRootCommand_RoleUpdate
=== RUN   TestLoggingRootCommand_Messages
=== PAUSE TestLoggingRootCommand_Messages
=== RUN   TestLoggingRootCommand_EntryExit
=== PAUSE TestLoggingRootCommand_EntryExit
=== RUN   TestLoggingRootCommand_Warnings
=== PAUSE TestLoggingRootCommand_Warnings
=== RUN   TestLoggingRootCommand_AutomodNoRule
=== PAUSE TestLoggingRootCommand_AutomodNoRule
=== RUN   TestLoggingRootCommand_AutomodWithRule
=== PAUSE TestLoggingRootCommand_AutomodWithRule
=== CONT  TestLoggingRootCommand_HandleSafety
=== CONT  TestLoggingRootCommand_Messages
=== CONT  TestLoggingRootCommand_AutomodNoRule
--- PASS: TestLoggingRootCommand_HandleSafety (0.00s)
=== CONT  TestLoggingRootCommand_RoleUpdate
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestLoggingRootCommand_EntryExit
=== CONT  TestLoggingRootCommand_AutomodWithRule
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=33333
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestLoggingRootCommand_Warnings
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=22222
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=44444
=== CONT  TestLoggingRootCommand_Avatar
=== CONT  TestLoggingCommands_RegisterCommands
--- PASS: TestLoggingCommands_RegisterCommands (0.00s)
--- PASS: TestLoggingRootCommand_Messages (0.00s)
--- PASS: TestLoggingRootCommand_RoleUpdate (0.00s)
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=55555
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=77777
--- PASS: TestLoggingRootCommand_EntryExit (0.00s)
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=66666
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=11111
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=77777
2026/06/23 22:07:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestLoggingRootCommand_AutomodNoRule (0.01s)
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=77777
--- PASS: TestLoggingRootCommand_Avatar (0.00s)
--- PASS: TestLoggingRootCommand_Warnings (0.00s)
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=77777
2026/06/23 22:07:52 INFO Operational telemetry: Logging channel updated channel_id=77777
--- PASS: TestLoggingRootCommand_AutomodWithRule (0.01s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/logging	1.488s
=== RUN   TestCommands_StatelessExecution
=== PAUSE TestCommands_StatelessExecution
=== RUN   TestMassBanCommand_Parity
=== PAUSE TestMassBanCommand_Parity
=== RUN   TestReactionBlockCommand_Parity
=== PAUSE TestReactionBlockCommand_Parity
=== CONT  TestCommands_StatelessExecution
--- PASS: TestCommands_StatelessExecution (0.00s)
=== CONT  TestMassBanCommand_Parity
--- PASS: TestMassBanCommand_Parity (0.00s)
=== CONT  TestReactionBlockCommand_Parity
--- PASS: TestReactionBlockCommand_Parity (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation	1.453s
=== RUN   TestPartnerCommands_ConcurrentStateMutation
=== PAUSE TestPartnerCommands_ConcurrentStateMutation
=== RUN   TestPartnerAddSubCommand
=== PAUSE TestPartnerAddSubCommand
=== RUN   TestPartnerRemoveSubCommand
=== PAUSE TestPartnerRemoveSubCommand
=== RUN   TestPartnerLinkSubCommand
=== PAUSE TestPartnerLinkSubCommand
=== RUN   TestPartnerRenameSubCommand
=== PAUSE TestPartnerRenameSubCommand
=== RUN   TestPartnerListSubCommand
=== PAUSE TestPartnerListSubCommand
=== RUN   TestPartnerPostSubCommand
=== PAUSE TestPartnerPostSubCommand
=== RUN   TestPartnerUnpostSubCommand
=== PAUSE TestPartnerUnpostSubCommand
=== RUN   TestPartnerRefreshSubCommand
=== PAUSE TestPartnerRefreshSubCommand
=== RUN   TestPartnerTemplates
=== PAUSE TestPartnerTemplates
=== CONT  TestPartnerCommands_ConcurrentStateMutation
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestPartnerRefreshSubCommand
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestPartnerUnpostSubCommand
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestPartnerTemplates
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
=== CONT  TestPartnerPostSubCommand
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestPartnerRenameSubCommand
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
=== CONT  TestPartnerLinkSubCommand
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
=== CONT  TestPartnerRemoveSubCommand
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:55 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:55 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerRenameSubCommand (0.10s)
=== CONT  TestPartnerListSubCommand
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerUnpostSubCommand (0.10s)
=== CONT  TestPartnerAddSubCommand
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerRemoveSubCommand (0.10s)
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerRefreshSubCommand (0.11s)
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerLinkSubCommand (0.10s)
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerAddSubCommand (0.00s)
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestPartnerListSubCommand (0.00s)
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerTemplates (0.11s)
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:56 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:56 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path="Fake IO Intercepted Store"
--- PASS: TestPartnerCommands_ConcurrentStateMutation (2.18s)
--- PASS: TestPartnerPostSubCommand (2.26s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/partners	3.555s
=== RUN   TestCommandHandler_ThunderingHerds
=== PAUSE TestCommandHandler_ThunderingHerds
=== RUN   TestCommandHandler_PanicRecovery
=== PAUSE TestCommandHandler_PanicRecovery
=== CONT  TestCommandHandler_ThunderingHerds
=== CONT  TestCommandHandler_PanicRecovery
--- PASS: TestCommandHandler_PanicRecovery (0.00s)
--- PASS: TestCommandHandler_ThunderingHerds (0.20s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd	1.320s
=== RUN   TestRolePanelCommands_Registration
2026/06/23 22:07:58 INFO Architectural state transition: Primary routines initialization component=RolePanelCommands
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=roles
--- PASS: TestRolePanelCommands_Registration (0.00s)
=== RUN   TestRolePanelCommands_ConvertPanelToArikawa
--- PASS: TestRolePanelCommands_ConvertPanelToArikawa (0.00s)
=== RUN   TestRolePanelCommands_SubCommands
=== RUN   TestRolePanelCommands_SubCommands/post
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/preview
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/set
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/delete
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/list
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/placeholders
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/buttons
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_SubCommands/fields
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestRolePanelCommands_SubCommands (0.02s)
    --- PASS: TestRolePanelCommands_SubCommands/post (0.01s)
    --- PASS: TestRolePanelCommands_SubCommands/preview (0.00s)
    --- PASS: TestRolePanelCommands_SubCommands/set (0.00s)
    --- PASS: TestRolePanelCommands_SubCommands/delete (0.00s)
    --- PASS: TestRolePanelCommands_SubCommands/list (0.00s)
    --- PASS: TestRolePanelCommands_SubCommands/placeholders (0.00s)
    --- PASS: TestRolePanelCommands_SubCommands/buttons (0.00s)
    --- PASS: TestRolePanelCommands_SubCommands/fields (0.00s)
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/disabled_feature
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/post_without_buttons
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/webhook_url_unsupported
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/non-existent_panel_on_set
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/non-existent_panel_on_delete
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/empty_panel_key
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/missing_button_options
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/missing_button_remove_options
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/list_empty_buttons_panel
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/list_empty_panels_list
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/respondStructuralError
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/refreshRolePanelPostingsBestEffort_nil_safety
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/post_failure
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/delete_with_postings_success
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/delete_with_postings_sync_failure
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/button_add_limit_reached
=== RUN   TestRolePanelCommands_ErrorsAndEdgeCases/button_remove_non-existent
--- PASS: TestRolePanelCommands_ErrorsAndEdgeCases (0.09s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/disabled_feature (0.02s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/post_without_buttons (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/webhook_url_unsupported (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/non-existent_panel_on_set (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/non-existent_panel_on_delete (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/empty_panel_key (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/missing_button_options (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/missing_button_remove_options (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/list_empty_buttons_panel (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/list_empty_panels_list (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/respondStructuralError (0.02s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/refreshRolePanelPostingsBestEffort_nil_safety (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/post_failure (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/delete_with_postings_success (0.02s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/delete_with_postings_sync_failure (0.00s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/button_add_limit_reached (0.02s)
    --- PASS: TestRolePanelCommands_ErrorsAndEdgeCases/button_remove_non-existent (0.00s)
=== RUN   TestRolePanelComponentHandler_InjectionAndRouting
=== PAUSE TestRolePanelComponentHandler_InjectionAndRouting
=== RUN   TestBuildRolePanelToggleResponseArikawa_VisibilityFlags
=== PAUSE TestBuildRolePanelToggleResponseArikawa_VisibilityFlags
=== RUN   TestConstants
=== PAUSE TestConstants
=== RUN   TestParseRolePanelButtonEmoji
=== PAUSE TestParseRolePanelButtonEmoji
=== CONT  TestRolePanelComponentHandler_InjectionAndRouting
=== RUN   TestRolePanelComponentHandler_InjectionAndRouting/valid_assignment
=== CONT  TestParseRolePanelButtonEmoji
=== RUN   TestParseRolePanelButtonEmoji/empty_returns_blanks
=== PAUSE TestParseRolePanelButtonEmoji/empty_returns_blanks
=== CONT  TestConstants
--- PASS: TestConstants (0.00s)
=== CONT  TestBuildRolePanelToggleResponseArikawa_VisibilityFlags
=== RUN   TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_Context_forces_Ephemeral_fallback
=== PAUSE TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_Context_forces_Ephemeral_fallback
=== RUN   TestParseRolePanelButtonEmoji/trims_whitespace
=== RUN   TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_GuildConfig_forces_Ephemeral_fallback
=== PAUSE TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_GuildConfig_forces_Ephemeral_fallback
=== RUN   TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_false_(Default_Ephemeral)
=== PAUSE TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_false_(Default_Ephemeral)
=== RUN   TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_true_(Public_Response)
=== PAUSE TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_true_(Public_Response)
=== RUN   TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/State_Isolation:_Global_config_does_not_leak_into_missing_GuildConfig
=== PAUSE TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/State_Isolation:_Global_config_does_not_leak_into_missing_GuildConfig
=== CONT  TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_Context_forces_Ephemeral_fallback
=== CONT  TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/State_Isolation:_Global_config_does_not_leak_into_missing_GuildConfig
=== CONT  TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_false_(Default_Ephemeral)
=== CONT  TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_true_(Public_Response)
=== CONT  TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_GuildConfig_forces_Ephemeral_fallback
--- PASS: TestBuildRolePanelToggleResponseArikawa_VisibilityFlags (0.00s)
    --- PASS: TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_Context_forces_Ephemeral_fallback (0.00s)
    --- PASS: TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/State_Isolation:_Global_config_does_not_leak_into_missing_GuildConfig (0.00s)
    --- PASS: TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_false_(Default_Ephemeral) (0.00s)
    --- PASS: TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Feature:_DisableInteractiveEphemeral_is_true_(Public_Response) (0.00s)
    --- PASS: TestBuildRolePanelToggleResponseArikawa_VisibilityFlags/Degradation:_Nil_GuildConfig_forces_Ephemeral_fallback (0.00s)
=== PAUSE TestParseRolePanelButtonEmoji/trims_whitespace
=== RUN   TestParseRolePanelButtonEmoji/unicode_glyph
=== PAUSE TestParseRolePanelButtonEmoji/unicode_glyph
=== RUN   TestParseRolePanelButtonEmoji/custom_static_emoji
=== PAUSE TestParseRolePanelButtonEmoji/custom_static_emoji
=== RUN   TestParseRolePanelButtonEmoji/custom_animated_emoji
=== PAUSE TestParseRolePanelButtonEmoji/custom_animated_emoji
=== RUN   TestParseRolePanelButtonEmoji/malformed_bracketed_input
=== PAUSE TestParseRolePanelButtonEmoji/malformed_bracketed_input
=== CONT  TestParseRolePanelButtonEmoji/trims_whitespace
=== CONT  TestParseRolePanelButtonEmoji/malformed_bracketed_input
=== CONT  TestParseRolePanelButtonEmoji/custom_animated_emoji
=== CONT  TestParseRolePanelButtonEmoji/custom_static_emoji
=== CONT  TestParseRolePanelButtonEmoji/unicode_glyph
=== CONT  TestParseRolePanelButtonEmoji/empty_returns_blanks
--- PASS: TestParseRolePanelButtonEmoji (0.00s)
    --- PASS: TestParseRolePanelButtonEmoji/trims_whitespace (0.00s)
    --- PASS: TestParseRolePanelButtonEmoji/malformed_bracketed_input (0.00s)
    --- PASS: TestParseRolePanelButtonEmoji/custom_animated_emoji (0.00s)
    --- PASS: TestParseRolePanelButtonEmoji/custom_static_emoji (0.00s)
    --- PASS: TestParseRolePanelButtonEmoji/unicode_glyph (0.00s)
    --- PASS: TestParseRolePanelButtonEmoji/empty_returns_blanks (0.00s)
=== RUN   TestRolePanelComponentHandler_InjectionAndRouting/valid_removal
=== RUN   TestRolePanelComponentHandler_InjectionAndRouting/malformed_custom_id
=== RUN   TestRolePanelComponentHandler_InjectionAndRouting/unknown_role_(not_in_config)
=== RUN   TestRolePanelComponentHandler_InjectionAndRouting/lookup_error
--- PASS: TestRolePanelComponentHandler_InjectionAndRouting (1.01s)
    --- PASS: TestRolePanelComponentHandler_InjectionAndRouting/valid_assignment (0.52s)
    --- PASS: TestRolePanelComponentHandler_InjectionAndRouting/valid_removal (0.16s)
    --- PASS: TestRolePanelComponentHandler_InjectionAndRouting/malformed_custom_id (0.00s)
    --- PASS: TestRolePanelComponentHandler_InjectionAndRouting/unknown_role_(not_in_config) (0.16s)
    --- PASS: TestRolePanelComponentHandler_InjectionAndRouting/lookup_error (0.17s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/roles	2.561s
=== RUN   TestHandler_HandleSlash_EphemeralValidation
=== PAUSE TestHandler_HandleSlash_EphemeralValidation
=== RUN   TestSaveRuntimeConfig_RaceDetection
=== PAUSE TestSaveRuntimeConfig_RaceDetection
=== RUN   TestEncodeDecodeState
=== PAUSE TestEncodeDecodeState
=== RUN   TestRuntimeInteractionAuthToken
=== PAUSE TestRuntimeInteractionAuthToken
=== RUN   TestFieldsForLines_BoundaryLimits
=== PAUSE TestFieldsForLines_BoundaryLimits
=== RUN   TestFieldsForLines_MultibyteSanity
=== PAUSE TestFieldsForLines_MultibyteSanity
=== CONT  TestSaveRuntimeConfig_RaceDetection
=== CONT  TestEncodeDecodeState
--- PASS: TestEncodeDecodeState (0.00s)
=== CONT  TestHandler_HandleSlash_EphemeralValidation
=== CONT  TestFieldsForLines_MultibyteSanity
--- PASS: TestFieldsForLines_MultibyteSanity (0.00s)
=== CONT  TestFieldsForLines_BoundaryLimits
=== RUN   TestFieldsForLines_BoundaryLimits/Empty_input_should_fallback_safely
=== CONT  TestRuntimeInteractionAuthToken
--- PASS: TestRuntimeInteractionAuthToken (0.00s)
=== RUN   TestFieldsForLines_BoundaryLimits/Exact_1024_bytes_fits_cleanly_into_one_field
=== RUN   TestFieldsForLines_BoundaryLimits/1025_bytes_partitions_into_two_fields
=== RUN   TestFieldsForLines_BoundaryLimits/Multibyte_UTF-8_boundary_slicing_does_not_fragment_runes
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
--- PASS: TestFieldsForLines_BoundaryLimits (0.00s)
    --- PASS: TestFieldsForLines_BoundaryLimits/Empty_input_should_fallback_safely (0.00s)
    --- PASS: TestFieldsForLines_BoundaryLimits/Exact_1024_bytes_fits_cleanly_into_one_field (0.00s)
    --- PASS: TestFieldsForLines_BoundaryLimits/1025_bytes_partitions_into_two_fields (0.00s)
    --- PASS: TestFieldsForLines_BoundaryLimits/Multibyte_UTF-8_boundary_slicing_does_not_fragment_runes (0.00s)
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Interaction routed to runtime configuration slash command guild_id="" request_id=12345
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestHandler_HandleSlash_EphemeralValidation (0.00s)
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:07:58 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestSaveRuntimeConfig_RaceDetection (0.03s)
=== RUN   FuzzDecodeState
=== RUN   FuzzDecodeState/seed#0
=== RUN   FuzzDecodeState/seed#1
=== RUN   FuzzDecodeState/seed#2
=== RUN   FuzzDecodeState/seed#3
--- PASS: FuzzDecodeState (0.00s)
    --- PASS: FuzzDecodeState/seed#0 (0.00s)
    --- PASS: FuzzDecodeState/seed#1 (0.00s)
    --- PASS: FuzzDecodeState/seed#2 (0.00s)
    --- PASS: FuzzDecodeState/seed#3 (0.00s)
=== RUN   FuzzDecodeRuntimeModalState
=== RUN   FuzzDecodeRuntimeModalState/seed#0
=== RUN   FuzzDecodeRuntimeModalState/seed#1
=== RUN   FuzzDecodeRuntimeModalState/seed#2
=== RUN   FuzzDecodeRuntimeModalState/seed#3
--- PASS: FuzzDecodeRuntimeModalState (0.00s)
    --- PASS: FuzzDecodeRuntimeModalState/seed#0 (0.00s)
    --- PASS: FuzzDecodeRuntimeModalState/seed#1 (0.00s)
    --- PASS: FuzzDecodeRuntimeModalState/seed#2 (0.00s)
    --- PASS: FuzzDecodeRuntimeModalState/seed#3 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime	1.463s
=== RUN   TestStatsAddPersistsChannelConfig
=== PAUSE TestStatsAddPersistsChannelConfig
=== RUN   TestStatsAddUpdatesExistingChannelConfig
=== PAUSE TestStatsAddUpdatesExistingChannelConfig
=== RUN   TestStatsAddWithRoleFilter
=== PAUSE TestStatsAddWithRoleFilter
=== RUN   TestStatsRemoveDeletesChannelConfig
=== PAUSE TestStatsRemoveDeletesChannelConfig
=== RUN   TestStatsRemoveReportsErrorForUnknownChannel
=== PAUSE TestStatsRemoveReportsErrorForUnknownChannel
=== RUN   TestStatsListShowsConfiguredChannels
=== PAUSE TestStatsListShowsConfiguredChannels
=== RUN   TestStatsListShowsEmptyStateWhenNoChannels
=== PAUSE TestStatsListShowsEmptyStateWhenNoChannels
=== RUN   TestStatsListShowsRoleFilter
=== PAUSE TestStatsListShowsRoleFilter
=== CONT  TestStatsAddWithRoleFilter
=== CONT  TestStatsListShowsConfiguredChannels
=== CONT  TestStatsAddUpdatesExistingChannelConfig
=== CONT  TestStatsAddPersistsChannelConfig
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestStatsRemoveDeletesChannelConfig
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
=== CONT  TestStatsRemoveReportsErrorForUnknownChannel
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
=== CONT  TestStatsListShowsEmptyStateWhenNoChannels
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
=== CONT  TestStatsListShowsRoleFilter
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO Architectural state transition: Registering native command command_name=stats
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:07:58 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:07:58 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestStatsRemoveReportsErrorForUnknownChannel (0.00s)
--- PASS: TestStatsRemoveDeletesChannelConfig (0.00s)
--- PASS: TestStatsListShowsRoleFilter (0.00s)
--- PASS: TestStatsAddUpdatesExistingChannelConfig (0.00s)
--- PASS: TestStatsListShowsConfiguredChannels (0.00s)
--- PASS: TestStatsAddWithRoleFilter (0.00s)
--- PASS: TestStatsAddPersistsChannelConfig (0.00s)
--- PASS: TestStatsListShowsEmptyStateWhenNoChannels (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/stats	1.482s
=== RUN   TestRouter_DeferBeforeIO
=== PAUSE TestRouter_DeferBeforeIO
=== CONT  TestRouter_DeferBeforeIO
--- PASS: TestRouter_DeferBeforeIO (0.01s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/commands/tickets	1.387s
=== RUN   TestRenderCustomEmbed
=== PAUSE TestRenderCustomEmbed
=== RUN   TestCustomEmbedPostingSyncer
=== PAUSE TestCustomEmbedPostingSyncer
=== CONT  TestRenderCustomEmbed
=== CONT  TestCustomEmbedPostingSyncer
--- PASS: TestRenderCustomEmbed (0.00s)
2026/06/23 22:08:00 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:00 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:00 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:00 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:00 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:00 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:00 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:00 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:00 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestCustomEmbedPostingSyncer (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/embeds	1.776s
?   	github.com/small-frappuccino/discordcore/pkg/discord/logging	[no test files]
=== RUN   TestGatewayListener_Lifecycle
=== PAUSE TestGatewayListener_Lifecycle
=== CONT  TestGatewayListener_Lifecycle
--- PASS: TestGatewayListener_Lifecycle (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/members	1.755s
=== RUN   TestGatewayListener_Lifecycle
=== PAUSE TestGatewayListener_Lifecycle
=== CONT  TestGatewayListener_Lifecycle
--- PASS: TestGatewayListener_Lifecycle (0.25s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/messages	2.057s
=== RUN   TestFallbackCache_ResolveMember
=== PAUSE TestFallbackCache_ResolveMember
=== RUN   TestBuildModerationEmbed_Golden
=== PAUSE TestBuildModerationEmbed_Golden
=== RUN   TestService_ContextTimeout
=== PAUSE TestService_ContextTimeout
=== RUN   TestService_ExponentialBackoff
=== PAUSE TestService_ExponentialBackoff
=== CONT  TestFallbackCache_ResolveMember
2026/06/23 22:08:08 WARN Mitigated service degradation: Target absent from local cache; executing REST fallback guild_id=123 target_id=456
--- PASS: TestFallbackCache_ResolveMember (0.00s)
=== CONT  TestService_ContextTimeout
--- PASS: TestService_ContextTimeout (0.00s)
=== CONT  TestService_ExponentialBackoff
--- PASS: TestService_ExponentialBackoff (0.00s)
=== CONT  TestBuildModerationEmbed_Golden
--- PASS: TestBuildModerationEmbed_Golden (0.01s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/moderation	1.622s
=== RUN   TestPartnerService_Render
=== PAUSE TestPartnerService_Render
=== CONT  TestPartnerService_Render
--- PASS: TestPartnerService_Render (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/partners	2.641s
?   	github.com/small-frappuccino/discordcore/pkg/discord/perf	[no test files]
=== RUN   TestArikawaPublisher_Errors
=== PAUSE TestArikawaPublisher_Errors
=== RUN   TestRuntimeService_GracefulShutdown
=== PAUSE TestRuntimeService_GracefulShutdown
=== CONT  TestRuntimeService_GracefulShutdown
=== CONT  TestArikawaPublisher_Errors
=== RUN   TestArikawaPublisher_Errors/404_Unknown_Channel
--- PASS: TestRuntimeService_GracefulShutdown (0.00s)
=== RUN   TestArikawaPublisher_Errors/403_Missing_Access
=== RUN   TestArikawaPublisher_Errors/429_Too_Many_Requests
--- PASS: TestArikawaPublisher_Errors (0.02s)
    --- PASS: TestArikawaPublisher_Errors/404_Unknown_Channel (0.01s)
    --- PASS: TestArikawaPublisher_Errors/403_Missing_Access (0.00s)
    --- PASS: TestArikawaPublisher_Errors/429_Too_Many_Requests (0.01s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/qotd	2.630s
=== RUN   TestRolePanelSyncEditsEachPosting
=== PAUSE TestRolePanelSyncEditsEachPosting
=== RUN   TestRolePanelSyncDropsMissingPostings
=== PAUSE TestRolePanelSyncDropsMissingPostings
=== CONT  TestRolePanelSyncEditsEachPosting
=== CONT  TestRolePanelSyncDropsMissingPostings
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestRolePanelSyncEditsEachPosting (0.02s)
2026/06/23 22:08:16 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:16 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestRolePanelSyncDropsMissingPostings (0.02s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/roles	5.420s
=== RUN   TestNewDiscordSessionEmptyToken
2026/06/23 22:08:16 ERROR Discord bot token is empty. Please set the token before starting the bot.
--- PASS: TestNewDiscordSessionEmptyToken (0.00s)
=== RUN   TestNewDiscordSessionCreateError
2026/06/23 22:08:16 INFO Creating Discord session (token redacted)
2026/06/23 22:08:16 ERROR Failed to create Discord session: boom
--- PASS: TestNewDiscordSessionCreateError (0.00s)
=== RUN   TestNewDiscordSessionSuccess
2026/06/23 22:08:16 INFO Creating Discord session (token redacted)
2026/06/23 22:08:16 INFO Discord session created successfully
--- PASS: TestNewDiscordSessionSuccess (0.00s)
=== RUN   TestNewDiscordSessionWithIntentsUsesProvidedMask
2026/06/23 22:08:16 INFO Creating Discord session (token redacted)
2026/06/23 22:08:16 INFO Discord session created successfully
--- PASS: TestNewDiscordSessionWithIntentsUsesProvidedMask (0.00s)
=== RUN   TestOpenSession
=== RUN   TestOpenSession/Success_path
=== RUN   TestOpenSession/OpenSession_failure
=== RUN   TestOpenSession/Context_timeout/cancellation
=== RUN   TestOpenSession/Nil_session
--- PASS: TestOpenSession (0.00s)
    --- PASS: TestOpenSession/Success_path (0.00s)
    --- PASS: TestOpenSession/OpenSession_failure (0.00s)
    --- PASS: TestOpenSession/Context_timeout/cancellation (0.00s)
    --- PASS: TestOpenSession/Nil_session (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/session	4.089s
=== RUN   TestArikawaGateway
=== PAUSE TestArikawaGateway
=== RUN   TestRegisterArikawaEventHandlers
=== PAUSE TestRegisterArikawaEventHandlers
=== RUN   TestHandleArikawaGuildMemberAdd
=== PAUSE TestHandleArikawaGuildMemberAdd
=== RUN   TestHandleArikawaGuildMemberRemove
=== PAUSE TestHandleArikawaGuildMemberRemove
=== RUN   TestHandleArikawaGuildMemberUpdate
=== PAUSE TestHandleArikawaGuildMemberUpdate
=== RUN   TestRegisterDiscordGoEventHandlers
=== PAUSE TestRegisterDiscordGoEventHandlers
=== RUN   TestHandleDiscordGoGuildMemberAdd
=== PAUSE TestHandleDiscordGoGuildMemberAdd
=== RUN   TestHandleDiscordGoGuildMemberRemove
=== PAUSE TestHandleDiscordGoGuildMemberRemove
=== RUN   TestHandleDiscordGoGuildMemberUpdate
=== PAUSE TestHandleDiscordGoGuildMemberUpdate
=== CONT  TestHandleArikawaGuildMemberRemove
    events_arikawa_test.go:142: skipping db tests
=== CONT  TestHandleArikawaGuildMemberUpdate
=== CONT  TestArikawaGateway
=== RUN   TestArikawaGateway/UpdateChannelName
=== CONT  TestHandleDiscordGoGuildMemberUpdate
    events_discordgo_test.go:100: skipping db tests
=== CONT  TestHandleDiscordGoGuildMemberRemove
    events_discordgo_test.go:67: skipping db tests
=== CONT  TestRegisterDiscordGoEventHandlers
2026/06/23 22:08:17 INFO Registered DiscordGo event handlers for stats
--- SKIP: TestHandleArikawaGuildMemberRemove (0.00s)
=== NAME  TestHandleArikawaGuildMemberUpdate
    events_arikawa_test.go:173: skipping db tests
--- SKIP: TestHandleArikawaGuildMemberUpdate (0.00s)
=== CONT  TestHandleDiscordGoGuildMemberAdd
    events_discordgo_test.go:30: skipping db tests
--- SKIP: TestHandleDiscordGoGuildMemberAdd (0.00s)
--- SKIP: TestHandleDiscordGoGuildMemberUpdate (0.00s)
=== CONT  TestHandleArikawaGuildMemberAdd
    events_arikawa_test.go:103: skipping db tests
--- SKIP: TestHandleArikawaGuildMemberAdd (0.00s)
=== CONT  TestRegisterArikawaEventHandlers
2026/06/23 22:08:17 INFO Registered Arikawa event handlers for stats
--- PASS: TestRegisterArikawaEventHandlers (0.00s)
--- SKIP: TestHandleDiscordGoGuildMemberRemove (0.00s)
--- PASS: TestRegisterDiscordGoEventHandlers (0.00s)
=== RUN   TestArikawaGateway/GetChannel
=== RUN   TestArikawaGateway/StreamGuildMembers
=== RUN   TestArikawaGateway/StreamGuildMembers_ContextCancel
--- PASS: TestArikawaGateway (0.01s)
    --- PASS: TestArikawaGateway/UpdateChannelName (0.00s)
    --- PASS: TestArikawaGateway/GetChannel (0.00s)
    --- PASS: TestArikawaGateway/StreamGuildMembers (0.00s)
    --- PASS: TestArikawaGateway/StreamGuildMembers_ContextCancel (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/stats	1.699s
=== RUN   TestService_GenerateAndUploadTranscript_Success
=== PAUSE TestService_GenerateAndUploadTranscript_Success
=== RUN   TestService_GenerateAndUploadTranscript_Deadlock
=== PAUSE TestService_GenerateAndUploadTranscript_Deadlock
=== CONT  TestService_GenerateAndUploadTranscript_Deadlock
=== CONT  TestService_GenerateAndUploadTranscript_Success
--- PASS: TestService_GenerateAndUploadTranscript_Deadlock (0.07s)
--- PASS: TestService_GenerateAndUploadTranscript_Success (0.10s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/tickets	1.497s
=== RUN   TestValidateMessageTarget_NetworkLifecycle
=== PAUSE TestValidateMessageTarget_NetworkLifecycle
=== RUN   TestValidateMessageTarget_ErrorAssertions
=== PAUSE TestValidateMessageTarget_ErrorAssertions
=== RUN   TestDecodeEmbeds_Fuzzing
=== PAUSE TestDecodeEmbeds_Fuzzing
=== RUN   TestArikawaAPI_ServerInjection_TableDriven
=== PAUSE TestArikawaAPI_ServerInjection_TableDriven
=== RUN   TestWebhookConcurrentExecution
=== PAUSE TestWebhookConcurrentExecution
=== CONT  TestWebhookConcurrentExecution
=== CONT  TestDecodeEmbeds_Fuzzing
2026/06/23 22:08:18 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=10
2026/06/23 22:08:18 INFO Architectural state transition: Background worker pool initialized parallelism_limit=4 queue_capacity=20
2026/06/23 22:08:18 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=10
=== CONT  TestValidateMessageTarget_ErrorAssertions
=== RUN   TestValidateMessageTarget_ErrorAssertions/Auth_Denied_401
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
=== CONT  TestValidateMessageTarget_NetworkLifecycle
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
=== RUN   TestValidateMessageTarget_ErrorAssertions/Not_Found_404
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
=== CONT  TestArikawaAPI_ServerInjection_TableDriven
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
--- PASS: TestDecodeEmbeds_Fuzzing (0.00s)
--- PASS: TestValidateMessageTarget_NetworkLifecycle (0.00s)
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
=== RUN   TestValidateMessageTarget_ErrorAssertions/Rate_Limited_429
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
--- PASS: TestValidateMessageTarget_ErrorAssertions (0.00s)
    --- PASS: TestValidateMessageTarget_ErrorAssertions/Auth_Denied_401 (0.00s)
    --- PASS: TestValidateMessageTarget_ErrorAssertions/Not_Found_404 (0.00s)
    --- PASS: TestValidateMessageTarget_ErrorAssertions/Rate_Limited_429 (0.00s)
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
=== RUN   TestArikawaAPI_ServerInjection_TableDriven/Valid_Target
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Baseline operational telemetry: Webhook message target successfully validated message_id=456 webhook_id=123
2026/06/23 22:08:18 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestWebhookConcurrentExecution (0.01s)
=== RUN   TestArikawaAPI_ServerInjection_TableDriven/Invalid_Webhook_ID
=== RUN   TestArikawaAPI_ServerInjection_TableDriven/Invalid_Message_ID
--- PASS: TestArikawaAPI_ServerInjection_TableDriven (0.01s)
    --- PASS: TestArikawaAPI_ServerInjection_TableDriven/Valid_Target (0.01s)
    --- PASS: TestArikawaAPI_ServerInjection_TableDriven/Invalid_Webhook_ID (0.00s)
    --- PASS: TestArikawaAPI_ServerInjection_TableDriven/Invalid_Message_ID (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/discord/webhook	1.260s
=== RUN   TestNormalizeAutoAssignmentRoleOrderBackfillsBoosterRole
=== PAUSE TestNormalizeAutoAssignmentRoleOrderBackfillsBoosterRole
=== RUN   TestValidateBotConfigRejectsAutoAssignmentOrderMismatch
=== PAUSE TestValidateBotConfigRejectsAutoAssignmentOrderMismatch
=== RUN   TestValidateBotConfigRejectsInvalidRequiredRolesLength
=== PAUSE TestValidateBotConfigRejectsInvalidRequiredRolesLength
=== RUN   TestConfigManagerLoadConfigMigratesAutoAssignmentBoosterRole
=== PAUSE TestConfigManagerLoadConfigMigratesAutoAssignmentBoosterRole
=== RUN   TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder
=== PAUSE TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder
=== RUN   TestChannelsConfigUnmarshalStrictSchema
=== PAUSE TestChannelsConfigUnmarshalStrictSchema
=== RUN   TestChannelsConfigHelpersStrict
=== PAUSE TestChannelsConfigHelpersStrict
=== RUN   TestGuildConfigIndexHit
=== PAUSE TestGuildConfigIndexHit
=== RUN   TestGuildConfigIndexMiss
=== PAUSE TestGuildConfigIndexMiss
=== RUN   TestGuildConfigIndexUpdate
=== PAUSE TestGuildConfigIndexUpdate
=== RUN   TestSnapshotConfigReturnsDefensiveCopy
=== PAUSE TestSnapshotConfigReturnsDefensiveCopy
=== RUN   TestPublishedConfigReadsReuseSnapshot
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
--- PASS: TestPublishedConfigReadsReuseSnapshot (0.07s)
=== RUN   TestGuildConfigIndexDuplicateFix
=== PAUSE TestGuildConfigIndexDuplicateFix
=== RUN   TestGuildConfigIndexDedupePersistsOnLoad
=== PAUSE TestGuildConfigIndexDedupePersistsOnLoad
=== RUN   TestGuildConfigIndexConcurrency
=== PAUSE TestGuildConfigIndexConcurrency
=== RUN   TestCloneFeatureTogglesRoundtripAllTrue
=== PAUSE TestCloneFeatureTogglesRoundtripAllTrue
=== RUN   TestCloneFeatureTogglesRoundtripAllFalse
=== PAUSE TestCloneFeatureTogglesRoundtripAllFalse
=== RUN   TestCloneFeatureTogglesRoundtripAllNil
=== PAUSE TestCloneFeatureTogglesRoundtripAllNil
=== RUN   TestCloneFeatureTogglesIsolatesMutation
=== PAUSE TestCloneFeatureTogglesIsolatesMutation
=== RUN   TestMemoryConfigStoreRoundTrip
=== PAUSE TestMemoryConfigStoreRoundTrip
=== RUN   TestMemoryConfigStoreReturnsDefensiveCopies
=== PAUSE TestMemoryConfigStoreReturnsDefensiveCopies
=== RUN   TestPostgresConfigStoreSaveLoadRoundTrip
=== PAUSE TestPostgresConfigStoreSaveLoadRoundTrip
=== RUN   TestUpdateRuntimeConfigRollsBackOnSaveError
=== PAUSE TestUpdateRuntimeConfigRollsBackOnSaveError
=== RUN   TestCreateWebhookEmbedUpdateRollsBackOnSaveError
=== PAUSE TestCreateWebhookEmbedUpdateRollsBackOnSaveError
=== RUN   TestEncryptionSymmetric
=== PAUSE TestEncryptionSymmetric
=== RUN   TestEncryptedStringJSON
=== PAUSE TestEncryptedStringJSON
=== RUN   TestEncryptedStringUnmarshalFallback
=== PAUSE TestEncryptedStringUnmarshalFallback
=== RUN   TestLoadEnvWithLocalBinFallbackUsesHomeFile
--- PASS: TestLoadEnvWithLocalBinFallbackUsesHomeFile (0.03s)
=== RUN   TestEnvHelpers
--- PASS: TestEnvHelpers (0.00s)
=== RUN   TestFeatureRegistryIDsAreUnique
=== PAUSE TestFeatureRegistryIDsAreUnique
=== RUN   TestFeatureRegistryDefaultsMatchLegacyResolveFeatures
=== PAUSE TestFeatureRegistryDefaultsMatchLegacyResolveFeatures
=== RUN   TestLookupToggleRoundTrip
=== PAUSE TestLookupToggleRoundTrip
=== RUN   TestHasAnyOverrideDetectsEachToggle
=== PAUSE TestHasAnyOverrideDetectsEachToggle
=== RUN   TestFeatureTogglesJSONRoundTrip
=== PAUSE TestFeatureTogglesJSONRoundTrip
=== RUN   TestNewMinimalGuildConfigDisablesAllFeatures
=== PAUSE TestNewMinimalGuildConfigDisablesAllFeatures
=== RUN   TestEnsureMinimalGuildConfigPersistsDormantGuild
=== PAUSE TestEnsureMinimalGuildConfigPersistsDormantGuild
=== RUN   TestEnsureMinimalGuildConfigPreservesDomainOverridesOnExistingGuild
=== PAUSE TestEnsureMinimalGuildConfigPreservesDomainOverridesOnExistingGuild
=== RUN   TestEnsureMinimalGuildConfigPersistsDormantGuildToPostgres
=== PAUSE TestEnsureMinimalGuildConfigPersistsDormantGuildToPostgres
=== RUN   TestResolveFeaturesDefaultsModerationCommandsEnabledWhenUnset
=== PAUSE TestResolveFeaturesDefaultsModerationCommandsEnabledWhenUnset
=== RUN   TestJSONManagerSaveWritesAtomically
=== PAUSE TestJSONManagerSaveWritesAtomically
=== RUN   TestGuildConfigLegacyMigration
=== PAUSE TestGuildConfigLegacyMigration
=== RUN   TestBotConfigRoundTripDropsLegacyModerationFields
=== PAUSE TestBotConfigRoundTripDropsLegacyModerationFields
=== RUN   TestNotifySubscribers_ConcurrencyLimitExceeded
=== PAUSE TestNotifySubscribers_ConcurrencyLimitExceeded
=== RUN   TestNotifySubscribers_PanicRecovery
=== PAUSE TestNotifySubscribers_PanicRecovery
=== RUN   TestNotifySubscribers_ContextTimeoutPreemption
=== PAUSE TestNotifySubscribers_ContextTimeoutPreemption
=== RUN   TestPartnersCRUDAndDeterministicOrder
=== PAUSE TestPartnersCRUDAndDeterministicOrder
=== RUN   TestPartnersValidationAndDedup
=== PAUSE TestPartnersValidationAndDedup
=== RUN   TestPartnersUpdateDeleteNotFound
=== PAUSE TestPartnersUpdateDeleteNotFound
=== RUN   TestPlatformPathsWindows
--- PASS: TestPlatformPathsWindows (0.00s)
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue
=== RUN   TestNormalizeQOTDConfigPreservesSelectionStrategy
=== PAUSE TestNormalizeQOTDConfigPreservesSelectionStrategy
=== RUN   TestNormalizeQOTDConfigDropsUnknownSelectionStrategy
=== PAUSE TestNormalizeQOTDConfigDropsUnknownSelectionStrategy
=== RUN   TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled
=== PAUSE TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled
=== RUN   TestNormalizeQOTDConfigRequiresScheduleWhenEnabled
=== PAUSE TestNormalizeQOTDConfigRequiresScheduleWhenEnabled
=== RUN   TestNormalizeQOTDConfigAllowsPartialScheduleWhileDisabled
=== PAUSE TestNormalizeQOTDConfigAllowsPartialScheduleWhileDisabled
=== RUN   TestNormalizeQOTDConfigNormalizesSuppressedScheduledPublishDate
=== PAUSE TestNormalizeQOTDConfigNormalizesSuppressedScheduledPublishDate
=== RUN   TestNormalizeQOTDConfigDedupesAndSortsSuppressionList
=== PAUSE TestNormalizeQOTDConfigDedupesAndSortsSuppressionList
=== RUN   TestQOTDConfigUnmarshalMigratesLegacySingleSuppressionField
=== PAUSE TestQOTDConfigUnmarshalMigratesLegacySingleSuppressionField
=== RUN   TestQOTDConfigUnmarshalPrefersNewListWhenBothFieldsPresent
=== PAUSE TestQOTDConfigUnmarshalPrefersNewListWhenBothFieldsPresent
=== RUN   TestNormalizeQOTDConfigRejectsInvalidSuppressedScheduledPublishDate
=== PAUSE TestNormalizeQOTDConfigRejectsInvalidSuppressedScheduledPublishDate
=== RUN   TestSetQOTDConfigCanonicalizesMessageChannelFields
=== PAUSE TestSetQOTDConfigCanonicalizesMessageChannelFields
=== RUN   TestSetQOTDConfigRollsBackOnSaveError
=== PAUSE TestSetQOTDConfigRollsBackOnSaveError
=== RUN   TestQOTDConfigUnmarshalMigratesLegacyChannelFields
=== PAUSE TestQOTDConfigUnmarshalMigratesLegacyChannelFields
=== RUN   TestQOTDConfigLegacyJSONTableMappings
=== PAUSE TestQOTDConfigLegacyJSONTableMappings
=== RUN   TestNormalizeReactionBlockConfigMergesPairsAndDedupesEmojis
=== PAUSE TestNormalizeReactionBlockConfigMergesPairsAndDedupesEmojis
=== RUN   TestSetReactionBlockConfigCanonicalizesAndReadsBack
=== PAUSE TestSetReactionBlockConfigCanonicalizesAndReadsBack
=== RUN   TestSetReactionBlockConfigRollsBackOnSaveError
=== PAUSE TestSetReactionBlockConfigRollsBackOnSaveError
=== RUN   TestRolePanelKeyValidation
=== PAUSE TestRolePanelKeyValidation
=== RUN   TestRolePanelButtonValidation
=== PAUSE TestRolePanelButtonValidation
=== RUN   TestRolePanelEmbedFieldValidation
=== PAUSE TestRolePanelEmbedFieldValidation
=== RUN   TestRolePanelFieldCRUD
=== PAUSE TestRolePanelFieldCRUD
=== RUN   TestRolePanelCRUDLifecycle
=== PAUSE TestRolePanelCRUDLifecycle
=== RUN   TestRolePanelButtonByRoleIDFindsAcrossPanels
=== PAUSE TestRolePanelButtonByRoleIDFindsAcrossPanels
=== RUN   TestRolePanelButtonCapEnforced
=== PAUSE TestRolePanelButtonCapEnforced
=== RUN   TestRolePanelMutationsAreSnapshotIsolated
=== PAUSE TestRolePanelMutationsAreSnapshotIsolated
=== RUN   TestRolePanelPostingValidation
=== PAUSE TestRolePanelPostingValidation
=== RUN   TestRolePanelPostingsCRUD
=== PAUSE TestRolePanelPostingsCRUD
=== RUN   TestFindRolePanelPostingSearchesAcrossPanels
=== PAUSE TestFindRolePanelPostingSearchesAcrossPanels
=== RUN   TestRuntimeConfigModerationLoggingEnabled
=== PAUSE TestRuntimeConfigModerationLoggingEnabled
=== RUN   TestRuntimeConfigUnmarshalMigratesLegacyModerationLogMode
=== PAUSE TestRuntimeConfigUnmarshalMigratesLegacyModerationLogMode
=== RUN   TestResolveRuntimeConfigModerationLoggingMerge
=== PAUSE TestResolveRuntimeConfigModerationLoggingMerge
=== RUN   TestResolveRuntimeConfigGlobalMaxWorkersMerge
=== PAUSE TestResolveRuntimeConfigGlobalMaxWorkersMerge
=== RUN   TestRuntimeConfigNormalizedWebhookEmbedUpdates
=== PAUSE TestRuntimeConfigNormalizedWebhookEmbedUpdates
=== RUN   TestRuntimeConfigUnmarshalMigratesLegacyWebhookEmbedUpdate
=== PAUSE TestRuntimeConfigUnmarshalMigratesLegacyWebhookEmbedUpdate
=== RUN   TestResolveRuntimeConfigWebhookEmbedUpdatesMerge
=== PAUSE TestResolveRuntimeConfigWebhookEmbedUpdatesMerge
=== RUN   TestRuntimeConfigWebhookEmbedValidationDefaultsAndNormalization
=== PAUSE TestRuntimeConfigWebhookEmbedValidationDefaultsAndNormalization
=== RUN   TestResolveRuntimeConfigWebhookEmbedValidationMerge
=== PAUSE TestResolveRuntimeConfigWebhookEmbedValidationMerge
=== RUN   TestNormalizeRuntimeConfigRejectsNegativeGlobalMaxWorkers
=== PAUSE TestNormalizeRuntimeConfigRejectsNegativeGlobalMaxWorkers
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField
=== PAUSE TestResolveRuntimeConfigAdoptsEveryGuildField
=== RUN   TestWebhookEmbedUpdatesCRUDGlobal
=== PAUSE TestWebhookEmbedUpdatesCRUDGlobal
=== RUN   TestWebhookEmbedUpdatesCRUDGuildScope
=== PAUSE TestWebhookEmbedUpdatesCRUDGuildScope
=== RUN   TestWebhookEmbedUpdatesCreateValidationAndDuplicates
=== PAUSE TestWebhookEmbedUpdatesCreateValidationAndDuplicates
=== RUN   TestWebhookEmbedUpdatesLegacyFallbackMigration
=== PAUSE TestWebhookEmbedUpdatesLegacyFallbackMigration
=== RUN   TestWebhookEmbedUpdatesUpdateDeleteNotFound
=== PAUSE TestWebhookEmbedUpdatesUpdateDeleteNotFound
=== RUN   TestIsValidationErrorMatchesWrappedValidation
=== PAUSE TestIsValidationErrorMatchesWrappedValidation
=== CONT  TestNormalizeAutoAssignmentRoleOrderBackfillsBoosterRole
=== CONT  TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled
--- PASS: TestNormalizeAutoAssignmentRoleOrderBackfillsBoosterRole (0.00s)
=== CONT  TestPartnersUpdateDeleteNotFound
--- PASS: TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled (0.00s)
=== CONT  TestRolePanelButtonByRoleIDFindsAcrossPanels
=== CONT  TestWebhookEmbedUpdatesCRUDGuildScope
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestSetQOTDConfigCanonicalizesMessageChannelFields
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestRolePanelButtonCapEnforced
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestPartnersUpdateDeleteNotFound (0.00s)
=== CONT  TestQOTDConfigUnmarshalMigratesLegacyChannelFields
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
--- PASS: TestWebhookEmbedUpdatesCRUDGuildScope (0.00s)
=== CONT  TestQOTDConfigUnmarshalPrefersNewListWhenBothFieldsPresent
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=transactional-test-store
=== CONT  TestWebhookEmbedUpdatesUpdateDeleteNotFound
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestQOTDConfigUnmarshalPrefersNewListWhenBothFieldsPresent (0.00s)
=== CONT  TestNormalizeQOTDConfigDedupesAndSortsSuppressionList
=== CONT  TestNormalizeQOTDConfigRejectsInvalidSuppressedScheduledPublishDate
=== CONT  TestWebhookEmbedUpdatesLegacyFallbackMigration
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestNormalizeQOTDConfigAllowsPartialScheduleWhileDisabled
=== CONT  TestWebhookEmbedUpdatesCreateValidationAndDuplicates
=== CONT  TestNormalizeQOTDConfigRequiresScheduleWhenEnabled
=== CONT  TestNormalizeQOTDConfigPreservesSelectionStrategy
=== CONT  TestSetReactionBlockConfigRollsBackOnSaveError
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestRolePanelButtonByRoleIDFindsAcrossPanels (0.00s)
--- PASS: TestSetQOTDConfigCanonicalizesMessageChannelFields (0.00s)
=== CONT  TestNormalizeQOTDConfigDropsUnknownSelectionStrategy
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestNormalizeQOTDConfigDropsUnknownSelectionStrategy (0.00s)
=== CONT  TestNormalizeQOTDConfigNormalizesSuppressedScheduledPublishDate
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
--- PASS: TestNormalizeQOTDConfigNormalizesSuppressedScheduledPublishDate (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestQOTDConfigUnmarshalMigratesLegacyChannelFields (0.00s)
=== CONT  TestRolePanelEmbedFieldValidation
--- PASS: TestRolePanelEmbedFieldValidation (0.00s)
=== CONT  TestRolePanelButtonValidation
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestWebhookEmbedUpdatesCreateValidationAndDuplicates (0.00s)
=== RUN   TestRolePanelButtonValidation/valid_custom_emoji
=== CONT  TestRolePanelFieldCRUD
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestSetReactionBlockConfigCanonicalizesAndReadsBack
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestSetReactionBlockConfigRollsBackOnSaveError (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestRolePanelKeyValidation
=== RUN   TestRolePanelKeyValidation/trims_and_lowercases
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestRolePanelKeyValidation/trims_and_lowercases
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=transactional-test-store
=== RUN   TestRolePanelKeyValidation/keeps_digits_and_dashes
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestRolePanelKeyValidation/keeps_digits_and_dashes
=== RUN   TestRolePanelKeyValidation/rejects_empty
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestRolePanelKeyValidation/rejects_empty
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelKeyValidation/rejects_whitespace_inside
=== PAUSE TestRolePanelKeyValidation/rejects_whitespace_inside
=== RUN   TestRolePanelKeyValidation/rejects_punctuation
--- PASS: TestSetReactionBlockConfigCanonicalizesAndReadsBack (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestRolePanelButtonValidation/valid_custom_emoji
=== RUN   TestRolePanelButtonValidation/valid_unicode_emoji
=== CONT  TestQOTDConfigUnmarshalMigratesLegacySingleSuppressionField
=== CONT  TestNormalizeReactionBlockConfigMergesPairsAndDedupesEmojis
=== PAUSE TestRolePanelButtonValidation/valid_unicode_emoji
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelButtonValidation/valid_no_emoji
=== PAUSE TestRolePanelButtonValidation/valid_no_emoji
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestResolveRuntimeConfigGlobalMaxWorkersMerge
=== CONT  TestResolveRuntimeConfigAdoptsEveryGuildField
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.Driver
=== CONT  TestWebhookEmbedUpdatesCRUDGlobal
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelButtonValidation/missing_role
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestNormalizeQOTDConfigRejectsInvalidSuppressedScheduledPublishDate (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestNormalizeQOTDConfigDedupesAndSortsSuppressionList (0.00s)
=== CONT  TestRolePanelCRUDLifecycle
=== PAUSE TestRolePanelKeyValidation/rejects_punctuation
=== PAUSE TestRolePanelButtonValidation/missing_role
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
=== CONT  TestNormalizeRuntimeConfigRejectsNegativeGlobalMaxWorkers
--- PASS: TestWebhookEmbedUpdatesUpdateDeleteNotFound (0.00s)
=== RUN   TestRolePanelButtonValidation/non-numeric_role
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestNormalizeQOTDConfigAllowsPartialScheduleWhileDisabled (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestNormalizeQOTDConfigRequiresScheduleWhenEnabled (0.00s)
=== CONT  TestIsValidationErrorMatchesWrappedValidation
=== CONT  TestResolveRuntimeConfigWebhookEmbedValidationMerge
=== PAUSE TestRolePanelButtonValidation/non-numeric_role
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestRuntimeConfigUnmarshalMigratesLegacyWebhookEmbedUpdate
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
=== RUN   TestRolePanelButtonValidation/missing_label
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestRuntimeConfigWebhookEmbedValidationDefaultsAndNormalization
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestQOTDConfigLegacyJSONTableMappings
=== CONT  TestResolveRuntimeConfigWebhookEmbedUpdatesMerge
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestUpdateRuntimeConfigRollsBackOnSaveError
=== CONT  TestPartnersValidationAndDedup
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== RUN   TestRolePanelKeyValidation/rejects_unicode_letters
--- PASS: TestNormalizeQOTDConfigPreservesSelectionStrategy (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=0
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestRolePanelButtonValidation/missing_label
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
=== RUN   TestRolePanelButtonValidation/emoji_id_without_name
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestQOTDConfigLegacyJSONTableMappings/legacy_question_channel_id_maps_to_default_deck_channel_id
--- PASS: TestNormalizeReactionBlockConfigMergesPairsAndDedupesEmojis (0.00s)
=== PAUSE TestQOTDConfigLegacyJSONTableMappings/legacy_question_channel_id_maps_to_default_deck_channel_id
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
=== RUN   TestQOTDConfigLegacyJSONTableMappings/legacy_forum_channel_id_maps_to_default_deck_forum_channel_id
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestQOTDConfigLegacyJSONTableMappings/legacy_forum_channel_id_maps_to_default_deck_forum_channel_id
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestQOTDConfigLegacyJSONTableMappings/legacy_qotd_time_hour_utc_and_minute_maps_to_Schedule
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 ERROR Mutational failure in runtime configuration request_id=c6660311c86cd1060ddee7518db98970 synthetic_code=500 stack_trace="goroutine 31 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/files.EmitBlockingError(0xc000048230, {0x140ab3f6d, 0x2b}, {0x140ad9c00, 0xc0000824e0}, {0xc00018a020, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/files/preferences.go:34 +0x11e\ngithub.com/small-frappuccino/discordcore/pkg/files.(*ConfigManager).UpdateRuntimeConfig(_, _)\n\tD:/Users/alice/git/discordcore/pkg/files/preferences.go:223 +0x385\ngithub.com/small-frappuccino/discordcore/pkg/files.TestUpdateRuntimeConfigRollsBackOnSaveError(0xc0001ff200)\n\tD:/Users/alice/git/discordcore/pkg/files/config_transaction_test.go:67 +0x16f\ntesting.tRunner(0xc0001ff200, 0x140ad19d8)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="ConfigManager.UpdateRuntimeConfig: ConfigManager.UpdateConfig: save configuration for transactional-test-store: save failed"
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestRolePanelKeyValidation/rejects_unicode_letters
=== PAUSE TestRolePanelButtonValidation/emoji_id_without_name
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestQOTDConfigUnmarshalMigratesLegacySingleSuppressionField (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestResolveRuntimeConfigGlobalMaxWorkersMerge (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestRolePanelFieldCRUD (0.00s)
=== PAUSE TestQOTDConfigLegacyJSONTableMappings/legacy_qotd_time_hour_utc_and_minute_maps_to_Schedule
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestNotifySubscribers_ContextTimeoutPreemption
--- PASS: TestNormalizeRuntimeConfigRejectsNegativeGlobalMaxWorkers (0.00s)
=== CONT  TestNotifySubscribers_PanicRecovery
=== CONT  TestPartnersCRUDAndDeterministicOrder
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.DatabaseURL
=== CONT  TestNotifySubscribers_ConcurrencyLimitExceeded
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
=== CONT  TestBotConfigRoundTripDropsLegacyModerationFields
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== RUN   TestQOTDConfigLegacyJSONTableMappings/legacy_publish_hour_utc_and_minute_maps_to_Schedule
--- PASS: TestWebhookEmbedUpdatesCRUDGlobal (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestResolveRuntimeConfigWebhookEmbedValidationMerge (0.00s)
=== PAUSE TestQOTDConfigLegacyJSONTableMappings/legacy_publish_hour_utc_and_minute_maps_to_Schedule
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelButtonValidation/label_too_long
--- PASS: TestIsValidationErrorMatchesWrappedValidation (0.00s)
=== CONT  TestGuildConfigLegacyMigration
=== PAUSE TestRolePanelButtonValidation/label_too_long
=== RUN   TestGuildConfigLegacyMigration/migrates_bot_instance_id
--- PASS: TestRuntimeConfigUnmarshalMigratesLegacyWebhookEmbedUpdate (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestQOTDConfigLegacyJSONTableMappings/legacy_suppress_scheduled_publish_date_utc_maps_to_list
=== PAUSE TestQOTDConfigLegacyJSONTableMappings/legacy_suppress_scheduled_publish_date_utc_maps_to_list
=== RUN   TestQOTDConfigLegacyJSONTableMappings/canonical_schedule_shadows_legacy
=== CONT  TestJSONManagerSaveWritesAtomically
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestGuildConfigLegacyMigration/migrates_bot_instance_id
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestRuntimeConfigWebhookEmbedValidationDefaultsAndNormalization (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestWebhookEmbedUpdatesLegacyFallbackMigration (0.01s)
--- PASS: TestResolveRuntimeConfigWebhookEmbedUpdatesMerge (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestUpdateRuntimeConfigRollsBackOnSaveError (0.00s)
--- PASS: TestRolePanelCRUDLifecycle (0.00s)
=== CONT  TestResolveFeaturesDefaultsModerationCommandsEnabledWhenUnset
=== RUN   TestGuildConfigLegacyMigration/migrates_domain_bot_instance_ids
--- PASS: TestNotifySubscribers_PanicRecovery (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestPartnersValidationAndDedup (0.00s)
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestQOTDConfigLegacyJSONTableMappings/canonical_schedule_shadows_legacy
=== PAUSE TestGuildConfigLegacyMigration/migrates_domain_bot_instance_ids
=== CONT  TestEnsureMinimalGuildConfigPreservesDomainOverridesOnExistingGuild
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestNotifySubscribers_ConcurrencyLimitExceeded (0.00s)
=== RUN   TestGuildConfigLegacyMigration/combines_both_legacy_fields
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.MaxOpenConns
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestGuildConfigLegacyMigration/combines_both_legacy_fields
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestBotConfigRoundTripDropsLegacyModerationFields (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/empty_falls_back_to_queue
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestGuildConfigLegacyMigration/normalizes_legacy_names
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/empty_falls_back_to_queue
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestResolveFeaturesDefaultsModerationCommandsEnabledWhenUnset (0.00s)
=== PAUSE TestGuildConfigLegacyMigration/normalizes_legacy_names
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/explicit_queue_stays_queue
=== RUN   TestGuildConfigLegacyMigration/ignores_empty_fields
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/explicit_queue_stays_queue
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.MaxIdleConns
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestGuildConfigLegacyMigration/ignores_empty_fields
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/random_is_honored
=== RUN   TestGuildConfigLegacyMigration/does_not_overwrite_existing_canonical_tokens
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/random_is_honored
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.ConnMaxLifetimeSecs
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestGuildConfigLegacyMigration/does_not_overwrite_existing_canonical_tokens
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/case-insensitive_random
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestEnsureMinimalGuildConfigPersistsDormantGuild
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/case-insensitive_random
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.ConnMaxIdleTimeSecs
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestEnsureMinimalGuildConfigPersistsDormantGuildToPostgres
=== CONT  TestFeatureTogglesJSONRoundTrip
2026/06/23 22:08:19 INFO Architectural state transition: Dormant guild node appended to global configuration tree guild_id=guild-new
--- PASS: TestEnsureMinimalGuildConfigPreservesDomainOverridesOnExistingGuild (0.00s)
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/whitespace_tolerated
=== NAME  TestEnsureMinimalGuildConfigPersistsDormantGuildToPostgres
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
    guild_defaults_test.go:119: skipping postgres integration test: postgres test database url not configured: DISCORDCORE_TEST_DATABASE_URL is required for Postgres integration tests
--- PASS: TestPartnersCRUDAndDeterministicOrder (0.00s)
--- SKIP: TestEnsureMinimalGuildConfigPersistsDormantGuildToPostgres (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestNewMinimalGuildConfigDisablesAllFeatures
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/Database.PingTimeoutMS
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/whitespace_tolerated
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestNewMinimalGuildConfigDisablesAllFeatures (0.00s)
=== RUN   TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/unknown_values_fall_back_to_queue
=== CONT  TestHasAnyOverrideDetectsEachToggle
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/BotTheme
=== PAUSE TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/unknown_values_fall_back_to_queue
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestFeatureRegistryDefaultsMatchLegacyResolveFeatures
--- PASS: TestHasAnyOverrideDetectsEachToggle (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestLookupToggleRoundTrip
--- PASS: TestEnsureMinimalGuildConfigPersistsDormantGuild (0.00s)
--- PASS: TestFeatureRegistryDefaultsMatchLegacyResolveFeatures (0.00s)
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableDBCleanup
=== CONT  TestEncryptedStringJSON
--- PASS: TestFeatureTogglesJSONRoundTrip (0.00s)
=== CONT  TestEncryptionSymmetric
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableMessageLogs
=== CONT  TestEncryptedStringUnmarshalFallback
=== CONT  TestFeatureRegistryIDsAreUnique
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestLookupToggleRoundTrip (0.00s)
=== CONT  TestSetQOTDConfigRollsBackOnSaveError
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestFeatureRegistryIDsAreUnique (0.00s)
=== CONT  TestRuntimeConfigModerationLoggingEnabled
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableEntryExitLogs
--- PASS: TestRuntimeConfigModerationLoggingEnabled (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestRuntimeConfigNormalizedWebhookEmbedUpdates
=== CONT  TestRuntimeConfigUnmarshalMigratesLegacyModerationLogMode
=== NAME  TestEncryptedStringJSON
    encryption_test.go:64: unmarshalled secret mismatch: got "", want "my-secret-password-xyz"
--- PASS: TestEncryptedStringUnmarshalFallback (0.00s)
=== CONT  TestResolveRuntimeConfigModerationLoggingMerge
=== CONT  TestFindRolePanelPostingSearchesAcrossPanels
=== CONT  TestRolePanelPostingsCRUD
--- PASS: TestEncryptionSymmetric (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestRolePanelPostingValidation
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestGuildConfigIndexConcurrency
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableReactionLogs
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestRuntimeConfigNormalizedWebhookEmbedUpdates (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
--- PASS: TestRolePanelButtonCapEnforced (0.01s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelPostingValidation/valid
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableUserLogs
=== PAUSE TestRolePanelPostingValidation/valid
--- FAIL: TestEncryptedStringJSON (0.00s)
=== RUN   TestRolePanelPostingValidation/missing_channel
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableCleanLog
=== CONT  TestPostgresConfigStoreSaveLoadRoundTrip
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
    config_store_postgres_test.go:40: skipping postgres integration test: postgres test database url not configured: DISCORDCORE_TEST_DATABASE_URL is required for Postgres integration tests
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestRolePanelPostingValidation/missing_channel
--- PASS: TestSetQOTDConfigRollsBackOnSaveError (0.00s)
=== RUN   TestRolePanelPostingValidation/non-numeric_channel
=== CONT  TestCreateWebhookEmbedUpdateRollsBackOnSaveError
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestResolveRuntimeConfigModerationLoggingMerge (0.00s)
=== PAUSE TestRolePanelPostingValidation/non-numeric_channel
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestRuntimeConfigUnmarshalMigratesLegacyModerationLogMode (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestRolePanelPostingValidation/missing_message
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== PAUSE TestRolePanelPostingValidation/missing_message
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=0
--- SKIP: TestPostgresConfigStoreSaveLoadRoundTrip (0.00s)
=== RUN   TestRolePanelPostingValidation/non-numeric_message
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/LogModerationScope
=== PAUSE TestRolePanelPostingValidation/non-numeric_message
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=0
=== CONT  TestMemoryConfigStoreReturnsDefensiveCopies
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/PresenceWatchUserID
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestMemoryConfigStoreReturnsDefensiveCopies (0.00s)
=== CONT  TestCloneFeatureTogglesRoundtripAllFalse
=== CONT  TestMemoryConfigStoreRoundTrip
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestCloneFeatureTogglesIsolatesMutation
--- PASS: TestFindRolePanelPostingSearchesAcrossPanels (0.00s)
--- PASS: TestCreateWebhookEmbedUpdateRollsBackOnSaveError (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestCloneFeatureTogglesRoundtripAllTrue
--- PASS: TestCloneFeatureTogglesIsolatesMutation (0.00s)
=== CONT  TestCloneFeatureTogglesRoundtripAllNil
--- PASS: TestMemoryConfigStoreRoundTrip (0.00s)
=== CONT  TestGuildConfigIndexDedupePersistsOnLoad
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=2
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/PresenceWatchBot
--- PASS: TestCloneFeatureTogglesRoundtripAllFalse (0.00s)
=== CONT  TestGuildConfigIndexDuplicateFix
2026/06/23 22:08:19 WARN Structural integrity corrected locally: duplicate guilds purged from vector reason=test duplicates=1 remaining=2
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=2
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=3
--- PASS: TestCloneFeatureTogglesRoundtripAllNil (0.00s)
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestGuildConfigIndexUpdate
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
=== CONT  TestChannelsConfigHelpersStrict
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/MessageCacheTTLHours
=== CONT  TestSnapshotConfigReturnsDefensiveCopy
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=2
--- PASS: TestCloneFeatureTogglesRoundtripAllTrue (0.00s)
--- PASS: TestGuildConfigIndexDuplicateFix (0.00s)
2026/06/23 22:08:19 WARN Structural integrity corrected locally: duplicate guilds purged from vector reason=apply duplicates=1 remaining=2
--- PASS: TestChannelsConfigHelpersStrict (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=2
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/MessageDeleteOnLog
2026/06/23 22:08:19 WARN Mitigated degradation in index rebuilding error="removed 1 duplicate guild configurations" path=memory://bot_config_state
--- PASS: TestSnapshotConfigReturnsDefensiveCopy (0.00s)
=== CONT  TestGuildConfigIndexHit
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=remove guilds_count=1
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/MessageCacheCleanup
2026/06/23 22:08:19 INFO Configuration state transition completed duplicates_removed=1
--- PASS: TestGuildConfigIndexUpdate (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=4
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=2
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestGuildConfigIndexMiss
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/GlobalMaxWorkers
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=test guilds_count=1
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/BackfillChannelID
2026/06/23 22:08:19 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=1 autoRoleOrderMigrated=false
--- PASS: TestGuildConfigIndexHit (0.00s)
=== CONT  TestConfigManagerLoadConfigMigratesAutoAssignmentBoosterRole
--- PASS: TestRolePanelPostingsCRUD (0.00s)
=== CONT  TestChannelsConfigUnmarshalStrictSchema
=== CONT  TestValidateBotConfigRejectsInvalidRequiredRolesLength
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=5
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
=== CONT  TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder
--- PASS: TestGuildConfigIndexMiss (0.00s)
2026/06/23 22:08:19 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestGuildConfigIndexDedupePersistsOnLoad (0.00s)
--- PASS: TestValidateBotConfigRejectsInvalidRequiredRolesLength (0.00s)
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/BackfillStartDay
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestValidateBotConfigRejectsAutoAssignmentOrderMismatch
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/MimuWelcomeString
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=6
=== CONT  TestRolePanelMutationsAreSnapshotIsolated
2026/06/23 22:08:19 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
--- PASS: TestChannelsConfigUnmarshalStrictSchema (0.00s)
2026/06/23 22:08:19 ERROR Blocking global persistence failure request_id=a0cdfcdb28f6c7cbbbd83ee995ea4ce3 synthetic_code=500 stack_trace="goroutine 12 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/files.EmitBlockingError(0xc000048230, {0x140aad60a, 0x23}, {0x140ad9c00, 0xc000454120}, {0xc000288040, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/files/preferences.go:34 +0x11e\ngithub.com/small-frappuccino/discordcore/pkg/files.(*ConfigManager).SaveConfig(0xc00039a280)\n\tD:/Users/alice/git/discordcore/pkg/files/preferences.go:169 +0x245\ngithub.com/small-frappuccino/discordcore/pkg/files.TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder(0xc000002c00)\n\tD:/Users/alice/git/discordcore/pkg/files/auto_assignment_validation_test.go:156 +0x2c5\ntesting.tRunner(0xc000002c00, 0x140ad1760)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="ConfigManager.SaveConfig: validation failed: validateBotConfig: validation failed for field 'guilds[0].roles.auto_assignment.required_roles': required_roles[1] must match roles.booster_role (booster-role)"
--- PASS: TestValidateBotConfigRejectsAutoAssignmentOrderMismatch (0.00s)
--- PASS: TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder (0.00s)
=== CONT  TestRolePanelKeyValidation/keeps_digits_and_dashes
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/MimuGoodbyeString
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestRolePanelKeyValidation/rejects_empty
2026/06/23 22:08:19 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
=== CONT  TestRolePanelKeyValidation/rejects_whitespace_inside
=== CONT  TestRolePanelKeyValidation/rejects_punctuation
--- PASS: TestConfigManagerLoadConfigMigratesAutoAssignmentBoosterRole (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=7
=== CONT  TestRolePanelKeyValidation/rejects_unicode_letters
=== CONT  TestRolePanelKeyValidation/trims_and_lowercases
--- PASS: TestRolePanelMutationsAreSnapshotIsolated (0.00s)
=== CONT  TestRolePanelButtonValidation/label_too_long
=== CONT  TestRolePanelButtonValidation/emoji_id_without_name
=== CONT  TestRolePanelButtonValidation/non-numeric_role
=== CONT  TestRolePanelButtonValidation/valid_custom_emoji
=== CONT  TestRolePanelButtonValidation/valid_no_emoji
=== CONT  TestRolePanelButtonValidation/valid_unicode_emoji
=== CONT  TestQOTDConfigLegacyJSONTableMappings/legacy_forum_channel_id_maps_to_default_deck_forum_channel_id
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableBotRolePermMirror
=== CONT  TestRolePanelButtonValidation/missing_role
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=8
--- PASS: TestRolePanelKeyValidation (0.00s)
    --- PASS: TestRolePanelKeyValidation/keeps_digits_and_dashes (0.00s)
    --- PASS: TestRolePanelKeyValidation/rejects_empty (0.00s)
    --- PASS: TestRolePanelKeyValidation/rejects_whitespace_inside (0.00s)
    --- PASS: TestRolePanelKeyValidation/rejects_punctuation (0.00s)
    --- PASS: TestRolePanelKeyValidation/rejects_unicode_letters (0.00s)
    --- PASS: TestRolePanelKeyValidation/trims_and_lowercases (0.00s)
=== CONT  TestRolePanelButtonValidation/missing_label
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/BotRolePermMirrorActorRoleID
=== CONT  TestQOTDConfigLegacyJSONTableMappings/legacy_suppress_scheduled_publish_date_utc_maps_to_list
=== CONT  TestQOTDConfigLegacyJSONTableMappings/canonical_schedule_shadows_legacy
=== CONT  TestQOTDConfigLegacyJSONTableMappings/legacy_qotd_time_hour_utc_and_minute_maps_to_Schedule
=== CONT  TestQOTDConfigLegacyJSONTableMappings/legacy_question_channel_id_maps_to_default_deck_channel_id
--- PASS: TestRolePanelButtonValidation (0.01s)
    --- PASS: TestRolePanelButtonValidation/label_too_long (0.00s)
    --- PASS: TestRolePanelButtonValidation/emoji_id_without_name (0.00s)
    --- PASS: TestRolePanelButtonValidation/non-numeric_role (0.00s)
    --- PASS: TestRolePanelButtonValidation/valid_custom_emoji (0.00s)
    --- PASS: TestRolePanelButtonValidation/valid_no_emoji (0.00s)
    --- PASS: TestRolePanelButtonValidation/valid_unicode_emoji (0.00s)
    --- PASS: TestRolePanelButtonValidation/missing_label (0.00s)
    --- PASS: TestRolePanelButtonValidation/missing_role (0.00s)
=== CONT  TestQOTDConfigLegacyJSONTableMappings/legacy_publish_hour_utc_and_minute_maps_to_Schedule
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/WebhookEmbedValidation.Mode
=== CONT  TestGuildConfigLegacyMigration/migrates_domain_bot_instance_ids
=== CONT  TestGuildConfigLegacyMigration/ignores_empty_fields
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/WebhookEmbedValidation.TimeoutMS
=== CONT  TestGuildConfigLegacyMigration/normalizes_legacy_names
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=9
=== CONT  TestGuildConfigLegacyMigration/migrates_bot_instance_id
=== CONT  TestGuildConfigLegacyMigration/combines_both_legacy_fields
--- PASS: TestQOTDConfigLegacyJSONTableMappings (0.00s)
    --- PASS: TestQOTDConfigLegacyJSONTableMappings/legacy_forum_channel_id_maps_to_default_deck_forum_channel_id (0.00s)
    --- PASS: TestQOTDConfigLegacyJSONTableMappings/legacy_suppress_scheduled_publish_date_utc_maps_to_list (0.00s)
    --- PASS: TestQOTDConfigLegacyJSONTableMappings/legacy_qotd_time_hour_utc_and_minute_maps_to_Schedule (0.00s)
    --- PASS: TestQOTDConfigLegacyJSONTableMappings/canonical_schedule_shadows_legacy (0.00s)
    --- PASS: TestQOTDConfigLegacyJSONTableMappings/legacy_question_channel_id_maps_to_default_deck_channel_id (0.00s)
    --- PASS: TestQOTDConfigLegacyJSONTableMappings/legacy_publish_hour_utc_and_minute_maps_to_Schedule (0.00s)
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/DisableInteractiveEphemeral
=== CONT  TestGuildConfigLegacyMigration/does_not_overwrite_existing_canonical_tokens
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/explicit_queue_stays_queue
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/ModerationLogging
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/case-insensitive_random
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/whitespace_tolerated
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/empty_falls_back_to_queue
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/BackfillInitialDate
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=10
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/random_is_honored
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/WebhookEmbedUpdates
=== CONT  TestRolePanelPostingValidation/non-numeric_message
=== CONT  TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/unknown_values_fall_back_to_queue
=== RUN   TestResolveRuntimeConfigAdoptsEveryGuildField/PastebinGlobalOnly
=== CONT  TestRolePanelPostingValidation/missing_message
=== CONT  TestRolePanelPostingValidation/non-numeric_channel
--- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue (0.00s)
    --- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/explicit_queue_stays_queue (0.00s)
    --- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/case-insensitive_random (0.00s)
    --- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/whitespace_tolerated (0.00s)
    --- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/empty_falls_back_to_queue (0.00s)
    --- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/random_is_honored (0.00s)
    --- PASS: TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue/unknown_values_fall_back_to_queue (0.00s)
=== CONT  TestRolePanelPostingValidation/valid
=== CONT  TestRolePanelPostingValidation/missing_channel
--- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField (0.02s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.Driver (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.DatabaseURL (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.MaxOpenConns (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.MaxIdleConns (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.ConnMaxLifetimeSecs (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.ConnMaxIdleTimeSecs (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/Database.PingTimeoutMS (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/BotTheme (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableDBCleanup (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableMessageLogs (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableEntryExitLogs (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableReactionLogs (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableUserLogs (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableCleanLog (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/LogModerationScope (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/PresenceWatchUserID (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/PresenceWatchBot (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/MessageCacheTTLHours (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/MessageDeleteOnLog (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/MessageCacheCleanup (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/GlobalMaxWorkers (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/BackfillChannelID (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/BackfillStartDay (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/MimuWelcomeString (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/MimuGoodbyeString (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableBotRolePermMirror (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/BotRolePermMirrorActorRoleID (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/WebhookEmbedValidation.Mode (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/WebhookEmbedValidation.TimeoutMS (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/DisableInteractiveEphemeral (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/ModerationLogging (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/BackfillInitialDate (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/WebhookEmbedUpdates (0.00s)
    --- PASS: TestResolveRuntimeConfigAdoptsEveryGuildField/PastebinGlobalOnly (0.00s)
--- PASS: TestRolePanelPostingValidation (0.00s)
    --- PASS: TestRolePanelPostingValidation/non-numeric_message (0.00s)
    --- PASS: TestRolePanelPostingValidation/missing_message (0.00s)
    --- PASS: TestRolePanelPostingValidation/non-numeric_channel (0.00s)
    --- PASS: TestRolePanelPostingValidation/valid (0.00s)
    --- PASS: TestRolePanelPostingValidation/missing_channel (0.00s)
--- PASS: TestGuildConfigLegacyMigration (0.00s)
    --- PASS: TestGuildConfigLegacyMigration/ignores_empty_fields (0.00s)
    --- PASS: TestGuildConfigLegacyMigration/migrates_domain_bot_instance_ids (0.00s)
    --- PASS: TestGuildConfigLegacyMigration/normalizes_legacy_names (0.00s)
    --- PASS: TestGuildConfigLegacyMigration/migrates_bot_instance_id (0.00s)
    --- PASS: TestGuildConfigLegacyMigration/combines_both_legacy_fields (0.00s)
    --- PASS: TestGuildConfigLegacyMigration/does_not_overwrite_existing_canonical_tokens (0.00s)
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=11
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=12
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=13
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=14
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=15
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=16
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=17
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=18
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=19
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=20
2026/06/23 22:08:19 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=21
--- PASS: TestGuildConfigIndexConcurrency (0.01s)
--- PASS: TestNotifySubscribers_ContextTimeoutPreemption (0.05s)
--- PASS: TestJSONManagerSaveWritesAtomically (0.39s)
FAIL
FAIL	github.com/small-frappuccino/discordcore/pkg/files	0.871s
=== RUN   TestGenerator
=== PAUSE TestGenerator
=== RUN   TestHostNameParsing
=== PAUSE TestHostNameParsing
=== CONT  TestGenerator
--- PASS: TestGenerator (0.00s)
=== CONT  TestHostNameParsing
--- PASS: TestHostNameParsing (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/idgen	1.234s
=== RUN   TestLoggerSetupAndClose
time=2026-06-23T22:08:20.309-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:337 msg="logger initialized" service=test-bot category=application time=2026-06-23T22:08:20.3096419-03:00
time=2026-06-23T22:08:20.310-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:157 msg="app info message 1" service=test-bot category=application
time=2026-06-23T22:08:20.311-03:00 level=WARN source=D:/Users/alice/git/discordcore/pkg/log/logger.go:159 msg="app warn message 2" service=test-bot category=application
time=2026-06-23T22:08:20.311-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:174 msg="discord info message 3" service=test-bot category=discord
time=2026-06-23T22:08:20.312-03:00 level=WARN source=D:/Users/alice/git/discordcore/pkg/log/logger.go:176 msg="discord warn message 4" service=test-bot category=discord
time=2026-06-23T22:08:20.312-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:191 msg="db info message 5" service=test-bot category=database
time=2026-06-23T22:08:20.320-03:00 level=WARN source=D:/Users/alice/git/discordcore/pkg/log/logger.go:193 msg="db warn message 6" service=test-bot category=database
time=2026-06-23T22:08:20.320-03:00 level=ERROR source=D:/Users/alice/git/discordcore/pkg/log/logger.go:206 msg="error message 7" service=test-bot category=error
time=2026-06-23T22:08:20.320-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:161 msg="app default message" service=test-bot category=application
time=2026-06-23T22:08:20.320-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:178 msg="discord default message" service=test-bot category=discord
time=2026-06-23T22:08:20.320-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger.go:195 msg="db default message" service=test-bot category=database
time=2026-06-23T22:08:20.328-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger_test.go:69 msg="message with attributes" service=test-bot category=application key=val
time=2026-06-23T22:08:20.328-03:00 level=INFO source=D:/Users/alice/git/discordcore/pkg/log/logger_test.go:71 msg="message in group" service=test-bot category=application key=val
--- PASS: TestLoggerSetupAndClose (0.02s)
=== RUN   TestNilGlobalLoggerFallbacks
time=2026-06-23T22:08:20.330-03:00 level=INFO msg="test nil cl app" service=test-bot category=application
time=2026-06-23T22:08:20.331-03:00 level=INFO msg="test nil cl discord" service=test-bot category=application
time=2026-06-23T22:08:20.331-03:00 level=INFO msg="test nil cl db" service=test-bot category=application
time=2026-06-23T22:08:20.331-03:00 level=INFO msg="test cl2 app" service=test-bot category=application
time=2026-06-23T22:08:20.331-03:00 level=INFO msg="test cl2 discord" service=test-bot category=application
time=2026-06-23T22:08:20.332-03:00 level=INFO msg="test cl2 db" service=test-bot category=application
time=2026-06-23T22:08:20.332-03:00 level=INFO msg="ERROR: test nil el error" service=test-bot category=application
time=2026-06-23T22:08:20.332-03:00 level=INFO msg="ERROR: test el2 error" service=test-bot category=application
--- PASS: TestNilGlobalLoggerFallbacks (0.00s)
=== RUN   TestHelpers
--- PASS: TestHelpers (0.00s)
=== RUN   TestEmitBlockingError
--- PASS: TestEmitBlockingError (0.00s)
=== RUN   TestFatalf
--- PASS: TestFatalf (0.06s)
=== RUN   TestFatalfNilLogger
--- PASS: TestFatalfNilLogger (0.05s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/log	1.272s
=== RUN   TestFormatUserLabel
=== PAUSE TestFormatUserLabel
=== RUN   TestFormatUserRef
=== PAUSE TestFormatUserRef
=== RUN   TestFormatChannelLabel
=== PAUSE TestFormatChannelLabel
=== RUN   TestFormatRoleLabel
=== PAUSE TestFormatRoleLabel
=== RUN   TestFormatDurationFull
=== PAUSE TestFormatDurationFull
=== RUN   TestFormatDurationSmart
=== PAUSE TestFormatDurationSmart
=== RUN   TestFormatDuration
=== PAUSE TestFormatDuration
=== RUN   TestTruncateString
=== PAUSE TestTruncateString
=== RUN   TestLogEventCapabilities
=== PAUSE TestLogEventCapabilities
=== RUN   TestResolveLogChannel
=== PAUSE TestResolveLogChannel
=== RUN   TestCheckFeatureEnabled_Errors
=== PAUSE TestCheckFeatureEnabled_Errors
=== RUN   TestCheckFeatureEnabled_Toggles
=== PAUSE TestCheckFeatureEnabled_Toggles
=== RUN   TestCheckFeatureEnabled_NoChannelConfigured
=== PAUSE TestCheckFeatureEnabled_NoChannelConfigured
=== RUN   TestValidateLogCapability
=== PAUSE TestValidateLogCapability
=== RUN   TestValidateModerationLogChannel
=== PAUSE TestValidateModerationLogChannel
=== RUN   TestIsSharedModerationChannel
=== PAUSE TestIsSharedModerationChannel
=== CONT  TestFormatUserRef
--- PASS: TestFormatUserRef (0.00s)
=== CONT  TestFormatDuration
--- PASS: TestFormatDuration (0.00s)
=== CONT  TestCheckFeatureEnabled_Toggles
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestTruncateString
--- PASS: TestTruncateString (0.00s)
=== CONT  TestIsSharedModerationChannel
=== CONT  TestFormatRoleLabel
=== CONT  TestLogEventCapabilities
=== CONT  TestFormatDurationFull
--- PASS: TestIsSharedModerationChannel (0.00s)
=== CONT  TestFormatChannelLabel
--- PASS: TestFormatDurationFull (0.00s)
--- PASS: TestFormatRoleLabel (0.00s)
--- PASS: TestLogEventCapabilities (0.00s)
=== CONT  TestFormatUserLabel
--- PASS: TestFormatUserLabel (0.00s)
=== CONT  TestCheckFeatureEnabled_NoChannelConfigured
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestCheckFeatureEnabled_NoChannelConfigured (0.00s)
--- PASS: TestFormatChannelLabel (0.00s)
=== CONT  TestCheckFeatureEnabled_Errors
2026/06/23 22:08:21 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Initialized in clean state: primary file not detected path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=lookup_miss guilds_count=0
--- PASS: TestCheckFeatureEnabled_Errors (0.00s)
=== CONT  TestResolveLogChannel
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestCheckFeatureEnabled_Toggles (0.01s)
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestResolveLogChannel (0.00s)
=== CONT  TestValidateModerationLogChannel
--- PASS: TestValidateModerationLogChannel (0.00s)
=== CONT  TestFormatDurationSmart
--- PASS: TestFormatDurationSmart (0.00s)
=== CONT  TestValidateLogCapability
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestValidateLogCapability (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/logging	1.257s
=== RUN   TestHasRoleID
=== PAUSE TestHasRoleID
=== RUN   TestMemberHasRole
=== PAUSE TestMemberHasRole
=== RUN   TestEvaluateAutoRoleDecision
=== PAUSE TestEvaluateAutoRoleDecision
=== RUN   TestMemberEventService_LifeCycle
=== PAUSE TestMemberEventService_LifeCycle
=== RUN   TestMemberEventService_IngestGuildMemberAdd
=== PAUSE TestMemberEventService_IngestGuildMemberAdd
=== RUN   TestMemberEventService_IngestGuildMemberRemove
=== PAUSE TestMemberEventService_IngestGuildMemberRemove
=== RUN   TestMemberEventService_IngestGuildMemberRemove_StoreFallback
=== PAUSE TestMemberEventService_IngestGuildMemberRemove_StoreFallback
=== RUN   TestMemberEventService_IngestGuildMemberUpdate
=== PAUSE TestMemberEventService_IngestGuildMemberUpdate
=== RUN   TestMemberEventService_CleanupJoinTimes
=== PAUSE TestMemberEventService_CleanupJoinTimes
=== RUN   TestInMemoryMetrics
=== PAUSE TestInMemoryMetrics
=== RUN   TestNopMemberSink
=== PAUSE TestNopMemberSink
=== RUN   TestNopMetrics
=== PAUSE TestNopMetrics
=== RUN   TestMemberEventService_HandlesGuild
=== PAUSE TestMemberEventService_HandlesGuild
=== CONT  TestHasRoleID
--- PASS: TestHasRoleID (0.00s)
=== CONT  TestNopMetrics
--- PASS: TestNopMetrics (0.00s)
=== CONT  TestMemberEventService_IngestGuildMemberRemove
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
=== CONT  TestMemberEventService_HandlesGuild
=== CONT  TestMemberEventService_IngestGuildMemberAdd
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=4
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestMemberEventService_IngestGuildMemberRemove_StoreFallback
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
=== CONT  TestMemberEventService_LifeCycle
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
=== CONT  TestMemberHasRole
=== CONT  TestEvaluateAutoRoleDecision
=== RUN   TestEvaluateAutoRoleDecision/add_target_when_member_has_role_A_and_role_B
=== CONT  TestInMemoryMetrics
--- PASS: TestMemberEventService_IngestGuildMemberRemove (0.00s)
=== CONT  TestNopMemberSink
--- PASS: TestNopMemberSink (0.00s)
--- PASS: TestMemberEventService_LifeCycle (0.00s)
--- PASS: TestMemberEventService_IngestGuildMemberAdd (0.00s)
--- PASS: TestMemberEventService_HandlesGuild (0.00s)
=== CONT  TestMemberEventService_CleanupJoinTimes
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
--- PASS: TestMemberEventService_CleanupJoinTimes (0.00s)
--- PASS: TestMemberHasRole (0.00s)
--- PASS: TestInMemoryMetrics (0.00s)
=== CONT  TestMemberEventService_IngestGuildMemberUpdate
2026/06/23 22:08:21 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/23 22:08:21 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:21 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/23 22:08:21 INFO Configuration re-persisted after runtime normalization path=memory://bot_config_state duplicates=0 autoRoleOrderMigrated=true
=== RUN   TestEvaluateAutoRoleDecision/remove_target_when_role_A_is_missing
=== RUN   TestEvaluateAutoRoleDecision/noop_when_member_already_has_target_and_still_has_role_A
--- PASS: TestMemberEventService_IngestGuildMemberUpdate (0.00s)
=== RUN   TestEvaluateAutoRoleDecision/noop_when_only_role_A_is_present
--- PASS: TestEvaluateAutoRoleDecision (0.00s)
    --- PASS: TestEvaluateAutoRoleDecision/add_target_when_member_has_role_A_and_role_B (0.00s)
    --- PASS: TestEvaluateAutoRoleDecision/remove_target_when_role_A_is_missing (0.00s)
    --- PASS: TestEvaluateAutoRoleDecision/noop_when_member_already_has_target_and_still_has_role_A (0.00s)
    --- PASS: TestEvaluateAutoRoleDecision/noop_when_only_role_A_is_present (0.00s)
--- PASS: TestMemberEventService_IngestGuildMemberRemove_StoreFallback (0.05s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/members	1.944s
=== RUN   TestInMemoryMetrics
=== PAUSE TestInMemoryMetrics
=== RUN   TestMessageWriterMetrics
=== PAUSE TestMessageWriterMetrics
=== RUN   TestMessageCreateWriter_Basic
=== PAUSE TestMessageCreateWriter_Basic
=== RUN   TestAuditCacheState
=== PAUSE TestAuditCacheState
=== RUN   TestMessageEventService_LifecycleAndMetadata
=== PAUSE TestMessageEventService_LifecycleAndMetadata
=== RUN   TestMessageEventService_IngestMessageCreate
=== PAUSE TestMessageEventService_IngestMessageCreate
=== RUN   TestMessageEventService_IngestMessageUpdate_And_Delete
=== PAUSE TestMessageEventService_IngestMessageUpdate_And_Delete
=== RUN   TestMessageEventService_ActiveBotInstanceRouting
=== PAUSE TestMessageEventService_ActiveBotInstanceRouting
=== RUN   TestMessageEventService_TaskRouterAsynchronousHandling
=== PAUSE TestMessageEventService_TaskRouterAsynchronousHandling
=== RUN   TestLookupCachedMessage_PollingAndCancellation
=== PAUSE TestLookupCachedMessage_PollingAndCancellation
=== RUN   TestMessageEventService_PersistFallbacks
=== PAUSE TestMessageEventService_PersistFallbacks
=== RUN   TestAuditLogFetchFailureFallback
=== PAUSE TestAuditLogFetchFailureFallback
=== RUN   TestMessageEventService_SummarizeMessageContent
=== PAUSE TestMessageEventService_SummarizeMessageContent
=== RUN   TestNewerAuditEntry
=== PAUSE TestNewerAuditEntry
=== RUN   TestDeleteOnLogEnabled
=== PAUSE TestDeleteOnLogEnabled
=== CONT  TestMessageEventService_LifecycleAndMetadata
=== CONT  TestMessageEventService_ActiveBotInstanceRouting
=== CONT  TestMessageEventService_SummarizeMessageContent
=== CONT  TestDeleteOnLogEnabled
=== CONT  TestMessageEventService_IngestMessageUpdate_And_Delete
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
--- PASS: TestMessageEventService_SummarizeMessageContent (0.00s)
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=2
=== CONT  TestNewerAuditEntry
--- PASS: TestNewerAuditEntry (0.00s)
=== CONT  TestAuditLogFetchFailureFallback
--- PASS: TestDeleteOnLogEnabled (0.00s)
=== CONT  TestMessageEventService_PersistFallbacks
=== CONT  TestAuditCacheState
=== CONT  TestLookupCachedMessage_PollingAndCancellation
=== CONT  TestMessageEventService_IngestMessageCreate
2026/06/23 22:08:28 INFO Message event service started
2026/06/23 22:08:28 INFO Message event service started
2026/06/23 22:08:28 INFO Message event service stopped
--- PASS: TestAuditCacheState (0.00s)
2026/06/23 22:08:28 INFO Message event service started
=== CONT  TestMessageEventService_TaskRouterAsynchronousHandling
2026/06/23 22:08:28 INFO MessageUpdate received messageID=999 userID=123 guildID=111 channelID=222
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:28 WARN MessageCreate: failed to persist message cache entry guildID=111 channelID=222 messageID=999 userID=123 error="sync upsert err"
2026/06/23 22:08:28 INFO MessageUpdate received messageID=999 userID=123 guildID=111 channelID=222
2026/06/23 22:08:28 INFO MessageUpdate received messageID=999 userID=123 guildID=111 channelID=222
2026/06/23 22:08:28 WARN MessageCreate: failed to persist message version guildID=111 channelID=222 messageID=999 userID=123 error="sync insert version err"
2026/06/23 22:08:28 INFO MessageUpdate received messageID=999 userID=123 guildID=111 channelID=222
2026/06/23 22:08:28 WARN MessageCreate: failed to increment daily message metric guildID=111 channelID=222 messageID=999 userID=123 error="sync increment daily err"
2026/06/23 22:08:28 INFO MessageUpdate received messageID=999 userID=123 guildID=111 channelID=222
2026/06/23 22:08:28 INFO Message edit detected guildID=111 channelID=222 messageID=999 userID=123 username=alice
2026/06/23 22:08:28 WARN MessageUpdate: failed to persist updated message cache entry guildID=111 channelID=222 messageID=999 userID=123 error="sync upsert err"
2026/06/23 22:08:28 INFO MessageUpdate: store updated with new content guildID=111 channelID=222 messageID=999
2026/06/23 22:08:28 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
2026/06/23 22:08:28 WARN MessageUpdate: failed to persist message edit version guildID=111 channelID=222 messageID=999 userID=123 error="sync insert version err"
2026/06/23 22:08:28 WARN MessageDelete: failed to persist message delete version guildID=111 channelID=222 messageID=999 userID=123 error="sync insert version err"
2026/06/23 22:08:28 INFO Message delete detected guildID=111 channelID=222 messageID=999 userID=123 username=alice
2026/06/23 22:08:28 WARN MessageDelete: failed to delete message cache entry operation=op guildID=111 channelID=222 messageID=999 error="sync delete err"
--- PASS: TestMessageEventService_PersistFallbacks (0.00s)
=== CONT  TestMessageWriterMetrics
=== CONT  TestMessageCreateWriter_Basic
2026/06/23 22:08:28 INFO Message event service stopped
=== CONT  TestInMemoryMetrics
2026/06/23 22:08:28 INFO Message event service started
--- PASS: TestMessageWriterMetrics (0.00s)
--- PASS: TestMessageEventService_IngestMessageUpdate_And_Delete (0.00s)
--- PASS: TestInMemoryMetrics (0.00s)
2026/06/23 22:08:28 INFO Message event service started
--- PASS: TestMessageEventService_LifecycleAndMetadata (0.00s)
2026/06/23 22:08:28 INFO MessageUpdate received messageID=999 userID=123 guildID=111 channelID=222
2026/06/23 22:08:28 WARN MessageCreate writer: batch message upsert failed; falling back to sequential writes operation=message_create_writer.flush_messages messages=1 error="upsert messages batch err"
2026/06/23 22:08:28 WARN MessageCreate writer: batch history insert failed; falling back to sequential writes operation=message_create_writer.flush_versions versions=1 error="insert version batch err"
2026/06/23 22:08:28 WARN MessageCreate writer: sequential history insert failed operation=message_create_writer.flush_versions_fallback guildID=111 channelID="" messageID=222 userID="" eventType=create error="insert version batch err"
2026/06/23 22:08:28 WARN MessageCreate writer: batch daily metric flush failed; falling back to sequential increments operation=message_create_writer.flush_metrics buckets=1 error="increment daily batch err"
2026/06/23 22:08:28 WARN MessageCreate writer: sequential daily metric increment failed operation=message_create_writer.flush_metrics_fallback guildID=111 channelID=333 userID=444 error="increment daily batch err"
--- PASS: TestMessageCreateWriter_Basic (0.00s)
2026/06/23 22:08:28 INFO Message edit detected guildID=111 channelID=222 messageID=999 userID=123 username=alice
2026/06/23 22:08:28 INFO MessageUpdate: store updated with new content guildID=111 channelID=222 messageID=999
2026/06/23 22:08:28 INFO Message delete detected guildID=111 channelID=222 messageID=999 userID=123 username=alice
--- PASS: TestLookupCachedMessage_PollingAndCancellation (0.20s)
2026/06/23 22:08:29 WARN slow gateway event handler event=message_create duration=571.6178ms duration_ms=571 guildID="" channelID=444 messageID=999 userID=123
2026/06/23 22:08:29 INFO Message event service stopped
--- PASS: TestMessageEventService_IngestMessageCreate (0.58s)
2026/06/23 22:08:29 INFO Message event service stopped
--- PASS: TestMessageEventService_TaskRouterAsynchronousHandling (0.58s)
2026/06/23 22:08:29 WARN slow gateway event handler event=message_delete duration=598.9267ms duration_ms=598 guildID=111 channelID=222 messageID=999
--- PASS: TestAuditLogFetchFailureFallback (0.71s)
2026/06/23 22:08:29 INFO Message event service stopped
--- PASS: TestMessageEventService_ActiveBotInstanceRouting (0.83s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/messages	2.871s
=== RUN   TestHasPermission
=== PAUSE TestHasPermission
=== RUN   TestCanModerate
=== PAUSE TestCanModerate
=== RUN   TestNextFallbackCaseNumber_Race
=== PAUSE TestNextFallbackCaseNumber_Race
=== CONT  TestNextFallbackCaseNumber_Race
=== CONT  TestCanModerate
=== RUN   TestCanModerate/Actor_strictly_higher
=== CONT  TestHasPermission
=== RUN   TestHasPermission/Member_with_specific_permission
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=1
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=2
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=3
=== RUN   TestHasPermission/Member_without_specific_permission
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=4
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=5
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=6
=== RUN   TestHasPermission/Member_with_Administrator_flag_override
=== RUN   TestCanModerate/Target_strictly_higher
=== RUN   TestHasPermission/Member_with_total_omission_of_roles
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=7
=== RUN   TestCanModerate/Actor_and_Target_on_the_exact_same_layer_(same_role)
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=8
=== RUN   TestHasPermission/Nil_member
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=9
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=10
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=11
=== RUN   TestCanModerate/Actor_and_Target_on_the_exact_same_layer_(different_roles)
--- PASS: TestHasPermission (0.00s)
    --- PASS: TestHasPermission/Member_with_specific_permission (0.00s)
    --- PASS: TestHasPermission/Member_without_specific_permission (0.00s)
    --- PASS: TestHasPermission/Member_with_Administrator_flag_override (0.00s)
    --- PASS: TestHasPermission/Member_with_total_omission_of_roles (0.00s)
    --- PASS: TestHasPermission/Nil_member (0.00s)
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=12
=== RUN   TestCanModerate/Actor_missing_roles,_target_has_roles
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=13
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=14
--- PASS: TestCanModerate (0.01s)
    --- PASS: TestCanModerate/Actor_strictly_higher (0.00s)
    --- PASS: TestCanModerate/Target_strictly_higher (0.00s)
    --- PASS: TestCanModerate/Actor_and_Target_on_the_exact_same_layer_(same_role) (0.00s)
    --- PASS: TestCanModerate/Actor_and_Target_on_the_exact_same_layer_(different_roles) (0.00s)
    --- PASS: TestCanModerate/Actor_missing_roles,_target_has_roles (0.00s)
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=15
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=16
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=17
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=18
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=19
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=20
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=21
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=22
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=23
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=24
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=25
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=26
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=27
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=28
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=29
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=30
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=31
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=32
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=33
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=34
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=35
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=36
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=37
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=38
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=39
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=40
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=41
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=42
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=43
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=44
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=45
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=46
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=47
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=48
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=49
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=50
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=51
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=52
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=53
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=54
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=55
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=56
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=57
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=58
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=59
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=60
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=61
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=62
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=63
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=64
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=65
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=66
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=67
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=68
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=69
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=70
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=71
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=72
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=73
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=74
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=75
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=76
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=77
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=78
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=79
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=80
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=81
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=82
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=83
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=84
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=85
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=86
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=87
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=88
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=89
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=90
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=91
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=92
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=93
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=94
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=95
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=96
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=97
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=98
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=99
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=100
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=101
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=102
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=103
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=104
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=105
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=106
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=107
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=108
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=109
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=110
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=111
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=112
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=113
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=114
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=115
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=116
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=117
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=118
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=119
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=120
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=121
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=122
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=123
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=124
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=125
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=126
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=127
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=128
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=129
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=130
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=131
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=132
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=133
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=134
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=135
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=136
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=137
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=138
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=139
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=140
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=141
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=142
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=143
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=144
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=145
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=146
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=147
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=148
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=149
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=150
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=151
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=152
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=153
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=154
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=155
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=156
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=157
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=158
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=159
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=160
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=161
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=162
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=163
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=164
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=165
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=166
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=167
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=168
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=169
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=170
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=171
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=172
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=173
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=174
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=175
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=176
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=177
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=178
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=179
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=180
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=181
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=182
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=183
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=184
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=185
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=186
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=187
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=188
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=189
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=190
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=191
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=192
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=193
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=194
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=195
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=196
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=197
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=198
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=199
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=200
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=201
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=202
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=203
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=204
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=205
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=206
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=207
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=208
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=209
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=210
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=211
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=212
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=213
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=214
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=215
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=216
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=217
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=218
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=219
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=220
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=221
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=222
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=223
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=224
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=225
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=226
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=227
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=228
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=229
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=230
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=231
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=232
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=233
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=234
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=235
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=236
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=237
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=238
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=239
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=240
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=241
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=242
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=243
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=244
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=245
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=246
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=247
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=248
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=249
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=250
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=251
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=252
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=253
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=254
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=255
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=256
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=257
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=258
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=259
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=260
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=261
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=262
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=263
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=264
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=265
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=266
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=267
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=268
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=269
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=270
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=271
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=272
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=273
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=274
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=275
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=276
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=277
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=278
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=279
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=280
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=281
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=282
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=283
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=284
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=285
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=286
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=287
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=288
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=289
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=290
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=291
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=292
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=293
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=294
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=295
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=296
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=297
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=298
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=299
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=300
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=301
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=302
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=303
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=304
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=305
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=306
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=307
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=308
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=309
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=310
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=311
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=312
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=313
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=314
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=315
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=316
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=317
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=318
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=319
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=320
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=321
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=322
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=323
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=324
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=325
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=326
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=327
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=328
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=329
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=330
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=331
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=332
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=333
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=334
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=335
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=336
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=337
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=338
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=339
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=340
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=341
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=342
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=343
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=344
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=345
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=346
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=347
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=348
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=349
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=350
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=351
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=352
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=353
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=354
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=355
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=356
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=357
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=358
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=359
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=360
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=361
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=362
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=363
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=364
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=365
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=366
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=367
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=368
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=369
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=370
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=371
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=372
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=373
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=374
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=375
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=376
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=377
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=378
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=379
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=380
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=381
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=382
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=383
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=384
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=385
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=386
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=387
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=388
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=389
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=390
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=391
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=392
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=393
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=394
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=395
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=396
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=397
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=398
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=399
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=400
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=401
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=402
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=403
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=404
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=405
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=406
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=407
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=408
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=409
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=410
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=411
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=412
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=413
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=414
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=415
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=416
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=417
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=418
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=419
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=420
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=421
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=422
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=423
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=424
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=425
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=426
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=427
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=428
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=429
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=430
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=431
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=432
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=433
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=434
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=435
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=436
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=437
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=438
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=439
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=440
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=441
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=442
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=443
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=444
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=445
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=446
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=447
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=448
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=449
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=450
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=451
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=452
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=453
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=454
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=455
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=456
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=457
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=458
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=459
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=460
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=461
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=462
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=463
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=464
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=465
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=466
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=467
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=468
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=469
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=470
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=471
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=472
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=473
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=474
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=475
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=476
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=477
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=478
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=479
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=480
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=481
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=482
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=483
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=484
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=485
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=486
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=487
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=488
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=489
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=490
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=491
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=492
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=493
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=494
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=495
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=496
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=497
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=498
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=499
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=500
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=501
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=502
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=503
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=504
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=505
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=506
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=507
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=508
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=509
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=510
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=511
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=512
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=513
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=514
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=515
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=516
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=517
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=518
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=519
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=520
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=521
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=522
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=523
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=524
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=525
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=526
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=527
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=528
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=529
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=530
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=531
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=532
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=533
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=534
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=535
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=536
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=537
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=538
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=539
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=540
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=541
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=542
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=543
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=544
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=545
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=546
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=547
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=548
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=549
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=550
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=551
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=552
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=553
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=554
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=555
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=556
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=557
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=558
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=559
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=560
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=561
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=562
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=563
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=564
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=565
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=566
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=567
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=568
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=569
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=570
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=571
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=572
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=573
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=574
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=575
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=576
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=577
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=578
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=579
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=580
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=581
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=582
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=583
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=584
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=585
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=586
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=587
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=588
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=589
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=590
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=591
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=592
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=593
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=594
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=595
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=596
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=597
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=598
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=599
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=600
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=601
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=602
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=603
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=604
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=605
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=606
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=607
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=608
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=609
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=610
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=611
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=612
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=613
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=614
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=615
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=616
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=617
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=618
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=619
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=620
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=621
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=622
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=623
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=624
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=625
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=626
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=627
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=628
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=629
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=630
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=631
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=632
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=633
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=634
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=635
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=636
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=637
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=638
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=639
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=640
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=641
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=642
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=643
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=644
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=645
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=646
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=647
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=648
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=649
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=650
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=651
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=652
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=653
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=654
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=655
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=656
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=657
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=658
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=659
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=660
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=661
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=662
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=663
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=664
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=665
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=666
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=667
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=668
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=669
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=670
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=671
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=672
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=673
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=674
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=675
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=676
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=677
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=678
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=679
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=680
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=681
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=682
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=683
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=684
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=685
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=686
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=687
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=688
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=689
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=690
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=691
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=692
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=693
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=694
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=695
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=696
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=697
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=698
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=699
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=700
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=701
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=702
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=703
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=704
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=705
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=706
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=707
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=708
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=709
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=710
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=711
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=712
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=713
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=714
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=715
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=716
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=717
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=718
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=719
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=720
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=721
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=722
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=723
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=724
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=725
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=726
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=727
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=728
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=729
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=730
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=731
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=732
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=733
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=734
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=735
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=736
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=737
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=738
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=739
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=740
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=741
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=742
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=743
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=744
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=745
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=746
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=747
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=748
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=749
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=750
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=751
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=752
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=753
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=754
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=755
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=756
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=757
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=758
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=759
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=760
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=761
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=762
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=763
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=764
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=765
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=766
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=767
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=768
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=769
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=770
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=771
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=772
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=773
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=774
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=775
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=776
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=777
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=778
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=779
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=780
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=781
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=782
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=783
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=784
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=785
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=786
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=787
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=788
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=789
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=790
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=791
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=792
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=793
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=794
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=795
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=796
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=797
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=798
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=799
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=800
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=801
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=802
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=803
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=804
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=805
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=806
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=807
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=808
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=809
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=810
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=811
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=812
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=813
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=814
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=815
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=816
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=817
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=818
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=819
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=820
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=821
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=822
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=823
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=824
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=825
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=826
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=827
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=828
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=829
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=830
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=831
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=832
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=833
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=834
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=835
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=836
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=837
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=838
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=839
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=840
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=841
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=842
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=843
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=844
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=845
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=846
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=847
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=848
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=849
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=850
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=851
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=852
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=853
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=854
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=855
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=856
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=857
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=858
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=859
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=860
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=861
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=862
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=863
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=864
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=865
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=866
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=867
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=868
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=869
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=870
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=871
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=872
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=873
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=874
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=875
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=876
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=877
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=878
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=879
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=880
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=881
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=882
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=883
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=884
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=885
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=886
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=887
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=888
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=889
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=890
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=891
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=892
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=893
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=894
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=895
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=896
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=897
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=898
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=899
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=900
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=901
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=902
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=903
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=904
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=905
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=906
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=907
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=908
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=909
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=910
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=911
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=912
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=913
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=914
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=915
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=916
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=917
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=918
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=919
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=920
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=921
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=922
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=923
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=924
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=925
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=926
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=927
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=928
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=929
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=930
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=931
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=932
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=933
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=934
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=935
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=936
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=937
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=938
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=939
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=940
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=941
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=942
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=943
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=944
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=945
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=946
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=947
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=948
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=949
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=950
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=951
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=952
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=953
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=954
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=955
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=956
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=957
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=958
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=959
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=960
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=961
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=962
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=963
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=964
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=965
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=966
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=967
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=968
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=969
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=970
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=971
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=972
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=973
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=974
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=975
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=976
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=977
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=978
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=979
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=980
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=981
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=982
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=983
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=984
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=985
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=986
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=987
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=988
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=989
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=990
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=991
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=992
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=993
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=994
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=995
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=996
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=997
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=998
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=999
2026/06/23 22:08:28 WARN Mitigated service degradation: Local memory fallback case sequence allocated guild_id=123456789012345 case_id=1000
--- PASS: TestNextFallbackCaseNumber_Race (0.09s)
=== RUN   FuzzParseMemberIDs
=== RUN   FuzzParseMemberIDs/seed#0
=== RUN   FuzzParseMemberIDs/seed#1
=== RUN   FuzzParseMemberIDs/seed#2
=== RUN   FuzzParseMemberIDs/seed#3
=== RUN   FuzzParseMemberIDs/seed#4
--- PASS: FuzzParseMemberIDs (0.00s)
    --- PASS: FuzzParseMemberIDs/seed#0 (0.00s)
    --- PASS: FuzzParseMemberIDs/seed#1 (0.00s)
    --- PASS: FuzzParseMemberIDs/seed#2 (0.00s)
    --- PASS: FuzzParseMemberIDs/seed#3 (0.00s)
    --- PASS: FuzzParseMemberIDs/seed#4 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/moderation	1.145s
=== RUN   TestSummaryBasic
=== PAUSE TestSummaryBasic
=== RUN   TestSummaryConcurrency
=== PAUSE TestSummaryConcurrency
=== RUN   TestGetOrCreateLabeledCounter
=== PAUSE TestGetOrCreateLabeledCounter
=== RUN   TestGetOrCreateLabeledSummary
=== PAUSE TestGetOrCreateLabeledSummary
=== CONT  TestGetOrCreateLabeledSummary
=== CONT  TestSummaryConcurrency
--- PASS: TestGetOrCreateLabeledSummary (0.00s)
=== CONT  TestGetOrCreateLabeledCounter
=== CONT  TestSummaryBasic
--- PASS: TestGetOrCreateLabeledCounter (0.00s)
--- PASS: TestSummaryBasic (0.00s)
--- PASS: TestSummaryConcurrency (0.01s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/observability	1.069s
=== RUN   TestPing
=== PAUSE TestPing
=== RUN   TestMigrator
=== PAUSE TestMigrator
=== RUN   TestOpen_InvalidConfig
=== PAUSE TestOpen_InvalidConfig
=== RUN   TestOpen_InvalidDSN
=== PAUSE TestOpen_InvalidDSN
=== CONT  TestOpen_InvalidDSN
--- PASS: TestOpen_InvalidDSN (0.00s)
=== CONT  TestOpen_InvalidConfig
--- PASS: TestOpen_InvalidConfig (0.00s)
=== CONT  TestMigrator
    migrator_test.go:15: skipping test due to missing database url
--- SKIP: TestMigrator (0.00s)
=== CONT  TestPing
    health_test.go:19: skipping test due to missing database url
--- SKIP: TestPing (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/persistence	1.628s
=== RUN   TestUncoveredLifecycleAndService
=== PAUSE TestUncoveredLifecycleAndService
=== RUN   TestExecuteInGuildActor_Serialization
=== PAUSE TestExecuteInGuildActor_Serialization
=== RUN   TestExecuteInGuildActor_Parallelism
=== PAUSE TestExecuteInGuildActor_Parallelism
=== RUN   TestPublishScheduledIfDue_ContextExpiration
=== PAUSE TestPublishScheduledIfDue_ContextExpiration
=== RUN   TestReconcileGuild_SystemicFailureIsolation
=== PAUSE TestReconcileGuild_SystemicFailureIsolation
=== RUN   TestService_SchedulingEdges
=== PAUSE TestService_SchedulingEdges
=== CONT  TestExecuteInGuildActor_Serialization
=== CONT  TestPublishScheduledIfDue_ContextExpiration
=== CONT  TestService_SchedulingEdges
=== CONT  TestExecuteInGuildActor_Parallelism
=== CONT  TestUncoveredLifecycleAndService
2026/06/23 22:08:31 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
=== RUN   TestService_SchedulingEdges/Ano_Bissexto_-_Dia_29_de_Fevereiro
2026/06/23 22:08:31 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestReconcileGuild_SystemicFailureIsolation
=== RUN   TestService_SchedulingEdges/Virada_de_Ciclo_Solar_-_Reveillon
--- PASS: TestReconcileGuild_SystemicFailureIsolation (0.00s)
--- PASS: TestUncoveredLifecycleAndService (0.00s)
=== RUN   TestService_SchedulingEdges/Mesmo_dia_após_o_horário
--- PASS: TestService_SchedulingEdges (0.00s)
    --- PASS: TestService_SchedulingEdges/Ano_Bissexto_-_Dia_29_de_Fevereiro (0.00s)
    --- PASS: TestService_SchedulingEdges/Virada_de_Ciclo_Solar_-_Reveillon (0.00s)
    --- PASS: TestService_SchedulingEdges/Mesmo_dia_após_o_horário (0.00s)
--- PASS: TestExecuteInGuildActor_Serialization (0.00s)
--- PASS: TestExecuteInGuildActor_Parallelism (0.00s)
--- PASS: TestPublishScheduledIfDue_ContextExpiration (0.05s)
=== RUN   FuzzCalculateNextPublishDelay
=== RUN   FuzzCalculateNextPublishDelay/seed#0
=== RUN   FuzzCalculateNextPublishDelay/seed#1
=== RUN   FuzzCalculateNextPublishDelay/seed#2
=== RUN   FuzzCalculateNextPublishDelay/seed#3
--- PASS: FuzzCalculateNextPublishDelay (0.00s)
    --- PASS: FuzzCalculateNextPublishDelay/seed#0 (0.00s)
    --- PASS: FuzzCalculateNextPublishDelay/seed#1 (0.00s)
    --- PASS: FuzzCalculateNextPublishDelay/seed#2 (0.00s)
    --- PASS: FuzzCalculateNextPublishDelay/seed#3 (0.00s)
=== RUN   FuzzDetermineOfficialPostLifecycle
=== RUN   FuzzDetermineOfficialPostLifecycle/seed#0
=== RUN   FuzzDetermineOfficialPostLifecycle/seed#1
--- PASS: FuzzDetermineOfficialPostLifecycle (0.00s)
    --- PASS: FuzzDetermineOfficialPostLifecycle/seed#0 (0.00s)
    --- PASS: FuzzDetermineOfficialPostLifecycle/seed#1 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/qotd	2.419s
=== RUN   TestManager
=== PAUSE TestManager
=== RUN   TestMonitoringTogglesChanged
=== PAUSE TestMonitoringTogglesChanged
=== CONT  TestManager
=== CONT  TestMonitoringTogglesChanged
--- PASS: TestManager (0.00s)
=== RUN   TestMonitoringTogglesChanged/no_changes
=== RUN   TestMonitoringTogglesChanged/DisableEntryExitLogs
=== RUN   TestMonitoringTogglesChanged/DisableMessageLogs
=== RUN   TestMonitoringTogglesChanged/DisableReactionLogs
=== RUN   TestMonitoringTogglesChanged/DisableUserLogs
=== RUN   TestMonitoringTogglesChanged/DisableBotRolePermMirror
=== RUN   TestMonitoringTogglesChanged/BotRolePermMirrorActorRoleID
--- PASS: TestMonitoringTogglesChanged (0.00s)
    --- PASS: TestMonitoringTogglesChanged/no_changes (0.00s)
    --- PASS: TestMonitoringTogglesChanged/DisableEntryExitLogs (0.00s)
    --- PASS: TestMonitoringTogglesChanged/DisableMessageLogs (0.00s)
    --- PASS: TestMonitoringTogglesChanged/DisableReactionLogs (0.00s)
    --- PASS: TestMonitoringTogglesChanged/DisableUserLogs (0.00s)
    --- PASS: TestMonitoringTogglesChanged/DisableBotRolePermMirror (0.00s)
    --- PASS: TestMonitoringTogglesChanged/BotRolePermMirrorActorRoleID (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/runtimeapply	1.627s
=== RUN   TestBaseServiceStopReturnsErrorAndKeepsErrorState
=== PAUSE TestBaseServiceStopReturnsErrorAndKeepsErrorState
=== RUN   TestLegacyServiceWrapperPassesLifecycleContext
=== PAUSE TestLegacyServiceWrapperPassesLifecycleContext
=== RUN   TestServiceManagerStopFailureLeavesServiceInErrorState
=== PAUSE TestServiceManagerStopFailureLeavesServiceInErrorState
=== RUN   TestDynamicManager
=== PAUSE TestDynamicManager
=== RUN   TestBaseServiceAccessors
=== PAUSE TestBaseServiceAccessors
=== RUN   TestManagedService
=== PAUSE TestManagedService
=== RUN   TestManager_DependencyResolution
=== PAUSE TestManager_DependencyResolution
=== RUN   TestManager_CascadingFailure
=== PAUSE TestManager_CascadingFailure
=== RUN   TestManager_HealthMonitor_Restart
=== PAUSE TestManager_HealthMonitor_Restart
=== RUN   TestManager_FatalPropagation
=== PAUSE TestManager_FatalPropagation
=== RUN   TestOrchestrator_Preemption
=== PAUSE TestOrchestrator_Preemption
=== RUN   TestExecuteOrchestration_PanicRecovery
=== PAUSE TestExecuteOrchestration_PanicRecovery
=== RUN   TestExecuteOrchestration_ContextCancellationPropagates
=== PAUSE TestExecuteOrchestration_ContextCancellationPropagates
=== RUN   TestExecuteOrchestration_FalseSharingMitigation
=== PAUSE TestExecuteOrchestration_FalseSharingMitigation
=== CONT  TestLegacyServiceWrapperPassesLifecycleContext
=== CONT  TestManager_DependencyResolution
=== CONT  TestBaseServiceAccessors
2026/06/23 22:08:33 INFO Starting service... service=wrapped
2026/06/23 22:08:33 INFO Service started successfully service=wrapped
2026/06/23 22:08:33 INFO Stopping service... service=wrapped
2026/06/23 22:08:33 INFO Starting service... service=base
2026/06/23 22:08:33 INFO Service stopped service=wrapped
2026/06/23 22:08:33 INFO Service started successfully service=base
=== RUN   TestManager_DependencyResolution/linear_dependency
2026/06/23 22:08:33 INFO Stopping service... service=base
2026/06/23 22:08:33 INFO Service stopped service=base
=== CONT  TestOrchestrator_Preemption
=== CONT  TestExecuteOrchestration_FalseSharingMitigation
=== PAUSE TestManager_DependencyResolution/linear_dependency
=== RUN   TestManager_DependencyResolution/circular_dependency
--- PASS: TestLegacyServiceWrapperPassesLifecycleContext (0.00s)
--- PASS: TestBaseServiceAccessors (0.00s)
=== CONT  TestManager_FatalPropagation
=== CONT  TestDynamicManager
=== CONT  TestServiceManagerStopFailureLeavesServiceInErrorState
--- PASS: TestManager_FatalPropagation (0.00s)
=== CONT  TestExecuteOrchestration_ContextCancellationPropagates
=== CONT  TestBaseServiceStopReturnsErrorAndKeepsErrorState
2026/06/23 22:08:33 INFO Starting service... service=dyn
2026/06/23 22:08:33 INFO Starting service... service=test
2026/06/23 22:08:33 INFO Service started successfully service=test
2026/06/23 22:08:33 INFO Stopping service... service=test
2026/06/23 22:08:33 INFO Service started successfully service=dyn
2026/06/23 22:08:33 INFO Service registered service=managed type=monitoring priority=5 dependencies=[]
=== CONT  TestManagedService
2026/06/23 22:08:33 INFO Stopping service... service=dyn
2026/06/23 22:08:33 ERROR Service stop failed service=test err="stop failed"
=== PAUSE TestManager_DependencyResolution/circular_dependency
2026/06/23 22:08:33 INFO Starting service... service=managed
=== CONT  TestManager_CascadingFailure
2026/06/23 22:08:33 INFO Service stopped service=dyn
--- PASS: TestExecuteOrchestration_FalseSharingMitigation (0.00s)
2026/06/23 22:08:33 INFO Starting service... service=managed
2026/06/23 22:08:33 INFO Starting service... service=managed
=== CONT  TestManager_HealthMonitor_Restart
2026/06/23 22:08:33 INFO Service started successfully service=managed
2026/06/23 22:08:33 INFO Service registered service=s1 type=monitoring priority=5 dependencies=[]
2026/06/23 22:08:33 INFO Service registered service=s1 type=monitoring priority=5 dependencies=[]
2026/06/23 22:08:33 INFO Service started successfully service=managed
2026/06/23 22:08:33 INFO Starting all services...
2026/06/23 22:08:33 INFO Service registered service=s1 type=monitoring priority=5 dependencies=[]
2026/06/23 22:08:33 INFO Starting all services...
2026/06/23 22:08:33 WARN Service error detected, attempting restart service=managed err="simulated error"
=== CONT  TestExecuteOrchestration_PanicRecovery
2026/06/23 22:08:33 INFO Service started successfully service=managed
2026/06/23 22:08:33 INFO Stopping all services...
--- PASS: TestExecuteOrchestration_ContextCancellationPropagates (0.00s)
2026/06/23 22:08:33 INFO Service registered service=s2 type=monitoring priority=5 dependencies=[s1]
=== RUN   TestManager_DependencyResolution/missing_dependency
2026/06/23 22:08:33 INFO All services stopped successfully
2026/06/23 22:08:33 INFO Stopping service... service=managed
2026/06/23 22:08:33 INFO Starting service... service=s1
--- PASS: TestBaseServiceStopReturnsErrorAndKeepsErrorState (0.00s)
2026/06/23 22:08:33 INFO Stopping service... service=managed
2026/06/23 22:08:33 INFO Service started successfully service=s1
2026/06/23 22:08:33 INFO Starting service... service=s1
=== PAUSE TestManager_DependencyResolution/missing_dependency
2026/06/23 22:08:33 INFO Service registered service=s3 type=monitoring priority=5 dependencies=[s2]
2026/06/23 22:08:33 INFO All services started successfully services_count=1
--- PASS: TestManagedService (0.00s)
2026/06/23 22:08:33 INFO Starting all services...
2026/06/23 22:08:33 INFO Service started successfully service=s1
=== RUN   TestManager_DependencyResolution/multiple_independent_trees
2026/06/23 22:08:33 ERROR Service stop failed service=managed err="stop failed"
=== PAUSE TestManager_DependencyResolution/multiple_independent_trees
2026/06/23 22:08:33 INFO All services started successfully services_count=1
2026/06/23 22:08:33 INFO Starting service... service=s1
--- PASS: TestExecuteOrchestration_PanicRecovery (0.00s)
2026/06/23 22:08:33 INFO Stopping all services...
2026/06/23 22:08:33 ERROR Service stop failed service=managed err="service stop hook failed: stop failed"
=== CONT  TestManager_DependencyResolution/linear_dependency
2026/06/23 22:08:33 INFO Service started successfully service=s1
=== CONT  TestManager_DependencyResolution/missing_dependency
2026/06/23 22:08:33 INFO Stopping service... service=s1
--- PASS: TestServiceManagerStopFailureLeavesServiceInErrorState (0.00s)
2026/06/23 22:08:33 INFO Starting service... service=s2
2026/06/23 22:08:33 INFO Service registered service=a type=monitoring priority=5 dependencies=[]
2026/06/23 22:08:33 INFO Service stopped successfully service=s1
=== CONT  TestManager_DependencyResolution/multiple_independent_trees
2026/06/23 22:08:33 INFO Stopping all services...
2026/06/23 22:08:33 INFO All services stopped successfully
2026/06/23 22:08:33 INFO Service registered service=b type=monitoring priority=5 dependencies=[a]
=== CONT  TestManager_DependencyResolution/circular_dependency
2026/06/23 22:08:33 INFO Service registered service=a type=monitoring priority=5 dependencies=[d]
--- PASS: TestDynamicManager (0.00s)
2026/06/23 22:08:33 INFO Stopping service... service=s1
2026/06/23 22:08:33 INFO Service registered service=c type=monitoring priority=5 dependencies=[b]
2026/06/23 22:08:33 INFO Service stopped successfully service=s1
2026/06/23 22:08:33 INFO Service registered service=x type=monitoring priority=5 dependencies=[]
2026/06/23 22:08:33 INFO All services stopped successfully
2026/06/23 22:08:33 INFO Service registered service=a type=monitoring priority=5 dependencies=[c]
2026/06/23 22:08:33 INFO Service registered service=y type=monitoring priority=5 dependencies=[x]
--- PASS: TestManager_CascadingFailure (0.00s)
2026/06/23 22:08:33 INFO Service registered service=b type=monitoring priority=5 dependencies=[a]
2026/06/23 22:08:33 INFO Service registered service=1 type=monitoring priority=5 dependencies=[]
2026/06/23 22:08:33 INFO Service registered service=c type=monitoring priority=5 dependencies=[b]
2026/06/23 22:08:33 INFO Service registered service=2 type=monitoring priority=5 dependencies=[1]
--- PASS: TestManager_DependencyResolution (0.00s)
    --- PASS: TestManager_DependencyResolution/missing_dependency (0.00s)
    --- PASS: TestManager_DependencyResolution/linear_dependency (0.00s)
    --- PASS: TestManager_DependencyResolution/multiple_independent_trees (0.00s)
    --- PASS: TestManager_DependencyResolution/circular_dependency (0.00s)
2026/06/23 22:08:33 ERROR Service health check failed service=s1 message="always failing" details=map[]
2026/06/23 22:08:33 WARN Attempting to restart unhealthy service service=s1
2026/06/23 22:08:33 INFO Restarting service... service=s1
2026/06/23 22:08:33 INFO Stopping service... service=s1
2026/06/23 22:08:33 INFO Service stopped successfully service=s1
2026/06/23 22:08:33 INFO Starting service... service=s1
2026/06/23 22:08:33 INFO Service started successfully service=s1
2026/06/23 22:08:33 ERROR Service health check failed service=s1 message="always failing" details=map[]
2026/06/23 22:08:33 WARN Attempting to restart unhealthy service service=s1
2026/06/23 22:08:33 INFO Restarting service... service=s1
2026/06/23 22:08:33 INFO Stopping service... service=s1
2026/06/23 22:08:33 INFO Service stopped successfully service=s1
2026/06/23 22:08:33 INFO Starting service... service=s1
2026/06/23 22:08:33 INFO Service started successfully service=s1
2026/06/23 22:08:33 INFO Stopping all services...
2026/06/23 22:08:33 INFO Stopping service... service=s1
2026/06/23 22:08:33 INFO Service stopped successfully service=s1
2026/06/23 22:08:33 INFO All services stopped successfully
--- PASS: TestManager_HealthMonitor_Restart (0.00s)
--- PASS: TestOrchestrator_Preemption (0.01s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/service	1.049s
=== RUN   TestApplyMemberAdd
=== PAUSE TestApplyMemberAdd
=== RUN   TestApplyMemberRemove
=== PAUSE TestApplyMemberRemove
=== RUN   TestApplyStatsMemberUpdate
=== PAUSE TestApplyStatsMemberUpdate
=== RUN   TestStatsService_DatabasePreemption
=== PAUSE TestStatsService_DatabasePreemption
=== RUN   TestHandlesGuild
=== PAUSE TestHandlesGuild
=== RUN   TestStatsServiceMethods
=== PAUSE TestStatsServiceMethods
=== RUN   TestShouldRunStatsUpdate
=== PAUSE TestShouldRunStatsUpdate
=== RUN   TestStatsTrackedRoles
=== PAUSE TestStatsTrackedRoles
=== RUN   TestStatsRequiresBotClassification
=== PAUSE TestStatsRequiresBotClassification
=== RUN   TestFilterTrackedRoles
=== PAUSE TestFilterTrackedRoles
=== RUN   TestStatsCountForChannel
=== PAUSE TestStatsCountForChannel
=== RUN   TestStatsGuildStateMethods
=== PAUSE TestStatsGuildStateMethods
=== RUN   TestStatsSnapshotHelpers
=== PAUSE TestStatsSnapshotHelpers
=== RUN   TestStatsIntervalHelpers
=== PAUSE TestStatsIntervalHelpers
=== RUN   TestStatsStateAndStoreHelpers
=== PAUSE TestStatsStateAndStoreHelpers
=== RUN   TestStatsGuildStateMemoryHelpers
=== PAUSE TestStatsGuildStateMemoryHelpers
=== RUN   TestStatsReconcileInterval
=== PAUSE TestStatsReconcileInterval
=== RUN   TestReconcileGuild
=== PAUSE TestReconcileGuild
=== RUN   TestReconcileAllGuilds
=== PAUSE TestReconcileAllGuilds
=== RUN   TestStatsServiceLifecycle
=== PAUSE TestStatsServiceLifecycle
=== RUN   TestStatsServiceHandlesGuild
=== PAUSE TestStatsServiceHandlesGuild
=== RUN   TestNormalizeMemberType
=== PAUSE TestNormalizeMemberType
=== RUN   TestMemberTypeMatches
=== PAUSE TestMemberTypeMatches
=== RUN   TestRenderStatsChannelName
=== PAUSE TestRenderStatsChannelName
=== CONT  TestApplyMemberAdd
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
=== CONT  TestStatsSnapshotHelpers
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
--- PASS: TestStatsSnapshotHelpers (0.00s)
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestRenderStatsChannelName
=== CONT  TestFilterTrackedRoles
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestRenderStatsChannelName (0.00s)
=== CONT  TestReconcileAllGuilds
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
--- PASS: TestFilterTrackedRoles (0.00s)
=== CONT  TestStatsReconcileInterval
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
=== CONT  TestStatsCountForChannel
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestStatsGuildStateMethods
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestMemberTypeMatches
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
=== CONT  TestStatsServiceHandlesGuild
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
=== CONT  TestShouldRunStatsUpdate
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
--- PASS: TestStatsCountForChannel (0.00s)
=== CONT  TestNormalizeMemberType
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
=== CONT  TestStatsServiceLifecycle
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
=== CONT  TestReconcileGuild
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
=== CONT  TestStatsGuildStateMemoryHelpers
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestStatsRequiresBotClassification
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=3
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
2026/06/23 22:08:33 INFO Updated stats channel name guild_id=guild-stats-main operation=monitoring.stats.publish_channel channelID=c1 count=0 name=test0
=== CONT  TestStatsIntervalHelpers
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestMemberTypeMatches (0.00s)
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestStatsServiceMethods
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
=== CONT  TestHandlesGuild
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
=== CONT  TestStatsService_DatabasePreemption
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
=== CONT  TestApplyStatsMemberUpdate
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
=== CONT  TestStatsTrackedRoles
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=add guilds_count=1
--- PASS: TestStatsGuildStateMethods (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
=== CONT  TestApplyMemberRemove
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestApplyMemberAdd (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
--- PASS: TestNormalizeMemberType (0.00s)
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
--- PASS: TestStatsReconcileInterval (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
=== CONT  TestStatsStateAndStoreHelpers
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestStatsRequiresBotClassification (0.00s)
2026/06/23 22:08:33 WARN Applied configuration does not contain active guilds. Running in basal mode. path=mock
--- PASS: TestShouldRunStatsUpdate (0.00s)
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestReconcileAllGuilds (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
--- PASS: TestStatsIntervalHelpers (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestStatsGuildStateMemoryHelpers (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=2
--- PASS: TestStatsServiceLifecycle (0.00s)
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
--- PASS: TestStatsTrackedRoles (0.00s)
2026/06/23 22:08:33 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
--- PASS: TestStatsServiceHandlesGuild (0.00s)
2026/06/23 22:08:33 INFO Configuration state transition completed duplicates_removed=0
--- PASS: TestStatsServiceMethods (0.00s)
2026/06/23 22:08:33 WARN Failed to read stats seed metadata guild_id=test-guild operation=monitoring.stats.seed.read err="context canceled"
--- PASS: TestStatsStateAndStoreHelpers (0.00s)
2026/06/23 22:08:33 INFO I/O state transition: Configuration successfully persisted path=mock
2026/06/23 22:08:33 INFO Reconciled stats counters guild_id=test-guild operation=monitoring.stats.reconcile members=0 trackedRoles=0
2026/06/23 22:08:33 INFO Reconciled stats counters guild_id=guild-stats-main operation=monitoring.stats.reconcile members=2 trackedRoles=0
--- PASS: TestReconcileGuild (0.00s)
2026/06/23 22:08:33 ERROR Failed to update stats channels guild_id=test-guild operation=monitoring.stats.publish err="MonitoringService.publishStatsForGuild: context canceled"
--- PASS: TestStatsService_DatabasePreemption (0.00s)
--- PASS: TestHandlesGuild (0.00s)
--- PASS: TestApplyStatsMemberUpdate (0.00s)
--- PASS: TestApplyMemberRemove (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/stats	1.121s
=== RUN   TestStore_Iterators_EarlyExitCursorClosure
=== PAUSE TestStore_Iterators_EarlyExitCursorClosure
=== RUN   TestStore_Context_ExecutionBoundaryTimeout
=== PAUSE TestStore_Context_ExecutionBoundaryTimeout
=== RUN   TestStore_Context_StructuralMisalignment
=== PAUSE TestStore_Context_StructuralMisalignment
=== RUN   TestStore_Context_UnaryMissingState
=== PAUSE TestStore_Context_UnaryMissingState
=== RUN   TestStore_Members_Idempotency_And_Temporal_Precedence
=== PAUSE TestStore_Members_Idempotency_And_Temporal_Precedence
=== RUN   TestStore_Members_UserPreferences
=== PAUSE TestStore_Members_UserPreferences
=== RUN   TestStore_Members_UpsertMemberJoinContext
=== PAUSE TestStore_Members_UpsertMemberJoinContext
=== RUN   TestStore_Members_GetActiveGuildMemberStatesContext
=== PAUSE TestStore_Members_GetActiveGuildMemberStatesContext
=== RUN   TestStore_Members_MarkMemberLeftContext
=== PAUSE TestStore_Members_MarkMemberLeftContext
=== RUN   TestStore_Members_UpsertMemberRoles
=== PAUSE TestStore_Members_UpsertMemberRoles
=== RUN   TestStore_Messages_UpsertMessage
=== PAUSE TestStore_Messages_UpsertMessage
=== RUN   TestStore_Messages_UpsertMessagesContext
=== PAUSE TestStore_Messages_UpsertMessagesContext
=== RUN   TestStore_Messages_GetMessage
=== PAUSE TestStore_Messages_GetMessage
=== RUN   TestStore_Messages_DeleteMessagesContext
=== PAUSE TestStore_Messages_DeleteMessagesContext
=== RUN   TestStore_Messages_InsertMessageVersionsMixedBatchContext
=== PAUSE TestStore_Messages_InsertMessageVersionsMixedBatchContext
=== RUN   TestStore_Messages_CleanupExpiredMessages
=== PAUSE TestStore_Messages_CleanupExpiredMessages
=== RUN   TestStore_Messages_IncrementDailyMessageCountsContext
=== PAUSE TestStore_Messages_IncrementDailyMessageCountsContext
=== RUN   TestStore_Messages_DeleteMessage
=== PAUSE TestStore_Messages_DeleteMessage
=== RUN   TestStore_Messages_InsertMessageVersion
=== PAUSE TestStore_Messages_InsertMessageVersion
=== RUN   TestStore_Messages_IncrementDailyMessageCount
=== PAUSE TestStore_Messages_IncrementDailyMessageCount
=== RUN   TestStore_Moderation_NextModerationCaseNumber
=== PAUSE TestStore_Moderation_NextModerationCaseNumber
=== RUN   TestStore_Moderation_GetGuildOwnerID_ErrNoRows
=== PAUSE TestStore_Moderation_GetGuildOwnerID_ErrNoRows
=== RUN   TestStore_Moderation_CreateWarning
=== PAUSE TestStore_Moderation_CreateWarning
=== RUN   TestStore_Moderation_NextModerationCaseNumber_Errors
=== PAUSE TestStore_Moderation_NextModerationCaseNumber_Errors
=== RUN   TestStore_Moderation_CreateWarning_Errors
=== PAUSE TestStore_Moderation_CreateWarning_Errors
=== RUN   TestStore_Moderation_ListModerationWarnings
=== PAUSE TestStore_Moderation_ListModerationWarnings
=== RUN   TestStore_Moderation_GuildOwner
=== PAUSE TestStore_Moderation_GuildOwner
=== RUN   TestStorageQueriesUsePositionalPlaceholders
=== PAUSE TestStorageQueriesUsePositionalPlaceholders
=== RUN   TestStore_Schema_ParametricDeletion
=== PAUSE TestStore_Schema_ParametricDeletion
=== RUN   TestStore_Schema_TypeRegression
=== PAUSE TestStore_Schema_TypeRegression
=== RUN   TestStore_Init_Success
=== PAUSE TestStore_Init_Success
=== RUN   TestStore_Init_MissingColumnsAndEmptyQOTD
=== PAUSE TestStore_Init_MissingColumnsAndEmptyQOTD
=== RUN   TestStore_Init_Failures
=== PAUSE TestStore_Init_Failures
=== RUN   TestStore_TransactionalLifecycle_CommitValidation
=== PAUSE TestStore_TransactionalLifecycle_CommitValidation
=== RUN   TestStore_TransactionalLifecycle_HybridRollbackFailures
=== PAUSE TestStore_TransactionalLifecycle_HybridRollbackFailures
=== RUN   TestNewStore_NilDB
=== PAUSE TestNewStore_NilDB
=== RUN   TestStore_System_NextTicketID
=== PAUSE TestStore_System_NextTicketID
=== RUN   TestStore_System_BotSince_ErrNoRows
=== PAUSE TestStore_System_BotSince_ErrNoRows
=== RUN   TestStore_System_GetCacheEntry_ErrNoRows
=== PAUSE TestStore_System_GetCacheEntry_ErrNoRows
=== RUN   TestStore_System_GetCacheEntry_Expired
=== PAUSE TestStore_System_GetCacheEntry_Expired
=== RUN   TestStore_System_BotSince
=== PAUSE TestStore_System_BotSince
=== RUN   TestStore_System_RuntimeMeta
=== PAUSE TestStore_System_RuntimeMeta
=== RUN   TestStore_System_UpsertCacheEntriesContext
=== PAUSE TestStore_System_UpsertCacheEntriesContext
=== RUN   TestStore_System_GetCacheEntriesByType
=== PAUSE TestStore_System_GetCacheEntriesByType
=== RUN   TestStore_System_CleanupExpiredCacheEntries
=== PAUSE TestStore_System_CleanupExpiredCacheEntries
=== RUN   TestStore_System_GetCacheStatsContext
=== PAUSE TestStore_System_GetCacheStatsContext
=== RUN   TestStore_System_PurgeGuildModerationData
=== PAUSE TestStore_System_PurgeGuildModerationData
=== RUN   TestStore_System_IncrementDailyMemberEvents
=== PAUSE TestStore_System_IncrementDailyMemberEvents
=== CONT  TestStore_Context_ExecutionBoundaryTimeout
=== CONT  TestStore_Moderation_ListModerationWarnings
=== RUN   TestStore_Moderation_ListModerationWarnings/empty_inputs
=== CONT  TestStore_Members_UserPreferences
=== CONT  TestStore_Messages_UpsertMessagesContext
=== RUN   TestStore_Members_UserPreferences/success_GetUserPreferences
=== RUN   TestStore_Messages_UpsertMessagesContext/empty_slice
=== CONT  TestStore_Messages_GetMessage
=== RUN   TestStore_Messages_UpsertMessagesContext/with_records_and_validation
=== RUN   TestStore_Moderation_ListModerationWarnings/success
=== CONT  TestStore_Members_UpsertMemberRoles
=== CONT  TestStore_Members_GetActiveGuildMemberStatesContext
=== RUN   TestStore_Members_GetActiveGuildMemberStatesContext/empty_validation
=== RUN   TestStore_Messages_GetMessage/found_with_expiry
=== CONT  TestStore_Messages_UpsertMessage
=== RUN   TestStore_Members_GetActiveGuildMemberStatesContext/success
=== RUN   TestStore_Messages_UpsertMessage/with_expiry
=== RUN   TestStore_Members_UserPreferences/defaults_when_GetUserPreferences_ErrNoRows
=== RUN   TestStore_Members_UserPreferences/error_GetUserPreferences
--- PASS: TestStore_Members_UpsertMemberRoles (0.00s)
=== CONT  TestStore_Members_UpsertMemberJoinContext
=== RUN   TestStore_Members_UpsertMemberJoinContext/empty_validation
=== RUN   TestStore_Members_UpsertMemberJoinContext/success
=== RUN   TestStore_Messages_GetMessage/found_without_expiry
=== RUN   TestStore_Moderation_ListModerationWarnings/query_error
=== RUN   TestStore_Messages_UpsertMessagesContext/db_error
=== RUN   TestStore_Members_GetActiveGuildMemberStatesContext/query_error
=== RUN   TestStore_Members_UserPreferences/success_UpdateUserPreferences
--- PASS: TestStore_Context_ExecutionBoundaryTimeout (0.00s)
=== CONT  TestStore_Members_Idempotency_And_Temporal_Precedence
=== RUN   TestStore_Messages_UpsertMessage/without_expiry
=== RUN   TestStore_Messages_GetMessage/not_found
=== CONT  TestStore_Members_MarkMemberLeftContext
--- PASS: TestStore_Moderation_ListModerationWarnings (0.01s)
    --- PASS: TestStore_Moderation_ListModerationWarnings/empty_inputs (0.00s)
    --- PASS: TestStore_Moderation_ListModerationWarnings/success (0.00s)
    --- PASS: TestStore_Moderation_ListModerationWarnings/query_error (0.00s)
--- PASS: TestStore_Members_UserPreferences (0.01s)
    --- PASS: TestStore_Members_UserPreferences/success_GetUserPreferences (0.00s)
    --- PASS: TestStore_Members_UserPreferences/defaults_when_GetUserPreferences_ErrNoRows (0.00s)
    --- PASS: TestStore_Members_UserPreferences/error_GetUserPreferences (0.00s)
    --- PASS: TestStore_Members_UserPreferences/success_UpdateUserPreferences (0.00s)
=== CONT  TestStore_Context_UnaryMissingState
=== CONT  TestStore_Messages_IncrementDailyMessageCount
=== CONT  TestStore_Moderation_CreateWarning
--- PASS: TestStore_Messages_UpsertMessagesContext (0.01s)
    --- PASS: TestStore_Messages_UpsertMessagesContext/empty_slice (0.00s)
    --- PASS: TestStore_Messages_UpsertMessagesContext/with_records_and_validation (0.00s)
    --- PASS: TestStore_Messages_UpsertMessagesContext/db_error (0.00s)
=== CONT  TestStore_Moderation_NextModerationCaseNumber_Errors
=== RUN   TestStore_Messages_GetMessage/scan_error
=== RUN   TestStore_Moderation_NextModerationCaseNumber_Errors/empty_guildID
--- PASS: TestStore_Members_MarkMemberLeftContext (0.00s)
--- PASS: TestStore_Members_UpsertMemberJoinContext (0.01s)
    --- PASS: TestStore_Members_UpsertMemberJoinContext/empty_validation (0.00s)
    --- PASS: TestStore_Members_UpsertMemberJoinContext/success (0.00s)
--- PASS: TestStore_Members_GetActiveGuildMemberStatesContext (0.01s)
    --- PASS: TestStore_Members_GetActiveGuildMemberStatesContext/empty_validation (0.00s)
    --- PASS: TestStore_Members_GetActiveGuildMemberStatesContext/success (0.00s)
    --- PASS: TestStore_Members_GetActiveGuildMemberStatesContext/query_error (0.00s)
=== RUN   TestStore_Messages_UpsertMessage/db_error
--- PASS: TestStore_Context_UnaryMissingState (0.00s)
=== CONT  TestStore_Moderation_GetGuildOwnerID_ErrNoRows
--- PASS: TestStore_Members_Idempotency_And_Temporal_Precedence (0.00s)
--- PASS: TestStore_Moderation_CreateWarning (0.00s)
=== CONT  TestStore_Messages_IncrementDailyMessageCountsContext
=== RUN   TestStore_Messages_IncrementDailyMessageCountsContext/empty_deltas
=== CONT  TestStore_Moderation_NextModerationCaseNumber
=== RUN   TestStore_Moderation_NextModerationCaseNumber_Errors/db_error
=== CONT  TestStore_Messages_InsertMessageVersion
=== RUN   TestStore_Messages_IncrementDailyMessageCountsContext/success
--- PASS: TestStore_Moderation_GetGuildOwnerID_ErrNoRows (0.00s)
=== CONT  TestStore_Messages_DeleteMessage
--- PASS: TestStore_Messages_IncrementDailyMessageCount (0.00s)
=== CONT  TestStore_Messages_CleanupExpiredMessages
=== RUN   TestStore_Messages_CleanupExpiredMessages/success
=== RUN   TestStore_Messages_CleanupExpiredMessages/error
--- PASS: TestStore_Moderation_NextModerationCaseNumber (0.00s)
=== CONT  TestStore_Context_StructuralMisalignment
=== CONT  TestStore_Messages_DeleteMessagesContext
=== CONT  TestStore_Messages_InsertMessageVersionsMixedBatchContext
--- PASS: TestStore_Messages_GetMessage (0.01s)
    --- PASS: TestStore_Messages_GetMessage/found_with_expiry (0.00s)
    --- PASS: TestStore_Messages_GetMessage/found_without_expiry (0.00s)
    --- PASS: TestStore_Messages_GetMessage/not_found (0.00s)
    --- PASS: TestStore_Messages_GetMessage/scan_error (0.00s)
=== CONT  TestNewStore_NilDB
=== CONT  TestStore_System_IncrementDailyMemberEvents
=== RUN   TestStore_Messages_DeleteMessagesContext/empty_keys
=== RUN   TestStore_Messages_InsertMessageVersionsMixedBatchContext/empty_versions
--- PASS: TestStore_Messages_DeleteMessage (0.00s)
=== CONT  TestStore_System_GetCacheStatsContext
=== CONT  TestStore_System_PurgeGuildModerationData
--- PASS: TestStore_Messages_InsertMessageVersion (0.00s)
=== RUN   TestStore_System_GetCacheStatsContext/success
--- PASS: TestStore_Messages_UpsertMessage (0.01s)
    --- PASS: TestStore_Messages_UpsertMessage/with_expiry (0.00s)
    --- PASS: TestStore_Messages_UpsertMessage/without_expiry (0.00s)
    --- PASS: TestStore_Messages_UpsertMessage/db_error (0.00s)
=== RUN   TestStore_System_PurgeGuildModerationData/success
--- PASS: TestNewStore_NilDB (0.00s)
--- PASS: TestStore_Moderation_NextModerationCaseNumber_Errors (0.00s)
    --- PASS: TestStore_Moderation_NextModerationCaseNumber_Errors/empty_guildID (0.00s)
    --- PASS: TestStore_Moderation_NextModerationCaseNumber_Errors/db_error (0.00s)
=== RUN   TestStore_Messages_InsertMessageVersionsMixedBatchContext/begin_tx_error
=== RUN   TestStore_Messages_DeleteMessagesContext/valid_keys_and_duplicates
--- PASS: TestStore_Context_StructuralMisalignment (0.00s)
=== CONT  TestStore_System_CleanupExpiredCacheEntries
=== CONT  TestStore_System_UpsertCacheEntriesContext
--- PASS: TestStore_Messages_CleanupExpiredMessages (0.00s)
    --- PASS: TestStore_Messages_CleanupExpiredMessages/success (0.00s)
    --- PASS: TestStore_Messages_CleanupExpiredMessages/error (0.00s)
=== RUN   TestStore_System_UpsertCacheEntriesContext/empty_entries
=== RUN   TestStore_Messages_IncrementDailyMessageCountsContext/db_error
=== RUN   TestStore_Messages_InsertMessageVersionsMixedBatchContext/lock_counter_error
=== RUN   TestStore_System_UpsertCacheEntriesContext/entries_with_and_without_guild_id
=== RUN   TestStore_System_GetCacheStatsContext/query_error
--- PASS: TestStore_System_CleanupExpiredCacheEntries (0.00s)
=== CONT  TestStore_System_RuntimeMeta
--- PASS: TestStore_System_IncrementDailyMemberEvents (0.00s)
=== RUN   TestStore_System_PurgeGuildModerationData/exec_error_and_rollback
=== RUN   TestStore_Messages_DeleteMessagesContext/db_error
=== CONT  TestStore_System_GetCacheEntriesByType
=== RUN   TestStore_System_GetCacheEntriesByType/success
=== CONT  TestStore_System_GetCacheEntry_Expired
--- PASS: TestStore_System_GetCacheStatsContext (0.00s)
    --- PASS: TestStore_System_GetCacheStatsContext/success (0.00s)
    --- PASS: TestStore_System_GetCacheStatsContext/query_error (0.00s)
--- PASS: TestStore_Messages_DeleteMessagesContext (0.00s)
    --- PASS: TestStore_Messages_DeleteMessagesContext/empty_keys (0.00s)
    --- PASS: TestStore_Messages_DeleteMessagesContext/valid_keys_and_duplicates (0.00s)
    --- PASS: TestStore_Messages_DeleteMessagesContext/db_error (0.00s)
=== CONT  TestStore_System_BotSince
--- PASS: TestStore_Messages_IncrementDailyMessageCountsContext (0.01s)
    --- PASS: TestStore_Messages_IncrementDailyMessageCountsContext/empty_deltas (0.00s)
    --- PASS: TestStore_Messages_IncrementDailyMessageCountsContext/success (0.00s)
    --- PASS: TestStore_Messages_IncrementDailyMessageCountsContext/db_error (0.00s)
=== RUN   TestStore_Messages_InsertMessageVersionsMixedBatchContext/successful_reservation_and_insertion
=== RUN   TestStore_System_UpsertCacheEntriesContext/foreign_key_error_should_be_ignored/mitigated
=== CONT  TestStore_System_BotSince_ErrNoRows
=== CONT  TestStore_System_GetCacheEntry_ErrNoRows
--- PASS: TestStore_System_GetCacheEntry_Expired (0.00s)
=== RUN   TestStore_System_BotSince/success
=== RUN   TestStore_System_GetCacheEntriesByType/query_error
=== CONT  TestStore_Iterators_EarlyExitCursorClosure
--- PASS: TestStore_System_PurgeGuildModerationData (0.00s)
    --- PASS: TestStore_System_PurgeGuildModerationData/success (0.00s)
    --- PASS: TestStore_System_PurgeGuildModerationData/exec_error_and_rollback (0.00s)
--- PASS: TestStore_System_GetCacheEntriesByType (0.00s)
    --- PASS: TestStore_System_GetCacheEntriesByType/success (0.00s)
    --- PASS: TestStore_System_GetCacheEntriesByType/query_error (0.00s)
=== CONT  TestStore_Schema_TypeRegression
--- PASS: TestStore_System_GetCacheEntry_ErrNoRows (0.00s)
=== NAME  TestStore_Schema_TypeRegression
    schema_test.go:188: skipping testcontainers-go tests on local windows environment due to rootless docker limitation
--- SKIP: TestStore_Schema_TypeRegression (0.00s)
=== CONT  TestStore_TransactionalLifecycle_CommitValidation
=== CONT  TestStore_Init_Failures
=== CONT  TestStore_System_NextTicketID
=== CONT  TestStore_TransactionalLifecycle_HybridRollbackFailures
--- PASS: TestStore_System_BotSince_ErrNoRows (0.00s)
--- PASS: TestStore_System_UpsertCacheEntriesContext (0.00s)
    --- PASS: TestStore_System_UpsertCacheEntriesContext/empty_entries (0.00s)
    --- PASS: TestStore_System_UpsertCacheEntriesContext/entries_with_and_without_guild_id (0.00s)
    --- PASS: TestStore_System_UpsertCacheEntriesContext/foreign_key_error_should_be_ignored/mitigated (0.00s)
--- PASS: TestStore_Iterators_EarlyExitCursorClosure (0.00s)
=== RUN   TestStore_Init_Failures/check_columns_error
=== RUN   TestStore_System_BotSince/empty_guildID
--- PASS: TestStore_System_BotSince (0.00s)
    --- PASS: TestStore_System_BotSince/success (0.00s)
    --- PASS: TestStore_System_BotSince/empty_guildID (0.00s)
=== CONT  TestStore_Moderation_GuildOwner
=== RUN   TestStore_Moderation_GuildOwner/empty_inputs
=== CONT  TestStore_Init_Success
=== RUN   TestStore_Moderation_GuildOwner/success_Set_and_Get
--- PASS: TestStore_System_NextTicketID (0.00s)
=== CONT  TestStore_Schema_ParametricDeletion
--- PASS: TestStore_System_RuntimeMeta (0.01s)
=== NAME  TestStore_Schema_ParametricDeletion
    schema_test.go:153: skipping testcontainers-go tests on local windows environment due to rootless docker limitation
--- SKIP: TestStore_Schema_ParametricDeletion (0.00s)
=== RUN   TestStore_Init_Failures/migration_alter_table_error_and_rollback_error
=== CONT  TestStore_Init_MissingColumnsAndEmptyQOTD
=== CONT  TestStorageQueriesUsePositionalPlaceholders
--- PASS: TestStore_TransactionalLifecycle_CommitValidation (0.00s)
=== CONT  TestStore_Moderation_CreateWarning_Errors
=== RUN   TestStore_Moderation_CreateWarning_Errors/missing_fields
=== RUN   TestStore_Messages_InsertMessageVersionsMixedBatchContext/failed_rollback_scenario
--- PASS: TestStore_TransactionalLifecycle_HybridRollbackFailures (0.00s)
=== RUN   TestStore_Moderation_GuildOwner/owner_is_null
=== RUN   TestStore_Moderation_CreateWarning_Errors/begin_tx_error
=== RUN   TestStore_Moderation_CreateWarning_Errors/insert_warning_error
--- PASS: TestStore_Moderation_GuildOwner (0.00s)
    --- PASS: TestStore_Moderation_GuildOwner/empty_inputs (0.00s)
    --- PASS: TestStore_Moderation_GuildOwner/success_Set_and_Get (0.00s)
    --- PASS: TestStore_Moderation_GuildOwner/owner_is_null (0.00s)
--- PASS: TestStore_Messages_InsertMessageVersionsMixedBatchContext (0.01s)
    --- PASS: TestStore_Messages_InsertMessageVersionsMixedBatchContext/empty_versions (0.00s)
    --- PASS: TestStore_Messages_InsertMessageVersionsMixedBatchContext/begin_tx_error (0.00s)
    --- PASS: TestStore_Messages_InsertMessageVersionsMixedBatchContext/lock_counter_error (0.00s)
    --- PASS: TestStore_Messages_InsertMessageVersionsMixedBatchContext/successful_reservation_and_insertion (0.01s)
    --- PASS: TestStore_Messages_InsertMessageVersionsMixedBatchContext/failed_rollback_scenario (0.00s)
--- PASS: TestStore_Moderation_CreateWarning_Errors (0.00s)
    --- PASS: TestStore_Moderation_CreateWarning_Errors/missing_fields (0.00s)
    --- PASS: TestStore_Moderation_CreateWarning_Errors/begin_tx_error (0.00s)
    --- PASS: TestStore_Moderation_CreateWarning_Errors/insert_warning_error (0.00s)
=== RUN   TestStore_Init_Failures/validate_schema_table_missing
--- PASS: TestStore_Init_Success (0.01s)
=== RUN   TestStore_Init_Failures/validate_schema_column_type_mismatch
--- PASS: TestStore_Init_MissingColumnsAndEmptyQOTD (0.01s)
=== RUN   TestStore_Init_Failures/validate_schema_column_missing
=== RUN   TestStore_Init_Failures/reset_qotd_query_failure
--- PASS: TestStore_Init_Failures (0.02s)
    --- PASS: TestStore_Init_Failures/check_columns_error (0.00s)
    --- PASS: TestStore_Init_Failures/migration_alter_table_error_and_rollback_error (0.00s)
    --- PASS: TestStore_Init_Failures/validate_schema_table_missing (0.00s)
    --- PASS: TestStore_Init_Failures/validate_schema_column_type_mismatch (0.00s)
    --- PASS: TestStore_Init_Failures/validate_schema_column_missing (0.00s)
    --- PASS: TestStore_Init_Failures/reset_qotd_query_failure (0.00s)
--- PASS: TestStorageQueriesUsePositionalPlaceholders (0.04s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/storage/postgres	2.369s
=== RUN   TestFailingStore
=== PAUSE TestFailingStore
=== CONT  TestFailingStore
err = failed to connect to `user=postgres database=postgres`:
	[::1]:5432 (localhost): dial error: storagetest: connector always fails
	127.0.0.1:5432 (localhost): dial error: storagetest: connector always fails
	[::1]:5432 (localhost): dial error: storagetest: connector always fails
	127.0.0.1:5432 (localhost): dial error: storagetest: connector always fails
--- PASS: TestFailingStore (0.02s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/storage/postgres/storagetest	2.262s
?   	github.com/small-frappuccino/discordcore/pkg/system	[no test files]
=== RUN   TestAdapters_TransactionalFallback
=== PAUSE TestAdapters_TransactionalFallback
=== RUN   TestNotificationAdapters_AllMethods
=== PAUSE TestNotificationAdapters_AllMethods
=== RUN   TestRouter_GroupKeySerialization
=== PAUSE TestRouter_GroupKeySerialization
=== RUN   TestRouter_ExecutionLimiter
=== PAUSE TestRouter_ExecutionLimiter
=== RUN   TestRouter_IdempotencyTTL
=== PAUSE TestRouter_IdempotencyTTL
=== RUN   TestRouter_RetryHeap
=== PAUSE TestRouter_RetryHeap
=== RUN   TestRouter_CronSchedule
=== PAUSE TestRouter_CronSchedule
=== RUN   TestRouter_ContextCancel
=== PAUSE TestRouter_ContextCancel
=== RUN   TestRouter_Observability
=== PAUSE TestRouter_Observability
=== CONT  TestRouter_GroupKeySerialization
=== CONT  TestRouter_RetryHeap
=== CONT  TestRouter_IdempotencyTTL
=== CONT  TestRouter_Observability
=== CONT  TestNotificationAdapters_AllMethods
=== CONT  TestRouter_CronSchedule
--- PASS: TestRouter_IdempotencyTTL (0.00s)
--- PASS: TestRouter_RetryHeap (0.00s)
=== CONT  TestRouter_ExecutionLimiter
=== CONT  TestRouter_ContextCancel
=== CONT  TestAdapters_TransactionalFallback
--- PASS: TestRouter_Observability (0.00s)
--- PASS: TestRouter_ContextCancel (0.00s)
--- PASS: TestAdapters_TransactionalFallback (0.00s)
--- PASS: TestNotificationAdapters_AllMethods (0.00s)
--- PASS: TestRouter_CronSchedule (0.00s)
--- PASS: TestRouter_ExecutionLimiter (0.00s)
--- PASS: TestRouter_GroupKeySerialization (0.09s)
=== RUN   FuzzRouter_QueueMutation
=== RUN   FuzzRouter_QueueMutation/seed#0
=== RUN   FuzzRouter_QueueMutation/seed#1
=== RUN   FuzzRouter_QueueMutation/seed#2
--- PASS: FuzzRouter_QueueMutation (0.00s)
    --- PASS: FuzzRouter_QueueMutation/seed#0 (0.00s)
    --- PASS: FuzzRouter_QueueMutation/seed#1 (0.00s)
    --- PASS: FuzzRouter_QueueMutation/seed#2 (0.00s)
=== RUN   FuzzRouter_HeapLimits
=== RUN   FuzzRouter_HeapLimits/seed#0
=== RUN   FuzzRouter_HeapLimits/seed#1
=== RUN   FuzzRouter_HeapLimits/seed#2
--- PASS: FuzzRouter_HeapLimits (0.00s)
    --- PASS: FuzzRouter_HeapLimits/seed#0 (0.00s)
    --- PASS: FuzzRouter_HeapLimits/seed#1 (0.00s)
    --- PASS: FuzzRouter_HeapLimits/seed#2 (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/task	2.160s
=== RUN   TestOpenIsolatedDatabase
=== PAUSE TestOpenIsolatedDatabase
=== RUN   TestBaseDatabaseURLFromEnv_NotConfigured
=== PAUSE TestBaseDatabaseURLFromEnv_NotConfigured
=== RUN   TestOpenIsolatedDatabase_Errors
=== PAUSE TestOpenIsolatedDatabase_Errors
=== CONT  TestOpenIsolatedDatabase_Errors
=== CONT  TestBaseDatabaseURLFromEnv_NotConfigured
=== CONT  TestOpenIsolatedDatabase
--- PASS: TestBaseDatabaseURLFromEnv_NotConfigured (0.00s)
=== NAME  TestOpenIsolatedDatabase
    postgres_test.go:15: skipping test due to missing database url
--- SKIP: TestOpenIsolatedDatabase (0.00s)
--- PASS: TestOpenIsolatedDatabase_Errors (0.51s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/testdb	2.261s
=== RUN   TestTheme_Default
=== PAUSE TestTheme_Default
=== RUN   TestTheme_Register
=== PAUSE TestTheme_Register
=== RUN   TestTheme_SetCurrent
=== PAUSE TestTheme_SetCurrent
=== RUN   TestTheme_GettersAndDefaults
=== PAUSE TestTheme_GettersAndDefaults
=== RUN   TestTheme_HalloweenTheme
=== PAUSE TestTheme_HalloweenTheme
=== CONT  TestTheme_Default
=== CONT  TestTheme_SetCurrent
=== CONT  TestTheme_GettersAndDefaults
--- PASS: TestTheme_Default (0.00s)
=== CONT  TestTheme_HalloweenTheme
=== CONT  TestTheme_Register
--- PASS: TestTheme_SetCurrent (0.00s)
--- PASS: TestTheme_GettersAndDefaults (0.00s)
--- PASS: TestTheme_Register (0.00s)
--- PASS: TestTheme_HalloweenTheme (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/theme	1.741s
=== RUN   TestPermissionsBitwise
=== PAUSE TestPermissionsBitwise
=== RUN   TestNamingLogic
=== PAUSE TestNamingLogic
=== RUN   TestOpenPermissions
=== PAUSE TestOpenPermissions
=== RUN   TestManager_NewAndNextID
=== PAUSE TestManager_NewAndNextID
=== CONT  TestPermissionsBitwise
=== RUN   TestPermissionsBitwise/ComputeCloseMember
=== CONT  TestManager_NewAndNextID
=== CONT  TestOpenPermissions
--- PASS: TestOpenPermissions (0.00s)
=== CONT  TestNamingLogic
--- PASS: TestNamingLogic (0.00s)
=== RUN   TestPermissionsBitwise/ComputeReopenMember
--- PASS: TestPermissionsBitwise (0.00s)
    --- PASS: TestPermissionsBitwise/ComputeCloseMember (0.00s)
    --- PASS: TestPermissionsBitwise/ComputeReopenMember (0.00s)
--- PASS: TestManager_NewAndNextID (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/pkg/tickets	2.023s
=== RUN   TestDistFSIncludesPlaceholderIndex
--- PASS: TestDistFSIncludesPlaceholderIndex (0.00s)
=== RUN   TestTrackedEmbedIndexMatchesTemplate
--- PASS: TestTrackedEmbedIndexMatchesTemplate (0.00s)
PASS
ok  	github.com/small-frappuccino/discordcore/ui	2.241s
FAIL
