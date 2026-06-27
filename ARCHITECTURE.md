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
    CmdDiscordcore["cmd/discordcore"]
    App["pkg/app"]
    Core["pkg/core"]
    Discord["pkg/discord"]
    Moderation["pkg/moderation"]
    Storage["pkg/storage"]

    %% SDK & API Flow
    DiscordGateway -. WebSocket .-> DiscordGo
    DiscordGo -. REST Calls .-> DiscordAPI
    Arikawa -. REST Calls .-> DiscordAPI

    %% Auto-generated internal dependencies
    App --> Core
    App --> Discord
    App --> Moderation
    App --> Storage
    CmdDiscordcore --> App
    Discord --> Core
    Discord --> Moderation
    Moderation --> Core
    Storage --> Core

    %% Styling
    classDef core fill:#232B2B,stroke:#5E81AC,stroke-width:2px,color:#ECEFF4;
    classDef adapter fill:#3B4252,stroke:#88C0D0,stroke-width:2px,color:#ECEFF4;
    classDef feature fill:#434C5E,stroke:#B48EAD,stroke-width:2px,color:#ECEFF4;
    classDef infra fill:#4C566A,stroke:#D8DEE9,stroke-width:2px,color:#ECEFF4;
    classDef external fill:#744210,stroke:#D69E2E,stroke-width:2px,color:#ECEFF4,shape:circle;
    classDef ui fill:#A3BE8C,stroke:#8FBCBB,stroke-width:2px,color:#2E3440;

    class App,Storage core;
    class Discord adapter;
    class Moderation feature;
    class Core infra;
    class DiscordGo,Arikawa,DiscordAPI,DiscordGateway external;
    class CmdDiscordcore ui;
```
## Layer Breakdown

- **Entrypoints (`cmd/*`)**: Contains the `main` package binaries (`discordcore`, `clean-config`, `tsgen`) that bootstrap the environment and start the application, or generate typescript types.
- **Bootstrapper (`pkg/app`)**: The glue that connects the configuration, the database, and the discord sessions together into a runnable state.
- **Discord Adapters (`pkg/discord/*`)**: Connects Discord SDK behavior (e.g., DiscordGo commands, events, caching) into the core bot systems.
- **Control & Background Tasks (`pkg/control`, `pkg/task`)**: Orchestrates HTTP APIs for the dashboard and scheduled tasks independent of Discord gateway events.
- **Vertical Features**: Domain-specific logic encapsulating behavior like `QOTD`, `Partners`, etc.
- **Core Domain (`pkg/files`, `pkg/storage`)**: The foundational data layers, modeling the application's configuration state and Postgres persistence.
- **Infrastructure**: Foundational utilities such as structured logging, lifecycle management, observability hooks, and distributed ID generation (`pkg/idgen` using Snowflakes).
