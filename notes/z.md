=== RUN   TestBotRuntime_InitializationRouting
=== RUN   TestBotRuntime_InitializationRouting/Exhaustive_Mocking:_All_Features_Enabled
2026/06/22 20:48:52 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/22 20:48:52 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:52 INFO Logging bot runtime capability activated guild_id=g1 bot_instance_id=main intents_mask=2131459
2026/06/22 20:48:52 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=main concurrency_budget=8
2026/06/22 20:48:52 INFO Architectural state transition: Configured runtime task router budget botInstanceID=main globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:52 INFO Service registered service=messages type=messages priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Service registered service=member_events_main type=monitoring priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Service registered service=discord_automod_adapter type=automod priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Service registered service=qotd type=monitoring priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Architectural state transition: QOTD runtime initialized botInstanceID=main
2026/06/22 20:48:52 INFO Registered DiscordGo event handlers for stats
2026/06/22 20:48:52 INFO Service registered service=stats type=monitoring priority=1 dependencies=[]
2026/06/22 20:48:52 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=main
2026/06/22 20:48:52 INFO Stopping all services...
2026/06/22 20:48:52 INFO All services stopped successfully
=== RUN   TestBotRuntime_InitializationRouting/Routing_Disabled_Features_Yields_Idle_Core
2026/06/22 20:48:52 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/22 20:48:52 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:52 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=main concurrency_budget=8
2026/06/22 20:48:52 INFO Architectural state transition: Configured runtime task router budget botInstanceID=main globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:52 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=main
2026/06/22 20:48:52 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=main
2026/06/22 20:48:52 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=main
2026/06/22 20:48:52 INFO Stopping all services...
2026/06/22 20:48:52 INFO All services stopped successfully
--- PASS: TestBotRuntime_InitializationRouting (0.00s)
    --- PASS: TestBotRuntime_InitializationRouting/Exhaustive_Mocking:_All_Features_Enabled (0.00s)
    --- PASS: TestBotRuntime_InitializationRouting/Routing_Disabled_Features_Yields_Idle_Core (0.00s)
=== RUN   TestBotRuntime_CapabilityBitmaskDerivation
=== PAUSE TestBotRuntime_CapabilityBitmaskDerivation
=== RUN   TestBotRuntimeResolver_ConcurrentMemoryRotation
=== PAUSE TestBotRuntimeResolver_ConcurrentMemoryRotation
=== RUN   TestBotRuntimeResolver_WaitBarrierOrchestration
2026/06/22 20:48:52 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:52 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:52 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
--- PASS: TestBotRuntimeResolver_WaitBarrierOrchestration (0.07s)
=== RUN   TestSupervisorFaultIsolation
2026/06/22 20:48:52 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/22 20:48:52 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:52 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=3
2026/06/22 20:48:52 INFO Architectural state transition: Background worker pool initialized parallelism_limit=4 queue_capacity=6
2026/06/22 20:48:52 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=3
2026/06/22 20:48:52 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:52 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/22 20:48:52 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child3
2026/06/22 20:48:52 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child2
2026/06/22 20:48:52 WARN Instance authentication compromised, triggering token revocation in configuration request_id=auth_bot_child3 botInstanceID=child3 error="open discord session for child3: HTTP 401 Unauthorized"
2026/06/22 20:48:52 WARN Interference in bot runtime initialization, triggering compensatory branch botInstanceID=child2 attempt=1 mitigation="executing exponential backoff algorithm" error="open discord session for child2: simulated gateway panic in child runtime ID 2"
2026/06/22 20:48:52 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/22 20:48:52 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
2026/06/22 20:48:52 INFO Planned instance shutdown triggered by token removal botInstanceID=child3
2026/06/22 20:48:52 ERROR Blocking failure after mitigation routine exhaustion on runtime start request_id=start_exhaust_child3 botInstanceID=child3 error="open discord session for child3: HTTP 401 Unauthorized"
2026/06/22 20:48:52 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/22 20:48:52 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/22 20:48:52 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/22 20:48:52 INFO Architectural state transition: Configured runtime task router budget botInstanceID=child1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:52 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/22 20:48:52 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Primary instance coupling and socket registration completed successfully botInstanceID=child1
2026/06/22 20:48:52 INFO Starting service... service=bot-runtime-child1-1782172132617264400
2026/06/22 20:48:52 INFO executeStopAndRemove DELETING instance id=child3
2026/06/22 20:48:52 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=child1
2026/06/22 20:48:52 INFO Starting all services...
2026/06/22 20:48:52 INFO Starting service... service=command-handler
2026/06/22 20:48:52 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:52 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:52 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/22 20:48:52 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:52 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/22 20:48:52 INFO Service started successfully service=command-handler
2026/06/22 20:48:52 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/22 20:48:52 INFO All services started successfully services_count=1
2026/06/22 20:48:52 INFO Architectural state transition: Configured runtime task router budget botInstanceID=child1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:52 INFO Runtime do bot totalmente operacional botInstanceID=child1
2026/06/22 20:48:52 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/22 20:48:52 INFO Service started successfully service=bot-runtime-child1-1782172132617264400
2026/06/22 20:48:52 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:52 INFO Primary instance coupling and socket registration completed successfully botInstanceID=child1
2026/06/22 20:48:52 INFO Starting service... service=bot-runtime-child1-1782172132617790200
2026/06/22 20:48:52 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:52 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/22 20:48:52 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=child1
2026/06/22 20:48:52 INFO Starting all services...
2026/06/22 20:48:52 INFO Starting service... service=command-handler
2026/06/22 20:48:52 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:52 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:52 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:52 INFO Service started successfully service=command-handler
2026/06/22 20:48:52 INFO All services started successfully services_count=1
2026/06/22 20:48:52 INFO Runtime do bot totalmente operacional botInstanceID=child1
2026/06/22 20:48:52 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:52 INFO Service started successfully service=bot-runtime-child1-1782172132617790200
2026/06/22 20:48:52 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/22 20:48:52 INFO executeStopAndRemove DELETING instance id=child2
2026/06/22 20:48:52 INFO Stopping service... service=bot-runtime-child1-1782172132617790200
2026/06/22 20:48:52 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=child1
2026/06/22 20:48:52 INFO Stopping all services...
2026/06/22 20:48:52 INFO Stopping service... service=command-handler
2026/06/22 20:48:52 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:52 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:52 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:52 INFO All services stopped successfully
2026/06/22 20:48:52 INFO Service stopped service=bot-runtime-child1-1782172132617790200
2026/06/22 20:48:52 INFO executeStopAndRemove DELETING instance id=child1
2026/06/22 20:48:52 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/22 20:48:52 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
--- PASS: TestSupervisorFaultIsolation (0.52s)
=== RUN   TestZeroStateIdling
2026/06/22 20:48:53 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/22 20:48:53 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/22 20:48:53 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:53 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/22 20:48:53 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/22 20:48:53 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/22 20:48:53 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:53 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/22 20:48:53 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/22 20:48:53 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/22 20:48:53 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestZeroStateIdling (0.05s)
=== RUN   TestSupervisorSwarmTopology
2026/06/22 20:48:53 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/22 20:48:53 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:53 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=10
2026/06/22 20:48:53 INFO Architectural state transition: Background worker pool initialized parallelism_limit=4 queue_capacity=20
2026/06/22 20:48:53 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=10
2026/06/22 20:48:53 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:53 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childH
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childF
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childH botUser=test#
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childF botUser=test#
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childH concurrency_budget=8
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childF concurrency_budget=8
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childH globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childF globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childH
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childF
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childH
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childH-1782172133191598200
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childF-1782172133191598200
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childH
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childF
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childF
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childD
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childB
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childD botUser=test#
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childB botUser=test#
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childD concurrency_budget=8
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childH
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childF
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childD globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childB concurrency_budget=8
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childH-1782172133191598200
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childB globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childF-1782172133191598200
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childD
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childB
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childB
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childB-1782172133192717700
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childD-1782172133192717700
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childD
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childD
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childB
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childA
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childC
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childA botUser=test#
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childC botUser=test#
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childA concurrency_budget=8
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childC concurrency_budget=8
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childB
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childA globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childB-1782172133192717700
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childC globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childA
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childC
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childD
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childA-1782172133193230800
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childC-1782172133193230800
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childD-1782172133192717700
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childA
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childC
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childA
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childG
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childE
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childE botUser=test#
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childE concurrency_budget=8
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childG botUser=test#
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childE globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childG concurrency_budget=8
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childE
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childG globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childG
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childG
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childG-1782172133194317200
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childA
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childJ
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childE
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childJ botUser=test#
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childE-1782172133194317200
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childG
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childJ concurrency_budget=8
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childJ globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childC
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childJ
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childA-1782172133193230800
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childJ-1782172133195498300
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childC
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childE
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childE
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childJ
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childC-1782172133193230800
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childJ
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childE-1782172133194317200
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childJ
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childJ-1782172133195498300
2026/06/22 20:48:53 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=childI
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childG
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childG-1782172133194317200
2026/06/22 20:48:53 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=childI botUser=test#
2026/06/22 20:48:53 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=childI concurrency_budget=8
2026/06/22 20:48:53 INFO Architectural state transition: Configured runtime task router budget botInstanceID=childI globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:53 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=childI
2026/06/22 20:48:53 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:53 INFO Primary instance coupling and socket registration completed successfully botInstanceID=childI
2026/06/22 20:48:53 INFO Starting service... service=bot-runtime-childI-1782172133197593400
2026/06/22 20:48:53 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:53 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=childI
2026/06/22 20:48:53 INFO Starting all services...
2026/06/22 20:48:53 INFO Starting service... service=command-handler
2026/06/22 20:48:53 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:53 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:53 INFO Service started successfully service=command-handler
2026/06/22 20:48:53 INFO All services started successfully services_count=1
2026/06/22 20:48:53 INFO Runtime do bot totalmente operacional botInstanceID=childI
2026/06/22 20:48:53 INFO Service started successfully service=bot-runtime-childI-1782172133197593400
2026/06/22 20:48:53 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childJ-1782172133195498300
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childI-1782172133197593400
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childG-1782172133194317200
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childE-1782172133194317200
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childH-1782172133191598200
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childG
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childI
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childE
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childA-1782172133193230800
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childJ
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childF-1782172133191598200
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childB-1782172133192717700
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childF
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childI-1782172133197593400
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childB
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childI
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childH
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childC-1782172133193230800
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Stopping service... service=bot-runtime-childD-1782172133192717700
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childC
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childA
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childE-1782172133194317200
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=childD
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childG-1782172133194317200
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childH-1782172133191598200
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childB-1782172133192717700
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childF-1782172133191598200
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childE
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childJ-1782172133195498300
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childG
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Stopping all services...
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childJ
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childA-1782172133193230800
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO Stopping service... service=command-handler
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childC-1782172133193230800
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childH
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childB
2026/06/22 20:48:53 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childF
2026/06/22 20:48:53 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:53 INFO All services stopped successfully
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childA
2026/06/22 20:48:53 INFO Service stopped service=bot-runtime-childD-1782172133192717700
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childC
2026/06/22 20:48:53 INFO executeStopAndRemove DELETING instance id=childD
2026/06/22 20:48:53 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:53 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:53 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:54 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:54 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:54 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:54 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:54 WARN Handshake failure with guild interface reported by central hub guildID=g1
--- PASS: TestSupervisorSwarmTopology (1.23s)
=== RUN   TestSupervisorConfigChange
2026/06/22 20:48:54 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/22 20:48:54 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:54 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/22 20:48:54 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/22 20:48:54 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:54 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=child1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/22 20:48:54 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=child1
2026/06/22 20:48:54 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-child1-1782172134413276600
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=child1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Starting service... service=command-handler
2026/06/22 20:48:54 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:54 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:54 INFO Service started successfully service=command-handler
2026/06/22 20:48:54 INFO All services started successfully services_count=1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=child1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-child1-1782172134413276600
2026/06/22 20:48:54 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:54 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=1
2026/06/22 20:48:54 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:54 INFO Planned instance shutdown triggered by token update botInstanceID=child1
2026/06/22 20:48:54 INFO Stopping service... service=bot-runtime-child1-1782172134413276600
2026/06/22 20:48:54 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=child1
2026/06/22 20:48:54 INFO Stopping all services...
2026/06/22 20:48:54 INFO Stopping service... service=command-handler
2026/06/22 20:48:54 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:54 INFO All services stopped successfully
2026/06/22 20:48:54 INFO Service stopped service=bot-runtime-child1-1782172134413276600
2026/06/22 20:48:54 INFO executeStopAndRemove DELETING instance id=child1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=child1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=child1 botUser=test#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=child1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=child1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=child1
2026/06/22 20:48:54 INFO Service registered service=command-handler type=commands priority=5 dependencies=[]
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=child1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-child1-1782172134428920400
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=child1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Starting service... service=command-handler
2026/06/22 20:48:54 INFO Starting primary routine of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:54 WARN command synchronization failed at initialization; operating in degraded state botInstanceID="" err="cannot setup commands: session user state is missing"
2026/06/22 20:48:54 INFO Service started successfully service=command-handler
2026/06/22 20:48:54 INFO All services started successfully services_count=1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=child1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-child1-1782172134428920400
2026/06/22 20:48:54 INFO Load summary initialized guilds_count=1
2026/06/22 20:48:54 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/22 20:48:54 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/22 20:48:54 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:54 INFO Planned instance shutdown triggered by token removal botInstanceID=child1
2026/06/22 20:48:54 INFO Stopping service... service=bot-runtime-child1-1782172134428920400
2026/06/22 20:48:54 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=child1
2026/06/22 20:48:54 INFO Stopping all services...
2026/06/22 20:48:54 INFO Stopping service... service=command-handler
2026/06/22 20:48:54 INFO Stopping main instances of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Service stopped successfully service=command-handler
2026/06/22 20:48:54 INFO All services stopped successfully
2026/06/22 20:48:54 INFO Service stopped service=bot-runtime-child1-1782172134428920400
2026/06/22 20:48:54 INFO executeStopAndRemove DELETING instance id=child1
2026/06/22 20:48:54 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/22 20:48:54 WARN Handshake failure with guild interface reported by central hub guildID=g1
2026/06/22 20:48:54 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/22 20:48:54 WARN Handshake failure with guild interface reported by central hub guildID=g1
--- PASS: TestSupervisorConfigChange (0.19s)
=== RUN   TestBotSupervisor_ConcurrentConfigThrashing
2026/06/22 20:48:54 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/22 20:48:54 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/22 20:48:54 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/22 20:48:54 WARN Applied configuration does not contain active guilds. Running in basal mode. path=memory://bot_config_state
2026/06/22 20:48:54 INFO Structural state transition completed: Guild index rebuilt reason=apply guilds_count=0
2026/06/22 20:48:54 INFO Configuration state transition completed duplicates_removed=0
2026/06/22 20:48:54 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:54 INFO Initializing primary routines of BotSupervisor component=BotSupervisor
2026/06/22 20:48:54 INFO Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines
2026/06/22 20:48:54 INFO Planned instance shutdown triggered by token update botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134616567300
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Planned instance shutdown triggered by token update botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134616567300
2026/06/22 20:48:54 INFO executeStopAndRemove DELETING instance id=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134618887000
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134618887000
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134619419300
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134619419300
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134619943100
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134619943100
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134620463300
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134620463300
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134621018200
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134621018200
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134621018200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134621541600
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134621541600
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134621541600"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134622082200
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134622082200
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134622646800
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134622646800
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134622646800"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134623175800
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134623175800
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134623175800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134623707600
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134623707600
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134623707600"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134624220000
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134624220000
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134624220000"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134624748700
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134624748700
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134624748700"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134625263500
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134625263500"
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134625263500
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134625784900
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134625784900
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134625784900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134626303300
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134626303300
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134626303300"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134626817200
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134626817200
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134626817200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134627335900
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134627335900
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134627335900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134627849900
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134627849900
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134627849900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134630827500
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134630827500
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134636686100
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134636686100
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134641681100
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134641681100
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134641681100"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134642409700
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134642409700
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134642409700"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134642978800
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134642978800
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134643545000
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134643545000
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134643545000"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134644069900
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134644069900
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134644586500
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134644586500
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134644586500"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134645201200
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134645201200
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134645747300
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134645747300
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134645747300"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134646240600
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134646240600
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134646861500
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134646861500
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134647380200
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134647380200
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134647898100
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134647898100
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134647898100"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134648422500
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134648422500
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134648934600
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134648934600
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134648934600"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134649447600
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134649447600
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134649447600"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134651008200
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134651008200
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134651008200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134651008200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134651008200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134651008200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134651008200"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134652519800
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134652519800
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134652519800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134652519800"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134652519800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134653534800
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134653534800
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134653534800"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134655545900
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134655545900
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 INFO executeStopAndRemove SKIPPING deletion: pointer mismatch id=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134655545900"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134657558100
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134657558100
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134657558100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134658071000
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134658071000
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658071000"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134658582100
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134658582100
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Stopping service... service=bot-runtime-instance_1-1782172134658582100
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state transition: Executing planned shutdown across main runtime instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Stopping all services...
2026/06/22 20:48:54 INFO All services stopped successfully
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service stopped service=bot-runtime-instance_1-1782172134658582100
2026/06/22 20:48:54 INFO executeStopAndRemove DELETING instance id=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134658582100
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134658582100
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 ERROR Failure in coupling interface with dynamic Service Manager request_id=register_svc_instance_1 botInstanceID=instance_1 error="service already registered: bot-runtime-instance_1-1782172134658582100"
2026/06/22 20:48:54 WARN Basal threshold reached: Empty guild allocation vector in boot routine
2026/06/22 20:48:54 INFO Architectural state transition: Initializing primary Discord API routine botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Socket bound and API authenticated botInstanceID=instance_1 botUser=stress_test_bot#
2026/06/22 20:48:54 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=instance_1 concurrency_budget=8
2026/06/22 20:48:54 INFO Architectural state transition: Configured runtime task router budget botInstanceID=instance_1 globalMaxWorkers=8 sharedLimiter=true
2026/06/22 20:48:54 INFO Architectural state bypass: Automod service skipped due to explicit capability flags botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state bypass: Commands skipped due to empty guild bindings botInstanceID=instance_1
2026/06/22 20:48:54 INFO Primary instance coupling and socket registration completed successfully botInstanceID=instance_1
2026/06/22 20:48:54 INFO Starting service... service=bot-runtime-instance_1-1782172134661133700
2026/06/22 20:48:54 INFO Architectural state transition: Executing StartAll across service manager instances botInstanceID=instance_1
2026/06/22 20:48:54 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/22 20:48:54 INFO Starting all services...
2026/06/22 20:48:54 INFO All services started successfully services_count=0
2026/06/22 20:48:54 INFO Runtime do bot totalmente operacional botInstanceID=instance_1
2026/06/22 20:48:54 INFO Service started successfully service=bot-runtime-instance_1-1782172134661133700
--- PASS: TestBotSupervisor_ConcurrentConfigThrashing (0.05s)
=== RUN   TestBotSupervisor_GracefulShutdownOrchestration
2026/06/22 20:48:54 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
2026/06/22 20:48:54 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
2026/06/22 20:48:54 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/22 20:48:54 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:54 INFO Triggering planned shutdown of main BotSupervisor instances
2026/06/22 20:48:54 INFO executeStopAndRemove DELETING instance id=zombie_instance
2026/06/22 20:48:54 ERROR BotSupervisor stop timeout exceeded before background task completion request_id=supervisor_shutdown error="context deadline exceeded"
2026/06/22 20:48:54 ERROR Failed to purge I/O, escalated to ForceRemove request_id=stop_remove_zombie_instance botInstanceID=zombie_instance error="stop signal failed for bot-runtime-zombie_instance-0: context deadline exceeded"
2026/06/22 20:48:54 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestBotSupervisor_GracefulShutdownOrchestration (0.05s)
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
2026/06/22 20:48:54 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=runtime
2026/06/22 20:48:54 INFO Architectural state transition: Primary routines initialization component=PartnerCommands
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=partner
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=ban
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=timeout
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=massban
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=reaction_block
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=clean
2026/06/22 20:48:54 INFO Architectural state transition: Primary routines initialization component=RolePanelCommands
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=roles
2026/06/22 20:48:54 INFO Architectural state transition: Primary routines initialization component=EmbedCommands
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=embed
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=qotd
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=logging
2026/06/22 20:48:54 INFO Command catalog fragments coupled to the native Arikawa router
2026/06/22 20:48:54 INFO Successfully synchronized commands via BulkOverwrite guild_id="" total_commands=11
2026/06/22 20:48:54 INFO Command architecture successfully established natively botInstanceID=""
2026/06/22 20:48:54 INFO Starting command and route coupling botInstanceID=""
2026/06/22 20:48:54 WARN overlapping handler registration; invoking cleanup of previous registrations botInstanceID=""
2026/06/22 20:48:54 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=runtime
2026/06/22 20:48:54 INFO Architectural state transition: Primary routines initialization component=PartnerCommands
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=partner
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=ban
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=timeout
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=massban
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=reaction_block
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=clean
2026/06/22 20:48:54 INFO Architectural state transition: Primary routines initialization component=RolePanelCommands
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=roles
2026/06/22 20:48:54 INFO Architectural state transition: Primary routines initialization component=EmbedCommands
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=embed
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=qotd
2026/06/22 20:48:54 INFO Architectural state transition: Registering native command command_name=logging
2026/06/22 20:48:54 INFO Command catalog fragments coupled to the native Arikawa router
2026/06/22 20:48:54 INFO Successfully synchronized commands via BulkOverwrite guild_id="" total_commands=11
2026/06/22 20:48:54 INFO Command architecture successfully established natively botInstanceID=""
2026/06/22 20:48:54 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
2026/06/22 20:48:54 INFO Starting connection drain and shutdown of CommandHandler botInstanceID=""
--- PASS: TestCommandHandlerSetupAndShutdownLifecycle (0.06s)
=== RUN   TestCommandHandlerSetupRollbackOnManagerFailure
2026/06/22 20:48:54 INFO Starting command and route coupling botInstanceID=""
--- PASS: TestCommandHandlerSetupRollbackOnManagerFailure (0.00s)
=== RUN   TestCommandHandlerSkipsGuildWithoutCommandsFeature
2026/06/22 20:48:54 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/22 20:48:54 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
--- PASS: TestCommandHandlerSkipsGuildWithoutCommandsFeature (0.00s)
=== RUN   TestCommandHandlerRoutesFeaturesToCorrectBotInstance
2026/06/22 20:48:54 INFO Structural state transition completed: Guild index rebuilt reason=update guilds_count=1
2026/06/22 20:48:54 INFO I/O state transition: Configuration successfully persisted path=memory://bot_config_state
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
2026/06/22 20:48:54 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
2026/06/22 20:48:54 INFO Architectural state transition: Initiating ad-hoc generation of local TLS credentials for control plane binding
--- PASS: TestResolveControlRuntimeUsesManagedLocalHTTPS (0.13s)
=== RUN   TestResolveControlRuntimeDerivesOAuthRedirectFromPublicOrigin
2026/06/22 20:48:54 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
--- PASS: TestResolveControlRuntimeDerivesOAuthRedirectFromPublicOrigin (0.00s)
=== RUN   TestArikawaQOTDPublisher_GetArikawaPublisher
=== PAUSE TestArikawaQOTDPublisher_GetArikawaPublisher
=== RUN   TestArikawaQOTDPublisher_PublishOfficialPost
=== PAUSE TestArikawaQOTDPublisher_PublishOfficialPost
=== RUN   TestArikawaQOTDPublisher_DeleteOfficialPost
=== PAUSE TestArikawaQOTDPublisher_DeleteOfficialPost
=== RUN   TestNotifyLifecycleEventSendsWebhook
2026/06/22 20:48:54 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=starting
2026/06/22 20:48:54 INFO Architectural state transition: Lifecycle webhook notification transmitted successfully reason=starting
2026/06/22 20:48:54 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=fatal
2026/06/22 20:48:54 INFO Architectural state transition: Lifecycle webhook notification transmitted successfully reason=fatal
--- PASS: TestNotifyLifecycleEventSendsWebhook (0.00s)
=== RUN   TestBuildLifecycleContentFormat
--- PASS: TestBuildLifecycleContentFormat (0.00s)
=== RUN   TestBuildLifecycleContentFallsBackWhenIdentityUnset
--- PASS: TestBuildLifecycleContentFallsBackWhenIdentityUnset (0.00s)
=== RUN   TestNotifyLifecycleEventHandles5xx
2026/06/22 20:48:54 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=fatal
2026/06/22 20:48:54 WARN Mitigated service degradation: Discord upstream rejected lifecycle webhook payload operation=lifecycle.webhook reason=fatal status_code=500 retry_after=0
--- PASS: TestNotifyLifecycleEventHandles5xx (0.00s)
=== RUN   TestNotifyLifecycleEventTimeoutContext
2026/06/22 20:48:54 INFO Architectural state transition: Initiating out-of-band lifecycle notification sequence reason=stopping
2026/06/22 20:48:57 WARN Mitigated service degradation: External webhook endpoint unreachable; timeout or DNS failure operation=lifecycle.webhook reason=stopping error="Post \"http://127.0.0.1:65045\": context deadline exceeded"
--- PASS: TestNotifyLifecycleEventTimeoutContext (5.00s)
=== RUN   TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild
=== PAUSE TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild
=== RUN   TestCollectStartupWebhookEmbedUpdatesNilConfig
=== PAUSE TestCollectStartupWebhookEmbedUpdatesNilConfig
=== RUN   TestRun_MissingDatabaseURL
2026/06/22 20:48:59 INFO Architectural state transition: Executing application boot sequence
2026/06/22 20:48:59 INFO Architectural state transition: Executing application binary version_info="🚀 Starting testapp (discordcore v0.840.0-rc.4)..."
2026/06/22 20:48:59 ERROR Blocking structural failure: Database bootstrap configuration unavailable request_id=3e06330f6c3ed299fb6d2b35bfc649be synthetic_code=500 stack_trace="goroutine 982 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0f925, 0x49}, {0x140f78620, 0xc000136190}, {0xc0000361e0, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.resolveDatabaseBootstrap()\n\tD:/Users/alice/git/discordcore/pkg/app/startup.go:54 +0x1cc\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).InitializeIO(0xc0000fc000, {0x140f84798?, 0x14165c380?})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:191 +0x2d3\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).Boot(0xc0000fc000, {0x140f84798, 0x14165c380})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:133 +0xab\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions({0x140dd5e4a, 0x7}, {{0x0, 0x0}, {{0x0, 0x0}, {0x0, 0x0}, {0x0, 0x0}}, ...})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:122 +0x177\ngithub.com/small-frappuccino/discordcore/pkg/app.Run(...)\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:96\ngithub.com/small-frappuccino/discordcore/pkg/app.TestRun_MissingDatabaseURL(0xc000003800)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:24 +0x99\ntesting.tRunner(0xc000003800, 0x140f6eaa0)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
2026/06/22 20:48:59 ERROR Structural dependency failure: Database manifest resolution aborted request_id=7bfe69d3bffcaab668a8faa10b1f1bf1 synthetic_code=500 stack_trace="goroutine 982 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0d691, 0x43}, {0x140f78c20, 0xc0005160c0}, {0xc000036200, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).InitializeIO(0xc0000fc000, {0x140f84798?, 0x14165c380?})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:194 +0x485\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).Boot(0xc0000fc000, {0x140f84798, 0x14165c380})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:133 +0xab\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions({0x140dd5e4a, 0x7}, {{0x0, 0x0}, {{0x0, 0x0}, {0x0, 0x0}, {0x0, 0x0}}, ...})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:122 +0x177\ngithub.com/small-frappuccino/discordcore/pkg/app.Run(...)\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:96\ngithub.com/small-frappuccino/discordcore/pkg/app.TestRun_MissingDatabaseURL(0xc000003800)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:24 +0x99\ntesting.tRunner(0xc000003800, 0x140f6eaa0)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="InitializeIO resolveDatabaseBootstrap: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
2026/06/22 20:48:59 INFO Architectural state transition: Commencing teardown sequence across local orchestrators app_name=testapp
2026/06/22 20:48:59 INFO Stopping all services...
2026/06/22 20:48:59 INFO All services stopped successfully
2026/06/22 20:48:59 ERROR Critical pipeline failure: Primary routine aborted request_id=bc059d5dc467c6e4a4b5580ba8169f17 synthetic_code=500 stack_trace="goroutine 982 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e035d9, 0x32}, {0x140f78c20, 0xc0005160c0}, {0xc000036320, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions.func2()\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:112 +0x1f5\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions({0x140dd5e4a, 0x7}, {{0x0, 0x0}, {{0x0, 0x0}, {0x0, 0x0}, {0x0, 0x0}}, ...})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:127 +0x1a5\ngithub.com/small-frappuccino/discordcore/pkg/app.Run(...)\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:96\ngithub.com/small-frappuccino/discordcore/pkg/app.TestRun_MissingDatabaseURL(0xc000003800)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:24 +0x99\ntesting.tRunner(0xc000003800, 0x140f6eaa0)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="InitializeIO resolveDatabaseBootstrap: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
--- PASS: TestRun_MissingDatabaseURL (0.00s)
=== RUN   TestRunWithOptions_MissingDatabaseURL
2026/06/22 20:48:59 INFO Architectural state transition: Executing application boot sequence
2026/06/22 20:48:59 INFO Architectural state transition: Executing application binary version_info="🚀 Starting testapp (discordcore v0.840.0-rc.4)..."
2026/06/22 20:48:59 ERROR Blocking structural failure: Database bootstrap configuration unavailable request_id=fc3ac92f83dbf00a15348b85128e242d synthetic_code=500 stack_trace="goroutine 983 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0f925, 0x49}, {0x140f78620, 0xc000136240}, {0xc000036500, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.resolveDatabaseBootstrap()\n\tD:/Users/alice/git/discordcore/pkg/app/startup.go:54 +0x1cc\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).InitializeIO(0xc0000fc140, {0x140f84798?, 0x14165c380?})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:191 +0x2d3\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).Boot(0xc0000fc140, {0x140f84798, 0x14165c380})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:133 +0xab\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions({0x140dd5e4a, 0x7}, {{0x0, 0x0}, {{0x0, 0x0}, {0x0, 0x0}, {0x0, 0x0}}, ...})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:122 +0x177\ngithub.com/small-frappuccino/discordcore/pkg/app.TestRunWithOptions_MissingDatabaseURL(0xc000003a00)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:32 +0x98\ntesting.tRunner(0xc000003a00, 0x140f6ea90)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
2026/06/22 20:48:59 ERROR Structural dependency failure: Database manifest resolution aborted request_id=9fef99f10bf9df0fa0e81bd5b2fc7070 synthetic_code=500 stack_trace="goroutine 983 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0d691, 0x43}, {0x140f78c20, 0xc000516120}, {0xc000036520, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).InitializeIO(0xc0000fc140, {0x140f84798?, 0x14165c380?})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:194 +0x485\ngithub.com/small-frappuccino/discordcore/pkg/app.(*App).Boot(0xc0000fc140, {0x140f84798, 0x14165c380})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:133 +0xab\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions({0x140dd5e4a, 0x7}, {{0x0, 0x0}, {{0x0, 0x0}, {0x0, 0x0}, {0x0, 0x0}}, ...})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:122 +0x177\ngithub.com/small-frappuccino/discordcore/pkg/app.TestRunWithOptions_MissingDatabaseURL(0xc000003a00)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:32 +0x98\ntesting.tRunner(0xc000003a00, 0x140f6ea90)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="InitializeIO resolveDatabaseBootstrap: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
2026/06/22 20:48:59 INFO Architectural state transition: Commencing teardown sequence across local orchestrators app_name=testapp
2026/06/22 20:48:59 INFO Stopping all services...
2026/06/22 20:48:59 INFO All services stopped successfully
2026/06/22 20:48:59 ERROR Critical pipeline failure: Primary routine aborted request_id=884ee1a1d8af91c95d05400d67fe32b4 synthetic_code=500 stack_trace="goroutine 983 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e035d9, 0x32}, {0x140f78c20, 0xc000516120}, {0xc000036560, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions.func2()\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:112 +0x1f5\ngithub.com/small-frappuccino/discordcore/pkg/app.RunWithOptions({0x140dd5e4a, 0x7}, {{0x0, 0x0}, {{0x0, 0x0}, {0x0, 0x0}, {0x0, 0x0}}, ...})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:127 +0x1a5\ngithub.com/small-frappuccino/discordcore/pkg/app.TestRunWithOptions_MissingDatabaseURL(0xc000003a00)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:32 +0x98\ntesting.tRunner(0xc000003a00, 0x140f6ea90)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="InitializeIO resolveDatabaseBootstrap: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
--- PASS: TestRunWithOptions_MissingDatabaseURL (0.00s)
=== RUN   TestSetupStorage
2026/06/22 20:48:59 ERROR Structural dependency failure: Core socket driver rejected host request_id=bfae88b3f77a71dcf2c83bbcac9e6a00 synthetic_code=500 stack_trace="goroutine 984 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0b73c, 0x3f}, {0x140f78c20, 0xc0005161a0}, {0xc000036580, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.setupStorage({{{0x0, 0x0}, {0x0, 0x0}, 0x0, 0x0, 0x0, 0x0, 0x0}, {0x0, ...}})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:559 +0x2ec\ngithub.com/small-frappuccino/discordcore/pkg/app.TestSetupStorage(0xc000003c00)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:40 +0x76\ntesting.tRunner(0xc000003c00, 0x140f6eae8)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="open postgres database: Open: database driver is required (expected: postgres)"
2026/06/22 20:48:59 ERROR Structural dependency failure: Core socket driver rejected host request_id=c2ee44db4ff0521cffa27e244ac0f214 synthetic_code=500 stack_trace="goroutine 984 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0b73c, 0x3f}, {0x140f78c20, 0xc000516800}, {0xc000037800, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.setupStorage({{{0x140dd6738, 0x8}, {0x140e0c6b1, 0x41}, 0x0, 0x0, 0x0, 0x0, 0x0}, {0x0, ...}})\n\tD:/Users/alice/git/discordcore/pkg/app/runner.go:559 +0x2ec\ngithub.com/small-frappuccino/discordcore/pkg/app.TestSetupStorage(0xc000003c00)\n\tD:/Users/alice/git/discordcore/pkg/app/runner_test.go:47 +0x13f\ntesting.tRunner(0xc000003c00, 0x140f6eae8)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="open postgres database: ping postgres connection: database ping failed: failed to connect to `user=username database=bogus`: 127.0.0.1:5433 (127.0.0.1): dial error: dial tcp 127.0.0.1:5433: connectex: No connection could be made because the target machine actively refused it."
--- PASS: TestSetupStorage (0.01s)
=== RUN   TestRunner_ShutdownStartupServices
--- PASS: TestRunner_ShutdownStartupServices (0.00s)
=== RUN   TestRunner_ResolveRuntimeCapabilities
--- PASS: TestRunner_ResolveRuntimeCapabilities (0.00s)
=== RUN   TestRunner_ApplyConfiguredTheme
2026/06/22 20:48:59 INFO Architectural state transition: Standard UI theme locked
--- PASS: TestRunner_ApplyConfiguredTheme (0.00s)
=== RUN   TestRunner_ScheduleDBCleanup
2026/06/22 20:48:59 INFO Architectural state transition: Initializing persistent cache garbage collector
--- PASS: TestRunner_ScheduleDBCleanup (0.00s)
=== RUN   TestFormatStartupMessage
=== PAUSE TestFormatStartupMessage
=== RUN   TestRun_CascadingRollbackFailures
    runner_test.go:160: Skipping test: database URL not configured
--- SKIP: TestRun_CascadingRollbackFailures (0.00s)
=== RUN   TestRun_ResourceCleanupOnBootFailure
    runner_test.go:160: Skipping test: database URL not configured
--- SKIP: TestRun_ResourceCleanupOnBootFailure (0.00s)
=== RUN   TestResolveDatabaseBootstrapFromEnv
--- PASS: TestResolveDatabaseBootstrapFromEnv (0.00s)
=== RUN   TestResolveDatabaseBootstrapRequiresEnv
2026/06/22 20:48:59 ERROR Blocking structural failure: Database bootstrap configuration unavailable request_id=ee9ef214701884582f287f0e5704779d synthetic_code=500 stack_trace="goroutine 1013 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0f925, 0x49}, {0x140f78620, 0xc000136a80}, {0xc000037ac0, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.resolveDatabaseBootstrap()\n\tD:/Users/alice/git/discordcore/pkg/app/startup.go:54 +0x1cc\ngithub.com/small-frappuccino/discordcore/pkg/app.TestResolveDatabaseBootstrapRequiresEnv(0xc0005b7200)\n\tD:/Users/alice/git/discordcore/pkg/app/startup_test.go:53 +0x10a\ntesting.tRunner(0xc0005b7200, 0x140f6ea70)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup"
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
=== CONT  TestResolveParallelism
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity
=== CONT  TestBotRuntime_CapabilityBitmaskDerivation
=== CONT  TestStartupTaskOrchestrator_GoNil
=== RUN   TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation
2026/06/22 20:48:59 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
=== CONT  TestArikawaQOTDPublisher_DeleteOfficialPost
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
=== CONT  TestRuntimeCommandCatalogRegistrar_FailFastBarrier
2026/06/22 20:48:59 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:59 INFO Architectural state transition: Allocating stateless native Arikawa publisher orchestrator
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
=== CONT  TestArikawaQOTDPublisher_GetArikawaPublisher
=== CONT  TestArikawaQOTDPublisher_PublishOfficialPost
2026/06/22 20:48:59 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
=== RUN   TestResolveParallelism/RuntimeStartup
2026/06/22 20:48:59 INFO Architectural state transition: Allocating stateless native Arikawa publisher orchestrator
2026/06/22 20:48:59 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evalua_como_verdadeiro_contra_qualquer_máscara_de_base
2026/06/22 20:48:59 INFO Architectural state transition: Allocating stateless native Arikawa publisher orchestrator
=== CONT  TestStartupTaskOrchestrator_GoLight
=== CONT  TestFormatStartupMessage
2026/06/22 20:48:59 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
=== PAUSE TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation
=== RUN   TestFormatStartupMessage/no_app_version_includes_discordcore
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
=== PAUSE TestFormatStartupMessage/no_app_version_includes_discordcore
--- PASS: TestRuntimeCommandCatalogRegistrar_FailFastBarrier (0.00s)
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
=== CONT  TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed
--- PASS: TestStartupTaskOrchestrator_GoNil (0.00s)
2026/06/22 20:48:59 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
=== CONT  TestStartupTaskOrchestrator_GoHeavy
2026/06/22 20:48:59 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
=== CONT  TestStartupTaskOrchestrator_ShutdownWithContextCancellation
2026/06/22 20:48:59 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=2
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
2026/06/22 20:48:59 INFO Architectural state transition: Startup task orchestrator instantiated runtime_count_heuristic=1
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evalua_como_verdadeiro_contra_qualquer_máscara_de_base
=== CONT  TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=4
=== RUN   TestFormatStartupMessage/different_versions_include_both
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=3 queue_capacity=6
--- PASS: TestArikawaQOTDPublisher_DeleteOfficialPost (0.00s)
2026/06/22 20:48:59 WARN Mitigated service degradation: Background startup task encountered an error and aborted task=error_task kind=heavy error="simulated task error"
2026/06/22 20:48:59 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
=== CONT  TestScheduleStartupWebhookEmbedUpdates
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=1 queue_capacity=1
=== CONT  TestCollectStartupWebhookEmbedUpdatesNilConfig
2026/06/22 20:48:59 INFO Architectural state transition: Background worker pool initialized parallelism_limit=2 queue_capacity=2
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_vazia_rejeita_qualquer_capacidade_específica
2026/06/22 20:48:59 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
=== PAUSE TestFormatStartupMessage/different_versions_include_both
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_vazia_rejeita_qualquer_capacidade_específica
2026/06/22 20:48:59 INFO Architectural state transition: Halting startup orchestrator and draining worker pools
--- PASS: TestArikawaQOTDPublisher_GetArikawaPublisher (0.00s)
2026/06/22 20:48:59 WARN Mitigated service degradation: Context deadline exceeded while awaiting worker pool drain error="context canceled"
=== CONT  TestCatalogRegistrars_Capabilities
=== RUN   TestCatalogRegistrars_Capabilities/Moderation_Capabilities
=== CONT  TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride
=== PAUSE TestCatalogRegistrars_Capabilities/Moderation_Capabilities
=== CONT  TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets
=== CONT  TestStartControlServerStartupTask
=== CONT  TestControlServerHolder_SetAndStop
=== CONT  TestCatalogRegistrars_DIFailures
2026/06/22 20:48:59 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=0
2026/06/22 20:48:59 ERROR Blocking structural failure: Light worker pool failed to terminate cleanly request_id=9f9b42dd9e7fc15c91a132ef66b70bbe synthetic_code=500 stack_trace="goroutine 1016 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0fdb7, 0x4a}, {0x140f78c20, 0xc000194060}, {0xc00030a080, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.(*StartupTaskOrchestrator).Shutdown(0xc00011e0e0, {0x140f849a0, 0xc00056e190})\n\tD:/Users/alice/git/discordcore/pkg/app/startup.go:607 +0x21a\ngithub.com/small-frappuccino/discordcore/pkg/app.TestStartupTaskOrchestrator_ShutdownWithContextCancellation(0xc0005b7800)\n\tD:/Users/alice/git/discordcore/pkg/app/startup_test.go:136 +0x198\ntesting.tRunner(0xc0005b7800, 0x140f6eb18)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="shutdown light startup tasks: context canceled"
=== RUN   TestFormatStartupMessage/same_versions_omit_discordcore_suffix
--- PASS: TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed (0.00s)
2026/06/22 20:48:59 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_singular
2026/06/22 20:48:59 WARN Mitigated service degradation: Context deadline exceeded while awaiting worker pool drain error="context canceled"
=== RUN   TestCatalogRegistrars_Capabilities/Stats_Capabilities
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_singular
2026/06/22 20:48:59 INFO Architectural transition: Binding control server socket address=127.0.0.1:0
=== CONT  TestScheduleControlServerStartup
2026/06/22 20:48:59 INFO Architectural state transition: Initializing primary HTTP control plane bind_addr=127.0.0.1:0
=== RUN   TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager
2026/06/22 20:48:59 INFO Architectural transition: Control server startup bypassed via explicit run options
=== PAUSE TestFormatStartupMessage/same_versions_omit_discordcore_suffix
2026/06/22 20:48:59 INFO Architectural state transition: Instantiating resolution pipeline for control plane bindings
2026/06/22 20:48:59 ERROR Blocking structural failure: Heavy worker pool failed to terminate cleanly request_id=5180ef2593757ece4a5cc361c57b3578 synthetic_code=500 stack_trace="goroutine 1016 [running]:\nruntime/debug.Stack()\n\tD:/Users/alice/scoop/apps/go/current/src/runtime/debug/stack.go:26 +0x68\ngithub.com/small-frappuccino/discordcore/pkg/log.EmitBlockingError({0x140e0fe01, 0x4a}, {0x140f78c20, 0xc000516d40}, {0xc000037ee0, 0x20})\n\tD:/Users/alice/git/discordcore/pkg/log/helpers.go:24 +0x186\ngithub.com/small-frappuccino/discordcore/pkg/app.(*StartupTaskOrchestrator).Shutdown(0xc00011e0e0, {0x140f849a0, 0xc00056e190})\n\tD:/Users/alice/git/discordcore/pkg/app/startup.go:614 +0x47a\ngithub.com/small-frappuccino/discordcore/pkg/app.TestStartupTaskOrchestrator_ShutdownWithContextCancellation(0xc0005b7800)\n\tD:/Users/alice/git/discordcore/pkg/app/startup_test.go:136 +0x198\ntesting.tRunner(0xc0005b7800, 0x140f6eb18)\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2036 +0x1cb\ncreated by testing.(*T).Run in goroutine 1\n\tD:/Users/alice/scoop/apps/go/current/src/testing/testing.go:2101 +0xb2b\n" error="shutdown heavy startup tasks: context canceled"
--- PASS: TestStartupTaskOrchestrator_GoLight (0.00s)
=== RUN   TestFormatStartupMessage/trims_spaces
--- PASS: TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild (0.00s)
2026/06/22 20:48:59 INFO Architectural transition: Control server initializing without authentication middleware addr=127.0.0.1:8376 dashboard_only=true
=== RUN   TestResolveParallelism/RuntimeBackground
2026/06/22 20:48:59 INFO Architectural transition: Binding control server socket address=127.0.0.1:8376
--- PASS: TestScheduleStartupWebhookEmbedUpdates (0.00s)
2026/06/22 20:48:59 INFO Architectural state transition: Initializing primary HTTP control plane bind_addr=127.0.0.1:8376
=== PAUSE TestFormatStartupMessage/trims_spaces
=== PAUSE TestCatalogRegistrars_Capabilities/Stats_Capabilities
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_não_contém_alvo_ausente
=== PAUSE TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager
=== CONT  TestCatalogRegistrars_RegisterArikawa
--- PASS: TestCollectStartupWebhookEmbedUpdatesNilConfig (0.00s)
=== CONT  TestBotRuntimeResolver_ConcurrentMemoryRotation
2026/06/22 20:48:59 INFO Architectural state transition: Initializing memory barrier for bot runtime multiplexing initial_runtimes_count=1
=== CONT  TestNewRuntimeTaskRouterConfigBuildsSharedLimiter
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_não_contém_alvo_ausente
2026/06/22 20:48:59 INFO Architectural state transition: Configured background worker budget for task router botInstanceID=default concurrency_budget=5
=== CONT  TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation
=== CONT  TestFormatStartupMessage/no_app_version_includes_discordcore
--- PASS: TestArikawaQOTDPublisher_PublishOfficialPost (0.00s)
=== RUN   TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring
=== RUN   TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_múltiplo_exato
=== CONT  TestFormatStartupMessage/trims_spaces
=== PAUSE TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_múltiplo_exato
=== CONT  TestFormatStartupMessage/same_versions_omit_discordcore_suffix
=== CONT  TestCatalogRegistrars_Capabilities/Stats_Capabilities
=== CONT  TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager
--- PASS: TestStartupTaskOrchestrator_GoHeavy (0.00s)
=== PAUSE TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring
=== CONT  TestCatalogRegistrars_Capabilities/Moderation_Capabilities
=== CONT  TestFormatStartupMessage/different_versions_include_both
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_vazia_rejeita_qualquer_capacidade_específica
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_múltiplo_exato
--- PASS: TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride (0.00s)
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evalua_como_verdadeiro_contra_qualquer_máscara_de_base
=== RUN   TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_não_contém_alvo_ausente
=== PAUSE TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring
--- PASS: TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets (0.00s)
--- PASS: TestControlServerHolder_SetAndStop (0.00s)
--- PASS: TestStartupTaskOrchestrator_ShutdownWithContextCancellation (0.00s)
=== CONT  TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_singular
=== CONT  TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring
=== RUN   TestResolveParallelism/StartupLight
=== CONT  TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring
--- PASS: TestNewRuntimeTaskRouterConfigBuildsSharedLimiter (0.00s)
=== RUN   TestResolveParallelism/StartupLightQueue
--- PASS: TestBotRuntime_CapabilityBitmaskDerivation (0.00s)
    --- PASS: TestBotRuntime_CapabilityBitmaskDerivation/Commands_and_Moderation_Escalation (0.00s)
--- PASS: TestCatalogRegistrars_RegisterArikawa (0.00s)
    --- PASS: TestCatalogRegistrars_RegisterArikawa/Stats_Catalog_Wiring (0.00s)
    --- PASS: TestCatalogRegistrars_RegisterArikawa/Moderation_Catalog_Wiring (0.00s)
--- PASS: TestFormatStartupMessage (0.00s)
    --- PASS: TestFormatStartupMessage/no_app_version_includes_discordcore (0.00s)
    --- PASS: TestFormatStartupMessage/trims_spaces (0.00s)
    --- PASS: TestFormatStartupMessage/same_versions_omit_discordcore_suffix (0.00s)
    --- PASS: TestFormatStartupMessage/different_versions_include_both (0.00s)
--- PASS: TestCatalogRegistrars_DIFailures (0.00s)
    --- PASS: TestCatalogRegistrars_DIFailures/StatsRegistrar_Requires_ConfigManager (0.00s)
--- PASS: TestCatalogRegistrars_Capabilities (0.00s)
    --- PASS: TestCatalogRegistrars_Capabilities/Stats_Capabilities (0.00s)
    --- PASS: TestCatalogRegistrars_Capabilities/Moderation_Capabilities (0.00s)
--- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_vazia_rejeita_qualquer_capacidade_específica (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_múltiplo_exato (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/CapNone_evalua_como_verdadeiro_contra_qualquer_máscara_de_base (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_não_contém_alvo_ausente (0.00s)
    --- PASS: TestCommandCatalogCapabilities_BitmaskIntegrity/Máscara_composta_contém_alvo_singular (0.00s)
--- PASS: TestResolveParallelism (0.00s)
    --- PASS: TestResolveParallelism/RuntimeStartup (0.00s)
    --- PASS: TestResolveParallelism/RuntimeBackground (0.00s)
    --- PASS: TestResolveParallelism/StartupLight (0.00s)
    --- PASS: TestResolveParallelism/StartupLightQueue (0.00s)
--- PASS: TestStartControlServerStartupTask (0.00s)
--- PASS: TestScheduleControlServerStartup (0.01s)
--- PASS: TestBotRuntimeResolver_ConcurrentMemoryRotation (0.50s)
PASS
ok      github.com/small-frappuccino/discordcore/pkg/app        9.289s