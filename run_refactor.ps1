gofmt -w -r 'core.NewCommandError(a, b) -> &core.CommandError{Message: a, Ephemeral: b}' .
gofmt -w -r 'NewCommandError(a, b) -> &CommandError{Message: a, Ephemeral: b}' .
gofmt -w -r 'files.NewMemoryConfigManager() -> files.NewConfigManagerWithStore(&files.MemoryConfigStore{})' .
gofmt -w -r 'NewMemoryConfigManager() -> NewConfigManagerWithStore(&MemoryConfigStore{})' .
gofmt -w -r 'files.NewMemoryConfigStore() -> &files.MemoryConfigStore{}' .
gofmt -w -r 'NewMemoryConfigStore() -> &MemoryConfigStore{}' .
gofmt -w -r 'files.NewJSONManager(a) -> &files.JSONManager{FilePath: a}' .
gofmt -w -r 'NewJSONManager(a) -> &JSONManager{FilePath: a}' .
