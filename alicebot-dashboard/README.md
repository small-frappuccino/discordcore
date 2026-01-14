# alicebot-dashboard

Dashboard local para gerenciar o **alicebot.exe** com API HTTP + WebSocket em `http://127.0.0.1:3130`.

## Como rodar

```bash
bun install
bun dev
```

A aplicação ficará disponível em `http://127.0.0.1:3130`.

## Variáveis de ambiente

Crie um `.env` (opcional):

```
ADMIN_TOKEN=troque-este-token
ALICEBOT_PATH=C:\Users\alice\.local\bin\alicebot.exe
```

- `ADMIN_TOKEN`: token esperado no header `X-Admin-Token` para rotas mutáveis.
- `ALICEBOT_PATH`: path default do executável do bot (substituível pela UI).

## Configurar path do executável

- Abra a página **Settings**.
- Ajuste **Bot executable path**.
- Clique em **Test path** para validar.
- Clique em **Save Settings** para persistir em `data/settings.json`.

## Rotas da API

- `GET /api/health`
- `GET /api/status`
- `GET /api/guilds`
- `GET /api/services`
- `GET /api/settings`
- `PUT /api/settings` (X-Admin-Token)
- `GET /api/logs?limit=200`
- `POST /api/process/start` (X-Admin-Token)
- `POST /api/process/stop` (X-Admin-Token)
- `POST /api/process/restart` (X-Admin-Token)
- `POST /api/process/validate-path` (X-Admin-Token)
- `WS /api/stream`

## Observações

- Logs do processo são capturados de stdout/stderr, normalizados e distribuídos via WebSocket.
- O stop do processo tenta `proc.kill()` e faz fallback para `taskkill` no Windows.
