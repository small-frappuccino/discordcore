import re

def insert_var(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Find the end of the import block
    import_end = content.find(')\n\n')
    if import_end != -1:
        insert_pos = import_end + 3
        var_block = '''var (
\t// Test hook: override this in tests to prevent real websocket connections
\topenBotArikawaState = func(ctx context.Context, s *state.State) error { return s.Open(ctx) }
)

'''
        content = content[:insert_pos] + var_block + content[insert_pos:]
    else:
        print("Import block not found")

    content = content.replace('if err := arikawaState.Open(ctx); err != nil {', 'if err := openBotArikawaState(ctx, arikawaState); err != nil {')

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

insert_var('bot_runtime.go')
print("Done")
