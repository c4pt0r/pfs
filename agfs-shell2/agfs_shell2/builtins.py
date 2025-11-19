"""Built-in shell commands"""

import re
import os
from typing import List
from .process import Process


def cmd_echo(process: Process) -> int:
    """Echo arguments to stdout"""
    if process.args:
        output = ' '.join(process.args) + '\n'
        process.stdout.write(output)
    else:
        process.stdout.write('\n')
    return 0


def cmd_cat(process: Process) -> int:
    """
    Concatenate and print files or stdin

    Usage: cat [file...]
    """
    if not process.args:
        # Read from stdin
        data = process.stdin.read()
        process.stdout.write(data)
    else:
        # Read from files using AGFS
        for filename in process.args:
            try:
                if process.filesystem:
                    # Use AGFS to read file
                    data = process.filesystem.read_file(filename)
                    process.stdout.write(data)
                else:
                    # Fallback to local filesystem
                    with open(filename, 'rb') as f:
                        data = f.read()
                        process.stdout.write(data)
            except Exception as e:
                # Extract meaningful error message
                error_msg = str(e)
                if "No such file or directory" in error_msg or "not found" in error_msg.lower():
                    process.stderr.write(f"cat: {filename}: No such file or directory\n")
                else:
                    process.stderr.write(f"cat: {filename}: {error_msg}\n")
                return 1
    return 0


def cmd_grep(process: Process) -> int:
    """
    Search for pattern in stdin

    Usage: grep pattern
    """
    if not process.args:
        process.stderr.write("grep: missing pattern\n")
        return 2

    pattern = process.args[0]
    try:
        regex = re.compile(pattern.encode('utf-8'))
    except re.error as e:
        process.stderr.write(f"grep: invalid pattern: {e}\n")
        return 2

    # Read from stdin line by line
    matched = False
    for line in process.stdin.readlines():
        if regex.search(line):
            process.stdout.write(line)
            matched = True

    return 0 if matched else 1


def cmd_wc(process: Process) -> int:
    """
    Count lines, words, and bytes

    Usage: wc [-l] [-w] [-c]
    """
    count_lines = False
    count_words = False
    count_bytes = False

    # Parse flags
    flags = [arg for arg in process.args if arg.startswith('-')]
    if not flags:
        # Default: count all
        count_lines = count_words = count_bytes = True
    else:
        for flag in flags:
            if 'l' in flag:
                count_lines = True
            if 'w' in flag:
                count_words = True
            if 'c' in flag:
                count_bytes = True

    # Read all data from stdin
    data = process.stdin.read()

    lines = data.count(b'\n')
    words = len(data.split())
    bytes_count = len(data)

    result = []
    if count_lines:
        result.append(str(lines))
    if count_words:
        result.append(str(words))
    if count_bytes:
        result.append(str(bytes_count))

    output = ' '.join(result) + '\n'
    process.stdout.write(output)

    return 0


def cmd_head(process: Process) -> int:
    """
    Output the first part of files

    Usage: head [-n count]
    """
    n = 10  # default

    # Parse -n flag
    args = process.args[:]
    i = 0
    while i < len(args):
        if args[i] == '-n' and i + 1 < len(args):
            try:
                n = int(args[i + 1])
                i += 2
                continue
            except ValueError:
                process.stderr.write(f"head: invalid number: {args[i + 1]}\n")
                return 1
        i += 1

    # Read lines from stdin
    lines = process.stdin.readlines()
    for line in lines[:n]:
        process.stdout.write(line)

    return 0


def cmd_tail(process: Process) -> int:
    """
    Output the last part of files

    Usage: tail [-n count]
    """
    n = 10  # default

    # Parse -n flag
    args = process.args[:]
    i = 0
    while i < len(args):
        if args[i] == '-n' and i + 1 < len(args):
            try:
                n = int(args[i + 1])
                i += 2
                continue
            except ValueError:
                process.stderr.write(f"tail: invalid number: {args[i + 1]}\n")
                return 1
        i += 1

    # Read lines from stdin
    lines = process.stdin.readlines()
    for line in lines[-n:]:
        process.stdout.write(line)

    return 0


def cmd_sort(process: Process) -> int:
    """
    Sort lines of text

    Usage: sort [-r]
    """
    reverse = '-r' in process.args

    # Read lines from stdin
    lines = process.stdin.readlines()
    lines.sort(reverse=reverse)

    for line in lines:
        process.stdout.write(line)

    return 0


def cmd_uniq(process: Process) -> int:
    """
    Report or omit repeated lines

    Usage: uniq
    """
    lines = process.stdin.readlines()
    if not lines:
        return 0

    prev_line = lines[0]
    process.stdout.write(prev_line)

    for line in lines[1:]:
        if line != prev_line:
            process.stdout.write(line)
            prev_line = line

    return 0


def cmd_tr(process: Process) -> int:
    """
    Translate characters

    Usage: tr set1 set2
    """
    if len(process.args) < 2:
        process.stderr.write("tr: missing operand\n")
        return 1

    set1 = process.args[0].encode('utf-8')
    set2 = process.args[1].encode('utf-8')

    if len(set1) != len(set2):
        process.stderr.write("tr: sets must be same length\n")
        return 1

    # Create translation table
    trans = bytes.maketrans(set1, set2)

    # Read and translate
    data = process.stdin.read()
    translated = data.translate(trans)
    process.stdout.write(translated)

    return 0


def cmd_ls(process: Process) -> int:
    """
    List directory contents

    Usage: ls [path]
    """
    # Default to current directory (root in AGFS)
    path = process.args[0] if process.args else "/"

    if not process.filesystem:
        process.stderr.write("ls: filesystem not available\n")
        return 1

    try:
        files = process.filesystem.list_directory(path)

        for file_info in files:
            name = file_info.get('name', '')
            is_dir = file_info.get('isDir', False)
            size = file_info.get('size', 0)

            # Format output similar to ls -l
            file_type = 'd' if is_dir else '-'
            mode = file_info.get('mode', 'rwxr-xr-x')

            # Simple formatting
            if is_dir:
                output = f"{name}/\n"
            else:
                output = f"{name}\n"

            process.stdout.write(output.encode('utf-8'))

        return 0
    except Exception as e:
        error_msg = str(e)
        if "No such file or directory" in error_msg or "not found" in error_msg.lower():
            process.stderr.write(f"ls: {path}: No such file or directory\n")
        else:
            process.stderr.write(f"ls: {path}: {error_msg}\n")
        return 1


def cmd_pwd(process: Process) -> int:
    """
    Print working directory (always / in AGFS shell)

    Usage: pwd
    """
    process.stdout.write(b"/\n")
    return 0


def cmd_mkdir(process: Process) -> int:
    """
    Create directory

    Usage: mkdir path
    """
    if not process.args:
        process.stderr.write("mkdir: missing operand\n")
        return 1

    if not process.filesystem:
        process.stderr.write("mkdir: filesystem not available\n")
        return 1

    path = process.args[0]

    try:
        # Use AGFS client to create directory
        process.filesystem.client.mkdir(path)
        return 0
    except Exception as e:
        error_msg = str(e)
        process.stderr.write(f"mkdir: {path}: {error_msg}\n")
        return 1


def cmd_rm(process: Process) -> int:
    """
    Remove file or directory

    Usage: rm [-r] path
    """
    if not process.args:
        process.stderr.write("rm: missing operand\n")
        return 1

    if not process.filesystem:
        process.stderr.write("rm: filesystem not available\n")
        return 1

    recursive = False
    path = None

    for arg in process.args:
        if arg == '-r' or arg == '-rf':
            recursive = True
        else:
            path = arg

    if not path:
        process.stderr.write("rm: missing file operand\n")
        return 1

    try:
        # Use AGFS client to remove file/directory
        process.filesystem.client.rm(path, recursive=recursive)
        return 0
    except Exception as e:
        error_msg = str(e)
        process.stderr.write(f"rm: {path}: {error_msg}\n")
        return 1


# Registry of built-in commands
BUILTINS = {
    'echo': cmd_echo,
    'cat': cmd_cat,
    'grep': cmd_grep,
    'wc': cmd_wc,
    'head': cmd_head,
    'tail': cmd_tail,
    'sort': cmd_sort,
    'uniq': cmd_uniq,
    'tr': cmd_tr,
    'ls': cmd_ls,
    'pwd': cmd_pwd,
    'mkdir': cmd_mkdir,
    'rm': cmd_rm,
}


def get_builtin(command: str):
    """Get a built-in command executor"""
    return BUILTINS.get(command)
