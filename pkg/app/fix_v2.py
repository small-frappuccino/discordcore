import re

def fix_bot_supervisor_test(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Remove var tracking
    content = re.sub(r'\s*origNewDiscordSession := newDiscordSession\n', '\n', content)
    content = re.sub(r'\s*origNewDiscordSessionWithIntents := newDiscordSessionWithIntents\n', '\n', content)
    content = re.sub(r'\s*origOpenBotDiscordSession := openBotDiscordSession\n', '\n', content)
    
    content = re.sub(r'\s*newDiscordSession = origNewDiscordSession\n', '\n', content)
    content = re.sub(r'\s*newDiscordSessionWithIntents = origNewDiscordSessionWithIntents\n', '\n', content)
    content = re.sub(r'\s*openBotDiscordSession = origOpenBotDiscordSession\n', '\n', content)

    # Remove session declarations
    content = re.sub(r'\s*session1, _ := discordgo\.New\(.*?\)\n\s*session1\.State\.User = &discordgo\.User\{.*?\}\n', '\n', content)
    content = re.sub(r'\s*session2, _ := discordgo\.New\(.*?\)\n\s*session2\.State\.User = &discordgo\.User\{.*?\}\n', '\n', content)
    content = re.sub(r'\s*session3, _ := discordgo\.New\(.*?\)\n\s*session3\.State\.User = &discordgo\.User\{.*?\}\n', '\n', content)

    # Remove mocked functions BEFORE replacing discordgo.Session
    content = re.sub(r'\s*newDiscordSession = func\(.*?\) \(\*discordgo\.Session, error\) \{.*?\n\t\}\n', '\n', content, flags=re.DOTALL)
    content = re.sub(r'\s*newDiscordSessionWithIntents = func\(.*?\) \(\*discordgo\.Session, error\) \{.*?\n\t\}\n', '\n', content, flags=re.DOTALL)
    content = re.sub(r'\s*openBotDiscordSession = func\(.*?\) error \{.*?\n\t\}\n', '\n', content, flags=re.DOTALL)

    content = content.replace('\t"github.com/small-frappuccino/discordgo"\n', '')

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

def fix_runner_test(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Remove var tracking
    content = re.sub(r'\s*origNewDiscordSession := newDiscordSession\n', '\n', content)
    content = re.sub(r'\s*origNewDiscordSessionWithIntents := newDiscordSessionWithIntents\n', '\n', content)
    content = re.sub(r'\s*origOpenBotDiscordSession := openBotDiscordSession\n', '\n', content)
    
    content = re.sub(r'\s*newDiscordSession = origNewDiscordSession\n', '\n', content)
    content = re.sub(r'\s*newDiscordSessionWithIntents = origNewDiscordSessionWithIntents\n', '\n', content)
    content = re.sub(r'\s*openBotDiscordSession = origOpenBotDiscordSession\n', '\n', content)

    # Remove mocked functions BEFORE replacing discordgo.Session
    content = re.sub(r'\s*newDiscordSession = func\(.*?\) \(\*discordgo\.Session, error\) \{.*?\n\t\}\n', '\n', content, flags=re.DOTALL)
    content = re.sub(r'\s*newDiscordSessionWithIntents = func\(.*?\) \(\*discordgo\.Session, error\) \{.*?\n\t\}\n', '\n', content, flags=re.DOTALL)
    content = re.sub(r'\s*openBotDiscordSession = func\(.*?\) error \{.*?\n\t\}\n', '\n', content, flags=re.DOTALL)

    content = re.sub(r'\s*session, err := discordgo\.New\("Bot test-token"\)\n\s*if err != nil \{\n\s*t\.Fatalf\("create fake discord session: %v", err\)\n\s*\}\n', '\n', content)
    content = re.sub(r'\s*session\.State\.User = &discordgo\.User\{.*?\}\n', '\n', content, flags=re.DOTALL)
    content = re.sub(r'\s*session\.State\.Guilds = \[\]\*discordgo\.Guild\{\{ID: "guild-1"\}\}\n', '\n', content)

    content = content.replace('\t"github.com/small-frappuccino/discordgo"\n', '')

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

fix_bot_supervisor_test(r'd:\Users\alice\git\discordcore\pkg\app\bot_supervisor_test.go')
fix_runner_test(r'd:\Users\alice\git\discordcore\pkg\app\runner_test.go')

print("Done")
