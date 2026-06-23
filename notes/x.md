A causa desta falha é um clássico anti-pattern de sistemas em Go: **Acoplamento de Telemetria e Interceptação Prematura de Erros**.

O seu teste `TestRun_MissingDatabaseURL` desconfigura a variável `DISCORDCORE_DATABASE_URL` intencionalmente para garantir que a aplicação aborte. A validação funciona perfeitamente, o problema é que a função de baixo nível `resolveDatabaseBootstrap` (e outras em `runner.go`) intercepta essa falha de validação e decide, por conta própria, acionar `log.EmitBlockingError`.

A chamada `log.EmitBlockingError` força a execução de `debug.Stack()`. Isso aloca buffers massivos na memória e varre toda a pilha de goroutines da máquina virtual do Go para montar o *stack trace*, apenas para avisar que uma string está vazia.

### A Decisão Arquitetural State-of-the-Art: Error Bubbling Puro



#### 2. A Purificação da Camada de I/O em `runner.go`

