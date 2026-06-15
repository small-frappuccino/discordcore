import os
import glob

def process_file(filepath):
    with open(filepath, 'r') as f:
        content = f.read()
    
    new_content = content.replace(', _ := guild.ResolveFeatureBotInstanceID', ' := guild.ResolveFeatureBotInstanceID')
    new_content = new_content.replace(', _ := guildConfig.ResolveFeatureBotInstanceID', ' := guildConfig.ResolveFeatureBotInstanceID')
    new_content = new_content.replace(', _ := gcfg.ResolveFeatureBotInstanceID', ' := gcfg.ResolveFeatureBotInstanceID')
    new_content = new_content.replace(', _ := cfg.ResolveFeatureBotInstanceID', ' := cfg.ResolveFeatureBotInstanceID')
    
    if content != new_content:
        with open(filepath, 'w') as f:
            f.write(new_content)
        print(f"Updated {filepath}")

for root, _, files in os.walk('d:/Users/alice/git/discordcore/pkg/'):
    for file in files:
        if file.endswith('.go'):
            process_file(os.path.join(root, file))
