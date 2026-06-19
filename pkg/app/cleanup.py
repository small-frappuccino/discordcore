import re

def remove_from_file(path, patterns):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    for pattern in patterns:
        content = re.sub(pattern, '', content, flags=re.MULTILINE | re.DOTALL)
        
    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

# runner_test.go
runner_test_patterns = [
    r'^\s*origNewDiscordSession\s*:=.*?\n',
    r'^\s*origNewDiscordSessionWithIntents\s*:=.*?\n',
    r'^\s*origOpenBotDiscordSession\s*:=.*?\n',
    r'^\s*newDiscordSession\s*=\s*origNewDiscordSession\n',
    r'^\s*newDiscordSessionWithIntents\s*=\s*origNewDiscordSessionWithIntents\n',
    r'^\s*openBotDiscordSession\s*=\s*origOpenBotDiscordSession\n',
    r'^\s*newDiscordSession\s*=\s*func.*?\{.*?\}\n',
    r'^\s*newDiscordSessionWithIntents\s*=\s*func.*?\{.*?\}\n',
    r'^\s*openBotDiscordSession\s*=\s*func.*?\{.*?\}\n',
    r'^\s*sess\.State\.User\s*=\s*&discordgo\.User\{.*?\}.*?\n',
    r'^\s*sess\.State\.Guilds\s*=\s*\[\]\*discordgo\.Guild.*?\{\{ID:\s*"guild-1"\}\}\n',
    r'^\s*sess\s*:=\s*session\.NewEmptySessionForCompat\("Bot test-token"\)\n',
    r'"github.com/small-frappuccino/discordgo"\n'
]
remove_from_file(r'd:\Users\alice\git\discordcore\pkg\app\runner_test.go', runner_test_patterns)

# bot_supervisor_test.go
bot_supervisor_test_patterns = [
    r'^\s*origNewDiscordSession\s*:=.*?\n',
    r'^\s*origNewDiscordSessionWithIntents\s*:=.*?\n',
    r'^\s*origOpenBotDiscordSession\s*:=.*?\n',
    r'^\s*newDiscordSession\s*=\s*origNewDiscordSession\n',
    r'^\s*newDiscordSessionWithIntents\s*=\s*origNewDiscordSessionWithIntents\n',
    r'^\s*openBotDiscordSession\s*=\s*origOpenBotDiscordSession\n',
    r'^\s*newDiscordSession\s*=\s*func.*?\{.*?\}\n',
    r'^\s*newDiscordSessionWithIntents\s*=\s*func.*?\{.*?\}\n',
    r'^\s*openBotDiscordSession\s*=\s*func.*?\{.*?\}\n',
    r'^\s*session1,\s*_\s*:=\s*discordgo\.New\("Bot token1"\)\n',
    r'^\s*session1\.State\.User\s*=\s*&discordgo\.User\{ID:\s*"child1"\}\n',
    r'^\s*session2,\s*_\s*:=\s*discordgo\.New\("Bot token2"\)\n',
    r'^\s*session2\.State\.User\s*=\s*&discordgo\.User\{ID:\s*"child2"\}\n',
    r'^\s*session3,\s*_\s*:=\s*discordgo\.New\("Bot token3"\)\n',
    r'^\s*session3\.State\.User\s*=\s*&discordgo\.User\{ID:\s*"child3"\}\n',
    r'^\s*session1,\s*_\s*:=\s*discordgo\.New\("token1"\)\n',
    r'"github.com/small-frappuccino/discordgo"\n'
]
remove_from_file(r'd:\Users\alice\git\discordcore\pkg\app\bot_supervisor_test.go', bot_supervisor_test_patterns)

# control_test.go
control_test_patterns = [
    r'func TestListBotGuildIDsFromSessionState\(t \*testing\.T\) \{.*?\}\n\n',
    r'"github.com/small-frappuccino/discordgo"\n'
]
remove_from_file(r'd:\Users\alice\git\discordcore\pkg\app\control_test.go', control_test_patterns)

# Remove unused imports from control_test.go (reflect was only used for TestListBotGuildIDsFromSessionState)
with open(r'd:\Users\alice\git\discordcore\pkg\app\control_test.go', 'r', encoding='utf-8') as f:
    content = f.read()
content = re.sub(r'"reflect"\n', '', content)
with open(r'd:\Users\alice\git\discordcore\pkg\app\control_test.go', 'w', encoding='utf-8') as f:
    f.write(content)

print("Done")
