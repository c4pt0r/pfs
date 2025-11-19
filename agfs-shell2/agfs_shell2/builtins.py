"""Built-in shell commands"""

import re
import os
from typing import List
from .process import Process


def _mode_to_rwx(mode: int) -> str:
    """Convert octal file mode to rwx string format"""
    # Handle both full mode (e.g., 0o100644) and just permissions (e.g., 0o644 or 420 decimal)
    # Extract last 9 bits for user/group/other permissions
    perms = mode & 0o777

    def _triple(val):
        """Convert 3-bit value to rwx"""
        r = 'r' if val & 4 else '-'
        w = 'w' if val & 2 else '-'
        x = 'x' if val & 1 else '-'
        return r + w + x

    # Split into user, group, other (3 bits each)
    user = (perms >> 6) & 7
    group = (perms >> 3) & 7
    other = perms & 7

    return _triple(user) + _triple(group) + _triple(other)


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
    Concatenate and print files or stdin (streaming mode)

    Usage: cat [file...]
    """
    import sys

    if not process.args:
        # Read from stdin in chunks
        # Check if process.stdin has real data or if we should read from real stdin
        stdin_value = process.stdin.get_value()

        if stdin_value:
            # Data already in stdin buffer (from pipeline)
            process.stdout.write(stdin_value)
            process.stdout.flush()
        else:
            # No data in buffer, read from real stdin (interactive mode)
            try:
                while True:
                    chunk = sys.stdin.buffer.read(8192)
                    if not chunk:
                        break
                    process.stdout.write(chunk)
                    process.stdout.flush()
            except KeyboardInterrupt:
                process.stderr.write(b"\ncat: interrupted\n")
                return 130
    else:
        # Read from files in streaming mode
        for filename in process.args:
            try:
                if process.filesystem:
                    # Stream file in chunks
                    stream = process.filesystem.read_file(filename, stream=True)
                    try:
                        for chunk in stream:
                            if chunk:
                                process.stdout.write(chunk)
                                process.stdout.flush()
                    except KeyboardInterrupt:
                        process.stderr.write(b"\ncat: interrupted\n")
                        return 130
                else:
                    # Fallback to local filesystem
                    with open(filename, 'rb') as f:
                        while True:
                            chunk = f.read(8192)
                            if not chunk:
                                break
                            process.stdout.write(chunk)
                            process.stdout.flush()
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

    Usage: ls [-l] [path]
    """
    # Parse arguments
    long_format = False
    path = None

    for arg in process.args:
        if arg == '-l':
            long_format = True
        elif not arg.startswith('-'):
            path = arg

    # Default to current working directory if no path specified
    if path is None:
        cwd = getattr(process, 'cwd', '/')
        path = cwd

    if not process.filesystem:
        process.stderr.write("ls: filesystem not available\n")
        return 1

    try:
        files = process.filesystem.list_directory(path)

        for file_info in files:
            name = file_info.get('name', '')
            is_dir = file_info.get('isDir', False) or file_info.get('type') == 'directory'
            size = file_info.get('size', 0)

            if long_format:
                # Long format output similar to ls -l
                file_type = 'd' if is_dir else '-'

                # Get mode/permissions
                mode_str = file_info.get('mode', '')
                if mode_str and isinstance(mode_str, str) and len(mode_str) >= 9:
                    # Already in rwxr-xr-x format
                    perms = mode_str[:9]
                elif mode_str and isinstance(mode_str, int):
                    # Convert octal mode to rwx format
                    perms = _mode_to_rwx(mode_str)
                else:
                    # Default permissions
                    perms = 'rwxr-xr-x' if is_dir else 'rw-r--r--'

                # Get modification time
                mtime = file_info.get('modTime', file_info.get('mtime', ''))
                if mtime:
                    # Format timestamp (YYYY-MM-DD HH:MM:SS)
                    if 'T' in mtime:
                        # ISO format: 2025-11-18T22:00:25Z
                        mtime = mtime.replace('T', ' ').replace('Z', '').split('.')[0]
                    elif len(mtime) > 19:
                        # Truncate to 19 chars if too long
                        mtime = mtime[:19]
                else:
                    mtime = '0000-00-00 00:00:00'

                # Format: permissions size date time name
                output = f"{file_type}{perms} {size:>8} {mtime} {name}"
                if is_dir:
                    output += "/"
                output += "\n"
            else:
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
    Print working directory

    Usage: pwd
    """
    # Get cwd from process metadata if available
    cwd = getattr(process, 'cwd', '/')
    process.stdout.write(f"{cwd}\n".encode('utf-8'))
    return 0


def cmd_cd(process: Process) -> int:
    """
    Change directory

    Usage: cd [path]

    Note: This is a special builtin that needs to be handled by the shell
    """
    if not process.args:
        # cd with no args goes to root
        target_path = '/'
    else:
        target_path = process.args[0]

    if not process.filesystem:
        process.stderr.write("cd: filesystem not available\n")
        return 1

    # Store the target path in process metadata for shell to handle
    # The shell will resolve the path and verify it exists
    process.cd_target = target_path

    # Return special exit code to indicate cd operation
    # Shell will check for this and update cwd
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


def cmd_export(process: Process) -> int:
    """
    Set or display environment variables

    Usage: export [VAR=value ...]
    """
    if not process.args:
        # Display all environment variables (like 'env')
        if hasattr(process, 'env'):
            for key, value in sorted(process.env.items()):
                process.stdout.write(f"{key}={value}\n".encode('utf-8'))
        return 0

    # Set environment variables
    for arg in process.args:
        if '=' in arg:
            var_name, var_value = arg.split('=', 1)
            var_name = var_name.strip()
            var_value = var_value.strip()

            # Validate variable name
            if var_name and var_name.replace('_', '').replace('-', '').isalnum():
                if hasattr(process, 'env'):
                    process.env[var_name] = var_value
            else:
                process.stderr.write(f"export: invalid variable name: {var_name}\n")
                return 1
        else:
            process.stderr.write(f"export: usage: export VAR=value\n")
            return 1

    return 0


def cmd_env(process: Process) -> int:
    """
    Display all environment variables

    Usage: env
    """
    if hasattr(process, 'env'):
        for key, value in sorted(process.env.items()):
            process.stdout.write(f"{key}={value}\n".encode('utf-8'))
    return 0


def cmd_unset(process: Process) -> int:
    """
    Unset environment variables

    Usage: unset VAR [VAR ...]
    """
    if not process.args:
        process.stderr.write("unset: missing variable name\n")
        return 1

    if not hasattr(process, 'env'):
        return 0

    for var_name in process.args:
        if var_name in process.env:
            del process.env[var_name]

    return 0


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
    'cd': cmd_cd,
    'mkdir': cmd_mkdir,
    'rm': cmd_rm,
    'export': cmd_export,
    'env': cmd_env,
    'unset': cmd_unset,
}


def get_builtin(command: str):
    """Get a built-in command executor"""
    return BUILTINS.get(command)
