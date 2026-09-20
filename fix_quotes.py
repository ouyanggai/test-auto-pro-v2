#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os
import sys

def fix_quotes(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()

        original = content
        content = content.replace('“', '"')  # "
        content = content.replace('”', '"')  # "

        if content != original:
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(content)
            return True
        return False
    except Exception as e:
        print(f"Error processing {filepath}: {e}", file=sys.stderr)
        return False

if __name__ == '__main__':
    files_fixed = 0
    for root, dirs, files in os.walk('internal'):
        for file in files:
            if file.endswith('.go'):
                filepath = os.path.join(root, file)
                if fix_quotes(filepath):
                    print(f"Fixed: {filepath}")
                    files_fixed += 1

    print(f"\nTotal files fixed: {files_fixed}")
