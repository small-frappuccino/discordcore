# Discordcore Architecture

This document provides a high-level overview of the `discordcore` system architecture, its dependencies, and how data flows across the various packages and layers.

## System Map

```mermaid
flowchart TD
    %% External APIs & Gateways (Top Level)
    DiscordGateway((Discord Gateway))
    DiscordAPI((Discord API))

    %% SDKs
    DiscordGo(("DiscordGo SDK (Fork)"))
    Arikawa(("Arikawa SDK"))
    
    %% Entrypoints
    CmdMain["cmd/discordcore"]
    CmdClean["cmd/clean-config"]
    CmdTSGen["cmd/tsgen"]
    
    %% Application Bootstrapper
    App["pkg/app (Bootstrapper)"]
    DualSDK["pkg/app (dual_sdk_publisher.go)"]

    %% Dashboard (UI)
    UI["ui (React/Vite Dashboard)"]
    
    %% Discord Sub-domains & Adapters
    Session["pkg/discord/session"]
    Commands["pkg/discord/commands"]
    Cache["pkg/discord/cache"]
    Control["pkg/control (HTTP API)"]
    ControlTLS["pkg/control/localtls"]
    Task["pkg/task (Background Jobs)"]
    RPC["pkg/discordrpc (Local IPC)"]
    Webhook["pkg/discord/webhook"]
    Perf["pkg/discord/perf"]
    Cleanup["pkg/discord/cleanup"]
    Maintenance["pkg/discord/maintenance"]
    MessageUpdate["pkg/discord/messageupdate"]
    AdapterQOTD["pkg/discord/qotd"]
    AdapterTickets["pkg/discord/tickets"]
    AdapterStats["pkg/discord/stats"]
    EventLog["pkg/discord/eventlog"]

    %% Vertical Features (Domain)
    QOTD["pkg/qotd"]
    Roles["pkg/roles"]
    Embeds["pkg/embeds"]
    Partners["pkg/partners"]
    Stats["pkg/stats"]
    Automod["pkg/automod"]
    Monitoring["pkg/monitoring"]
    Messages["pkg/messages"]
    Members["pkg/members"]
    Reactions["pkg/reactions"]
    Notifications["pkg/notifications"]
    
    %% Core Domain
    Files["pkg/files (Config & State)"]
    Storage["pkg/storage (Postgres)"]
    Persistence["pkg/persistence"]
    RuntimeApply["pkg/runtimeapply"]

    %% Infrastructure & Observability
    Service["pkg/service (Lifecycle)"]
    Log["pkg/log"]
    LogPolicy["pkg/logpolicy"]
    Observability["pkg/observability"]
    Clock["pkg/clock"]
    Theme["pkg/theme"]
    TestDB["pkg/testdb"]
    IDGen["pkg/idgen"]

    %% SDK & API Flow
    DiscordGateway -. WebSocket .-> DiscordGo
    DiscordGo -. REST Calls .-> DiscordAPI
    Arikawa -. REST Calls .-> DiscordAPI
    
    %% Downward dependencies from SDKs
    DiscordGo --> Session
    DiscordGo --> Commands
    DiscordGo --> Cache
    DiscordGo -- Provides Token --> App
    
    App -- Instantiates --> DualSDK
    DualSDK -- Instantiates --> Arikawa
    
    %% Application Bootstrapping
    %% Auto-generated internal dependencies
    %% Auto-generated internal dependencies
    AdapterQOTD --> Log
    AdapterQOTD --> QOTD
    AdapterQOTD --> Service
    AdapterStats --> Stats
    App --> AdapterQOTD
    App --> AdapterStats
    App --> Cache
    App --> Clock
    App --> Commands
    App --> Control
    App --> ControlTLS
    App --> Files
    App --> IDGen
    App --> Log
    App --> Members
    App --> Messages
    App --> Persistence
    App --> QOTD
    App --> RuntimeApply
    App --> Service
    App --> Session
    App --> Stats
    App --> Storage
    App --> Task
    App --> Webhook
    Cache --> Storage
    Clock --> Log
    CmdClean --> Files
    CmdClean --> Persistence
    CmdMain --> App
    Commands --> AdapterTickets
    Commands --> Files
    Commands --> Service
    Commands --> Stats
    Control --> Cache
    Control --> Files
    Control --> Log
    Control --> Members
    Control --> Messages
    Control --> RuntimeApply
    Control --> Storage
    Control --> UI
    Files --> IDGen
    Files --> Log
    Files --> Persistence
    Files --> Theme
    Members --> Files
    Members --> Perf
    Members --> Service
    Members --> Storage
    Messages --> Files
    Messages --> Observability
    Messages --> Perf
    Messages --> Service
    Messages --> Storage
    Messages --> Task
    Perf --> Files
    Perf --> Log
    Perf --> Observability
    Persistence --> Log
    Persistence --> Observability
    QOTD --> Clock
    QOTD --> Files
    QOTD --> Storage
    RuntimeApply --> Files
    RuntimeApply --> Service
    Service --> Storage
    Session --> Log
    Stats --> Files
    Stats --> Service
    Stats --> Storage
    Storage --> IDGen
    Task --> Clock
    Task --> Files
    Task --> Observability
    Task --> Storage
    TestDB --> Persistence
    Webhook --> Log
    
    
    %% Additional Adapter Connections
    
    %% Infrastructure Dependencies
    TestDB -. Used by tests .-> Storage
    
    %% UI & Control Relationships
    Control -. Serves embedded .-> UI
    Control -. Authenticates .-> DiscordAPI
    UI -- Fetches via Control API --> Control
    UI -. Configures .-> Partners
    UI -. Configures .-> Roles
    UI -. Configures .-> Monitoring
    
    %% Dual SDK Injection into Vertical Features
    DualSDK == Injected as Publisher ==> QOTD
    
    %% Commands orchestrating features
    
    %% Discord Domain to Adapters
    

    AdapterStats --> Arikawa
    
    Monitoring --> Arikawa
    Messages --> Arikawa
    Members --> Arikawa
    
    EventLog --> Arikawa
    
    
    
    %% Vertical Features touching Core
    
    
    %% Styling
    classDef core fill:#232B2B,stroke:#5E81AC,stroke-width:2px,color:#ECEFF4;
    classDef adapter fill:#3B4252,stroke:#88C0D0,stroke-width:2px,color:#ECEFF4;
    classDef feature fill:#434C5E,stroke:#B48EAD,stroke-width:2px,color:#ECEFF4;
    classDef infra fill:#4C566A,stroke:#D8DEE9,stroke-width:2px,color:#ECEFF4;
    classDef external fill:#744210,stroke:#D69E2E,stroke-width:2px,color:#ECEFF4,shape:circle;
    classDef ui fill:#A3BE8C,stroke:#8FBCBB,stroke-width:2px,color:#2E3440;
    
    class Files,Storage,Persistence,RuntimeApply core;
    class Control,ControlTLS,Task,RPC,Commands,Cache,Session,DualSDK,Webhook,Perf,Cleanup,Maintenance,MessageUpdate,AdapterQOTD,AdapterTickets,AdapterStats,EventLog adapter;
    class QOTD,Roles,Embeds,Partners,Stats,Automod,Monitoring,Messages,Members,Reactions,Notifications feature;
    class Service,Log,LogPolicy,Observability,Clock,Theme,TestDB,IDGen infra;
    class DiscordGo,Arikawa,DiscordAPI,DiscordGateway external;
    class UI ui;
```

## Layer Breakdown

- **Entrypoints (`cmd/*`)**: Contains the `main` package binaries (`discordcore`, `clean-config`, `tsgen`) that bootstrap the environment and start the application, or generate typescript types.
- **Bootstrapper (`pkg/app`)**: The glue that connects the configuration, the database, and the discord sessions together into a runnable state.
- **Discord Adapters (`pkg/discord/*`)**: Connects Discord SDK behavior (e.g., DiscordGo commands, events, caching) into the core bot systems.
- **Control & Background Tasks (`pkg/control`, `pkg/task`)**: Orchestrates HTTP APIs for the dashboard and scheduled tasks independent of Discord gateway events.
- **Vertical Features**: Domain-specific logic encapsulating behavior like `QOTD`, `Partners`, etc.
- **Core Domain (`pkg/files`, `pkg/storage`)**: The foundational data layers, modeling the application's configuration state and Postgres persistence.
- **Infrastructure**: Foundational utilities such as structured logging, lifecycle management, observability hooks, and distributed ID generation (`pkg/idgen` using Snowflakes).
