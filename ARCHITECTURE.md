# Discordcore Architecture

This document provides a high-level overview of the `discordcore` system architecture, its dependencies, and how data flows across the various packages and layers.

## System Map

```mermaid
flowchart TD
    %% External APIs & Gateways (Top Level)
    DiscordGateway((Discord Gateway))
    DiscordAPI((Discord API))

    %% SDKs
    DiscordGo(("DiscordGo SDK"))
    Arikawa(("Arikawa SDK"))

    %% Nodes
    CmdClean-config["cmd/clean-config"]
    CmdDiscordcore["cmd/discordcore"]
    CmdTsgen["cmd/tsgen"]
    App["pkg/app"]
    AppRuntimecmd["pkg/app/runtimecmd"]
    Automod["pkg/automod"]
    AutomodAutomodmocks["pkg/automod/automodmocks"]
    Clean["pkg/clean"]
    Clock["pkg/clock"]
    Control["pkg/control"]
    ControlLocaltls["pkg/control/localtls"]
    Discord["pkg/discord"]
    AdapterAutomod["pkg/discord/automod"]
    AdapterCache["pkg/discord/cache"]
    AdapterClean["pkg/discord/clean"]
    Commands["pkg/discord/commands"]
    CommandsClean["pkg/discord/commands/clean"]
    CommandsCore["pkg/discord/commands/core"]
    CommandsEmbeds["pkg/discord/commands/embeds"]
    CommandsLogging["pkg/discord/commands/logging"]
    CommandsModeration["pkg/discord/commands/moderation"]
    CommandsPartners["pkg/discord/commands/partners"]
    CommandsQotd["pkg/discord/commands/qotd"]
    CommandsRoles["pkg/discord/commands/roles"]
    CommandsRuntime["pkg/discord/commands/runtime"]
    CommandsStats["pkg/discord/commands/stats"]
    CommandsTickets["pkg/discord/commands/tickets"]
    AdapterEmbeds["pkg/discord/embeds"]
    AdapterLogging["pkg/discord/logging"]
    AdapterMembers["pkg/discord/members"]
    AdapterMessages["pkg/discord/messages"]
    AdapterModeration["pkg/discord/moderation"]
    AdapterPartners["pkg/discord/partners"]
    AdapterPerf["pkg/discord/perf"]
    AdapterQotd["pkg/discord/qotd"]
    AdapterRoles["pkg/discord/roles"]
    AdapterSession["pkg/discord/session"]
    AdapterStats["pkg/discord/stats"]
    AdapterTickets["pkg/discord/tickets"]
    AdapterWebhook["pkg/discord/webhook"]
    Files["pkg/files"]
    Idgen["pkg/idgen"]
    Log["pkg/log"]
    Logging["pkg/logging"]
    Members["pkg/members"]
    Messages["pkg/messages"]
    Moderation["pkg/moderation"]
    Observability["pkg/observability"]
    Persistence["pkg/persistence"]
    Qotd["pkg/qotd"]
    Runtimeapply["pkg/runtimeapply"]
    Service["pkg/service"]
    Stats["pkg/stats"]
    StoragePostgres["pkg/storage/postgres"]
    StoragePostgresStoragetest["pkg/storage/postgres/storagetest"]
    System["pkg/system"]
    Task["pkg/task"]
    Testdb["pkg/testdb"]
    Theme["pkg/theme"]
    Tickets["pkg/tickets"]
    UI["ui"]

    %% SDK & API Flow
    DiscordGateway -. WebSocket .-> DiscordGo
    DiscordGo -. REST Calls .-> DiscordAPI
    Arikawa -. REST Calls .-> DiscordAPI

    %% Auto-generated internal dependencies
    AdapterAutomod --> AdapterPerf
    AdapterAutomod --> Arikawa
    AdapterAutomod --> Automod
    AdapterAutomod --> Service
    AdapterCache --> AdapterSession
    AdapterCache --> Arikawa
    AdapterCache --> StoragePostgres
    AdapterClean --> Arikawa
    AdapterClean --> Clean
    AdapterEmbeds --> Arikawa
    AdapterEmbeds --> Files
    AdapterLogging --> AdapterEmbeds
    AdapterLogging --> Arikawa
    AdapterLogging --> Automod
    AdapterLogging --> Files
    AdapterLogging --> Logging
    AdapterLogging --> Members
    AdapterLogging --> Messages
    AdapterLogging --> Theme
    AdapterMembers --> Arikawa
    AdapterMembers --> Members
    AdapterMembers --> Service
    AdapterMessages --> Arikawa
    AdapterMessages --> Messages
    AdapterMessages --> Service
    AdapterModeration --> Arikawa
    AdapterPartners --> Arikawa
    AdapterPartners --> Files
    AdapterPartners --> Theme
    AdapterPerf --> Files
    AdapterPerf --> Log
    AdapterPerf --> Observability
    AdapterQotd --> Arikawa
    AdapterQotd --> Log
    AdapterQotd --> Qotd
    AdapterQotd --> Service
    AdapterRoles --> Arikawa
    AdapterRoles --> Files
    AdapterSession --> DiscordGo
    AdapterSession --> Log
    AdapterStats --> Arikawa
    AdapterStats --> DiscordGo
    AdapterStats --> Stats
    AdapterTickets --> Arikawa
    AdapterTickets --> Tickets
    AdapterWebhook --> Arikawa
    AdapterWebhook --> Log
    App --> AdapterAutomod
    App --> AdapterCache
    App --> AdapterClean
    App --> AdapterEmbeds
    App --> AdapterLogging
    App --> AdapterMembers
    App --> AdapterMessages
    App --> AdapterModeration
    App --> AdapterPartners
    App --> AdapterQotd
    App --> AdapterRoles
    App --> AdapterSession
    App --> AdapterStats
    App --> AdapterTickets
    App --> AdapterWebhook
    App --> Arikawa
    App --> Clock
    App --> Commands
    App --> CommandsClean
    App --> CommandsEmbeds
    App --> CommandsLogging
    App --> CommandsModeration
    App --> CommandsPartners
    App --> CommandsQotd
    App --> CommandsRoles
    App --> CommandsRuntime
    App --> CommandsStats
    App --> Control
    App --> ControlLocaltls
    App --> DiscordGo
    App --> Files
    App --> Idgen
    App --> Log
    App --> Members
    App --> Messages
    App --> Persistence
    App --> Qotd
    App --> Runtimeapply
    App --> Service
    App --> Stats
    App --> StoragePostgres
    App --> Task
    AppRuntimecmd --> App
    Automod --> Arikawa
    AutomodAutomodmocks --> Arikawa
    AutomodAutomodmocks --> Automod
    Clock --> Log
    CmdClean-config --> Files
    CmdClean-config --> Persistence
    CmdDiscordcore --> App
    CmdDiscordcore --> AppRuntimecmd
    Commands --> Arikawa
    Commands --> Files
    Commands --> Log
    CommandsClean --> Arikawa
    CommandsClean --> Clean
    CommandsClean --> Commands
    CommandsClean --> Files
    CommandsCore --> Arikawa
    CommandsEmbeds --> AdapterEmbeds
    CommandsEmbeds --> Arikawa
    CommandsEmbeds --> Commands
    CommandsEmbeds --> Discord
    CommandsEmbeds --> Files
    CommandsLogging --> Arikawa
    CommandsLogging --> Commands
    CommandsLogging --> Files
    CommandsModeration --> AdapterModeration
    CommandsModeration --> Arikawa
    CommandsModeration --> Commands
    CommandsModeration --> Files
    CommandsModeration --> Moderation
    CommandsPartners --> AdapterPartners
    CommandsPartners --> Arikawa
    CommandsPartners --> Commands
    CommandsPartners --> Discord
    CommandsPartners --> Files
    CommandsPartners --> Log
    CommandsPartners --> Theme
    CommandsQotd --> Arikawa
    CommandsQotd --> Log
    CommandsRoles --> AdapterRoles
    CommandsRoles --> Arikawa
    CommandsRoles --> Commands
    CommandsRoles --> Files
    CommandsRuntime --> Arikawa
    CommandsRuntime --> Files
    CommandsStats --> Arikawa
    CommandsStats --> Commands
    CommandsStats --> Files
    CommandsTickets --> AdapterTickets
    CommandsTickets --> Arikawa
    CommandsTickets --> Files
    CommandsTickets --> Tickets
    Control --> AdapterCache
    Control --> Arikawa
    Control --> CommandsModeration
    Control --> Files
    Control --> Log
    Control --> Members
    Control --> Messages
    Control --> Qotd
    Control --> Runtimeapply
    Control --> StoragePostgres
    Control --> UI
    Discord --> DiscordGo
    Discord --> Files
    Files --> DiscordGo
    Files --> Idgen
    Files --> Log
    Files --> Persistence
    Files --> Theme
    Logging --> Files
    Members --> AdapterPerf
    Members --> Files
    Members --> Logging
    Members --> Service
    Members --> System
    Messages --> AdapterPerf
    Messages --> Files
    Messages --> Logging
    Messages --> Observability
    Messages --> Service
    Messages --> System
    Messages --> Task
    Persistence --> Log
    Persistence --> Observability
    Qotd --> Clock
    Qotd --> Files
    Runtimeapply --> Files
    Runtimeapply --> Service
    Service --> System
    Stats --> Files
    Stats --> Members
    Stats --> Service
    StoragePostgres --> Idgen
    StoragePostgres --> Members
    StoragePostgres --> Messages
    StoragePostgres --> Moderation
    StoragePostgres --> Qotd
    StoragePostgres --> System
    StoragePostgresStoragetest --> StoragePostgres
    Task --> Arikawa
    Task --> Clock
    Task --> Files
    Task --> Members
    Task --> Observability
    Testdb --> Persistence
    Tickets --> Arikawa
    Tickets --> StoragePostgres

    %% Styling
    classDef core fill:#232B2B,stroke:#5E81AC,stroke-width:2px,color:#ECEFF4;
    classDef adapter fill:#3B4252,stroke:#88C0D0,stroke-width:2px,color:#ECEFF4;
    classDef feature fill:#434C5E,stroke:#B48EAD,stroke-width:2px,color:#ECEFF4;
    classDef infra fill:#4C566A,stroke:#D8DEE9,stroke-width:2px,color:#ECEFF4;
    classDef external fill:#744210,stroke:#D69E2E,stroke-width:2px,color:#ECEFF4,shape:circle;
    classDef ui fill:#A3BE8C,stroke:#8FBCBB,stroke-width:2px,color:#2E3440;

    class App,AppRuntimecmd,Files,Persistence,Runtimeapply,StoragePostgres,StoragePostgresStoragetest core;
    class Discord,AdapterAutomod,AdapterCache,AdapterClean,Commands,CommandsClean,CommandsCore,CommandsEmbeds,CommandsLogging,CommandsModeration,CommandsPartners,CommandsQotd,CommandsRoles,CommandsRuntime,CommandsStats,CommandsTickets,AdapterEmbeds,AdapterLogging,AdapterMembers,AdapterMessages,AdapterModeration,AdapterPartners,AdapterPerf,AdapterQotd,AdapterRoles,AdapterSession,AdapterStats,AdapterTickets,AdapterWebhook adapter;
    class Automod,Clean,Control,ControlLocaltls,Logging,Members,Messages,Moderation,Qotd,Stats,Task,Tickets feature;
    class AutomodAutomodmocks,Clock,Idgen,Log,Observability,Service,System,Testdb,Theme infra;
    class DiscordGo,Arikawa,DiscordAPI,DiscordGateway external;
    class CmdClean-config,CmdDiscordcore,CmdTsgen,UI ui;
```
## Layer Breakdown

- **Entrypoints (`cmd/*`)**: Contains the `main` package binaries (`discordcore`, `clean-config`, `tsgen`) that bootstrap the environment and start the application, or generate typescript types.
- **Bootstrapper (`pkg/app`)**: The glue that connects the configuration, the database, and the discord sessions together into a runnable state.
- **Discord Adapters (`pkg/discord/*`)**: Connects Discord SDK behavior (e.g., DiscordGo commands, events, caching) into the core bot systems.
- **Control & Background Tasks (`pkg/control`, `pkg/task`)**: Orchestrates HTTP APIs for the dashboard and scheduled tasks independent of Discord gateway events.
- **Vertical Features**: Domain-specific logic encapsulating behavior like `QOTD`, `Partners`, etc.
- **Core Domain (`pkg/files`, `pkg/storage`)**: The foundational data layers, modeling the application's configuration state and Postgres persistence.
- **Infrastructure**: Foundational utilities such as structured logging, lifecycle management, observability hooks, and distributed ID generation (`pkg/idgen` using Snowflakes).
