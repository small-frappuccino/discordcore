import os
import glob
import re

def search_files(directory):
    files_to_modify = []
    for root, dirs, files in os.walk(directory):
        for file in files:
            if file.endswith('_test.go'):
                filepath = os.path.join(root, file)
                with open(filepath, 'r', encoding='utf-8') as f:
                    content = f.read()
                    if '"main"' in content or '"custom"' in content:
                        files_to_modify.append(filepath)
    return files_to_modify

print(search_files('d:/Users/alice/git/discordcore'))
