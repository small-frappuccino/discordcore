def strip_lines(path):
    with open(path, 'r', encoding='utf-8') as f:
        lines = f.readlines()
        
    out = []
    for line in lines:
        if 'newDiscordSession' in line or 'newDiscordSessionWithIntents' in line or 'openBotDiscordSession' in line:
            # Skip multiline functions manually by noticing that we already broke their structure in the previous run?
            # Actually, I already replaced the `func(...) { ... }` blocks. The only things left are the `orig = ...` and `var = orig` and `var = func...` assignments that were one-liners or still there.
            pass
        else:
            out.append(line)
            
    with open(path, 'w', encoding='utf-8') as f:
        f.writelines(out)

strip_lines(r'd:\Users\alice\git\discordcore\pkg\app\bot_supervisor_test.go')
strip_lines(r'd:\Users\alice\git\discordcore\pkg\app\runner_test.go')

print("Done")
