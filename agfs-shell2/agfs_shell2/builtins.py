"""Built-in shell commands"""

import re
import os
from typing import List
from .process import Process
from .command_decorators import command


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


@command()
def cmd_echo(process: Process) -> int:
    """Echo arguments to stdout"""
    if process.args:
        output = ' '.join(process.args) + '\n'
        process.stdout.write(output)
    else:
        process.stdout.write('\n')
    return 0


@command(needs_path_resolution=True, supports_streaming=True)
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


@command(supports_streaming=True)
def cmd_grep(process: Process) -> int:
    """
    Search for pattern in files or stdin

    Usage: grep [OPTIONS] PATTERN [FILE...]

    Options:
        -i          Ignore case
        -v          Invert match (select non-matching lines)
        -n          Print line numbers
        -c          Count matching lines
        -l          Print only filenames with matches
        -h          Suppress filename prefix (default for single file)
        -H          Print filename prefix (default for multiple files)

    Examples:
        echo 'hello world' | grep hello
        grep 'pattern' file.txt
        grep -i 'error' *.log
        grep -n 'function' code.py
        grep -v 'debug' app.log
        grep -c 'TODO' *.py
    """
    import re

    # Parse options
    ignore_case = False
    invert_match = False
    show_line_numbers = False
    count_only = False
    files_only = False
    show_filename = None  # None = auto, True = force, False = suppress

    args = process.args[:]
    options = []

    while args and args[0].startswith('-') and args[0] != '-':
        opt = args.pop(0)
        if opt == '--':
            break

        for char in opt[1:]:
            if char == 'i':
                ignore_case = True
            elif char == 'v':
                invert_match = True
            elif char == 'n':
                show_line_numbers = True
            elif char == 'c':
                count_only = True
            elif char == 'l':
                files_only = True
            elif char == 'h':
                show_filename = False
            elif char == 'H':
                show_filename = True
            else:
                process.stderr.write(f"grep: invalid option -- '{char}'\n")
                return 2

    # Get pattern
    if not args:
        process.stderr.write("grep: missing pattern\n")
        process.stderr.write("Usage: grep [OPTIONS] PATTERN [FILE...]\n")
        return 2

    pattern = args.pop(0)
    files = args

    # Compile regex
    try:
        flags = re.IGNORECASE if ignore_case else 0
        regex = re.compile(pattern, flags)
    except re.error as e:
        process.stderr.write(f"grep: invalid pattern: {e}\n")
        return 2

    # Determine if we should show filenames
    if show_filename is None:
        show_filename = len(files) > 1

    # Process files or stdin
    total_matched = False

    if not files:
        # Read from stdin
        total_matched = _grep_search(
            process, regex, None, invert_match, show_line_numbers,
            count_only, files_only, False
        )
    else:
        # Read from files
        for filepath in files:
            try:
                # Read file content
                content = process.filesystem.read_file(filepath)
                if isinstance(content, bytes):
                    content = content.decode('utf-8')

                # Create a file-like object for the content
                from io import StringIO
                file_obj = StringIO(content)

                matched = _grep_search(
                    process, regex, filepath, invert_match, show_line_numbers,
                    count_only, files_only, show_filename, file_obj
                )

                if matched:
                    total_matched = True
                    if files_only:
                        # Already printed filename, move to next file
                        continue

            except FileNotFoundError:
                process.stderr.write(f"grep: {filepath}: No such file or directory\n")
            except Exception as e:
                process.stderr.write(f"grep: {filepath}: {e}\n")

    return 0 if total_matched else 1


def _grep_search(process, regex, filename, invert_match, show_line_numbers,
                 count_only, files_only, show_filename, file_obj=None):
    """
    Helper function to search for pattern in a file or stdin

    Returns True if any matches found, False otherwise
    """
    if file_obj is None:
        # Read from stdin
        lines = process.stdin.readlines()
    else:
        # Read from file object
        lines = file_obj.readlines()

    match_count = 0
    line_number = 0

    for line in lines:
        line_number += 1

        # Handle both str and bytes
        if isinstance(line, bytes):
            line_str = line.decode('utf-8', errors='replace')
        else:
            line_str = line

        # Remove trailing newline for matching
        line_clean = line_str.rstrip('\n\r')

        # Check if line matches
        matches = bool(regex.search(line_clean))
        if invert_match:
            matches = not matches

        if matches:
            match_count += 1

            if files_only:
                # Just print filename and stop processing this file
                if filename:
                    process.stdout.write(f"{filename}\n")
                return True

            if not count_only:
                # Build output line
                output_parts = []

                if show_filename and filename:
                    output_parts.append(filename)

                if show_line_numbers:
                    output_parts.append(str(line_number))

                # Format: filename:linenum:line or just line
                if output_parts:
                    prefix = ':'.join(output_parts) + ':'
                    process.stdout.write(prefix + line_clean + '\n')
                else:
                    process.stdout.write(line_str if line_str.endswith('\n') else line_clean + '\n')

    # If count_only, print the count
    if count_only:
        if show_filename and filename:
            process.stdout.write(f"{filename}:{match_count}\n")
        else:
            process.stdout.write(f"{match_count}\n")

    return match_count > 0


@command()
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


@command()
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


@command()
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


@command()
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


@command()
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


@command()
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


def _human_readable_size(size: int) -> str:
    """Convert size in bytes to human-readable format"""
    units = ['B', 'K', 'M', 'G', 'T', 'P']
    unit_index = 0
    size_float = float(size)

    while size_float >= 1024.0 and unit_index < len(units) - 1:
        size_float /= 1024.0
        unit_index += 1

    if unit_index == 0:
        # Bytes - no decimal
        return f"{int(size_float)}{units[unit_index]}"
    elif size_float >= 10:
        # >= 10 - no decimal places
        return f"{int(size_float)}{units[unit_index]}"
    else:
        # < 10 - one decimal place
        return f"{size_float:.1f}{units[unit_index]}"


@command(needs_path_resolution=True)
def cmd_ls(process: Process) -> int:
    """
    List directory contents

    Usage: ls [-l] [-h] [path]

    Options:
        -l    Use long listing format
        -h    Print human-readable sizes (e.g., 1K, 234M, 2G)
    """
    # Parse arguments
    long_format = False
    human_readable = False
    path = None

    for arg in process.args:
        if arg.startswith('-') and arg != '-':
            # Handle combined flags like -lh
            if 'l' in arg:
                long_format = True
            if 'h' in arg:
                human_readable = True
        else:
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
                # Add color for directories (blue)
                if is_dir:
                    # Blue color for directories
                    colored_name = f"\033[1;34m{name}/\033[0m"
                else:
                    colored_name = name

                # Format size based on human_readable flag
                if human_readable:
                    size_str = f"{_human_readable_size(size):>8}"
                else:
                    size_str = f"{size:>8}"

                output = f"{file_type}{perms} {size_str} {mtime} {colored_name}\n"
            else:
                # Simple formatting
                if is_dir:
                    # Blue color for directories
                    output = f"\033[1;34m{name}/\033[0m\n"
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


@command()
def cmd_pwd(process: Process) -> int:
    """
    Print working directory

    Usage: pwd
    """
    # Get cwd from process metadata if available
    cwd = getattr(process, 'cwd', '/')
    process.stdout.write(f"{cwd}\n".encode('utf-8'))
    return 0


@command(no_pipeline=True, changes_cwd=True, needs_path_resolution=True)
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


@command(needs_path_resolution=True)
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


@command(needs_path_resolution=True)
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


@command()
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


@command()
def cmd_env(process: Process) -> int:
    """
    Display all environment variables

    Usage: env
    """
    if hasattr(process, 'env'):
        for key, value in sorted(process.env.items()):
            process.stdout.write(f"{key}={value}\n".encode('utf-8'))
    return 0


@command()
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


@command(needs_path_resolution=True)
def cmd_test(process: Process) -> int:
    """
    Evaluate conditional expressions (similar to bash test/[)

    Usage: test EXPRESSION
           [ EXPRESSION ]

    File operators:
      -f FILE    True if file exists and is a regular file
      -d FILE    True if file exists and is a directory
      -e FILE    True if file exists

    String operators:
      -z STRING  True if string is empty
      -n STRING  True if string is not empty
      STRING1 = STRING2   True if strings are equal
      STRING1 != STRING2  True if strings are not equal

    Integer operators:
      INT1 -eq INT2  True if integers are equal
      INT1 -ne INT2  True if integers are not equal
      INT1 -gt INT2  True if INT1 is greater than INT2
      INT1 -lt INT2  True if INT1 is less than INT2
      INT1 -ge INT2  True if INT1 is greater than or equal to INT2
      INT1 -le INT2  True if INT1 is less than or equal to INT2

    Logical operators:
      ! EXPR     True if expr is false
      EXPR -a EXPR  True if both expressions are true (AND)
      EXPR -o EXPR  True if either expression is true (OR)
    """
    # Handle [ command - last arg should be ]
    if process.command == '[':
        if not process.args or process.args[-1] != ']':
            process.stderr.write("[: missing ']'\n")
            return 2
        # Remove the closing ]
        process.args = process.args[:-1]

    if not process.args:
        # Empty test is false
        return 1

    # Evaluate the expression
    try:
        result = _evaluate_test_expression(process.args, process)
        return 0 if result else 1
    except Exception as e:
        process.stderr.write(f"test: {e}\n")
        return 2


def _evaluate_test_expression(args: List[str], process: Process) -> bool:
    """Evaluate a test expression"""
    if not args:
        return False

    # Single argument - test if non-empty string
    if len(args) == 1:
        return bool(args[0])

    # Negation operator
    if args[0] == '!':
        return not _evaluate_test_expression(args[1:], process)

    # File test operators
    if args[0] == '-f':
        if len(args) < 2:
            raise ValueError("-f requires an argument")
        path = args[1]
        if process.filesystem:
            try:
                info = process.filesystem.get_file_info(path)
                is_dir = info.get('isDir', False) or info.get('type') == 'directory'
                return not is_dir
            except:
                return False
        return False

    if args[0] == '-d':
        if len(args) < 2:
            raise ValueError("-d requires an argument")
        path = args[1]
        if process.filesystem:
            return process.filesystem.is_directory(path)
        return False

    if args[0] == '-e':
        if len(args) < 2:
            raise ValueError("-e requires an argument")
        path = args[1]
        if process.filesystem:
            return process.filesystem.file_exists(path)
        return False

    # String test operators
    if args[0] == '-z':
        if len(args) < 2:
            raise ValueError("-z requires an argument")
        return len(args[1]) == 0

    if args[0] == '-n':
        if len(args) < 2:
            raise ValueError("-n requires an argument")
        return len(args[1]) > 0

    # Binary operators
    if len(args) >= 3:
        # Logical AND
        if '-a' in args:
            idx = args.index('-a')
            left = _evaluate_test_expression(args[:idx], process)
            right = _evaluate_test_expression(args[idx+1:], process)
            return left and right

        # Logical OR
        if '-o' in args:
            idx = args.index('-o')
            left = _evaluate_test_expression(args[:idx], process)
            right = _evaluate_test_expression(args[idx+1:], process)
            return left or right

        # String comparison
        if args[1] == '=':
            return args[0] == args[2]

        if args[1] == '!=':
            return args[0] != args[2]

        # Integer comparison
        if args[1] in ['-eq', '-ne', '-gt', '-lt', '-ge', '-le']:
            try:
                left = int(args[0])
                right = int(args[2])
                if args[1] == '-eq':
                    return left == right
                elif args[1] == '-ne':
                    return left != right
                elif args[1] == '-gt':
                    return left > right
                elif args[1] == '-lt':
                    return left < right
                elif args[1] == '-ge':
                    return left >= right
                elif args[1] == '-le':
                    return left <= right
            except ValueError:
                raise ValueError(f"integer expression expected: {args[0]} or {args[2]}")

    # Default: non-empty first argument
    return bool(args[0])


@command(supports_streaming=True)
def cmd_jq(process: Process) -> int:
    """
    Process JSON using jq-like syntax

    Usage:
        jq FILTER [file...]
        cat file.json | jq FILTER

    Examples:
        echo '{"name":"test"}' | jq .
        cat data.json | jq '.name'
        jq '.items[]' data.json
    """
    try:
        import jq as jq_lib
        import json
    except ImportError:
        process.stderr.write("jq: jq library not installed (run: uv pip install jq)\n")
        return 1

    # First argument is the filter
    if not process.args:
        process.stderr.write("jq: missing filter expression\n")
        process.stderr.write("Usage: jq FILTER [file...]\n")
        return 1

    filter_expr = process.args[0]
    input_files = process.args[1:] if len(process.args) > 1 else []

    try:
        # Compile the jq filter
        compiled_filter = jq_lib.compile(filter_expr)
    except Exception as e:
        process.stderr.write(f"jq: compile error: {e}\n")
        return 1

    # Read JSON input
    json_data = []

    if input_files:
        # Read from files
        for filepath in input_files:
            try:
                # Read file content
                content = process.filesystem.read_file(filepath)
                if isinstance(content, bytes):
                    content = content.decode('utf-8')

                # Parse JSON
                data = json.loads(content)
                json_data.append(data)
            except FileNotFoundError:
                process.stderr.write(f"jq: {filepath}: No such file or directory\n")
                return 1
            except json.JSONDecodeError as e:
                process.stderr.write(f"jq: {filepath}: parse error: {e}\n")
                return 1
            except Exception as e:
                process.stderr.write(f"jq: {filepath}: {e}\n")
                return 1
    else:
        # Read from stdin
        stdin_data = process.stdin.read()
        if isinstance(stdin_data, bytes):
            stdin_data = stdin_data.decode('utf-8')

        if not stdin_data.strip():
            process.stderr.write("jq: no input\n")
            return 1

        try:
            data = json.loads(stdin_data)
            json_data.append(data)
        except json.JSONDecodeError as e:
            process.stderr.write(f"jq: parse error: {e}\n")
            return 1

    # Apply filter to each JSON input
    try:
        for data in json_data:
            # Run the filter
            results = compiled_filter.input(data)

            # Output results
            for result in results:
                # Pretty print JSON output
                output = json.dumps(result, indent=2, ensure_ascii=False)
                process.stdout.write(output + '\n')

        return 0
    except Exception as e:
        process.stderr.write(f"jq: filter error: {e}\n")
        return 1


@command(needs_path_resolution=True)
def cmd_stat(process: Process) -> int:
    """
    Display file status and check if file exists

    Usage: stat path
    """
    if not process.args:
        process.stderr.write("stat: missing operand\n")
        return 1

    if not process.filesystem:
        process.stderr.write("stat: filesystem not available\n")
        return 1

    path = process.args[0]

    try:
        # Get file info from the filesystem
        file_info = process.filesystem.get_file_info(path)

        # File exists, display information
        name = file_info.get('name', path.split('/')[-1] if '/' in path else path)
        is_dir = file_info.get('isDir', False) or file_info.get('type') == 'directory'
        size = file_info.get('size', 0)

        # Get mode/permissions
        mode_str = file_info.get('mode', '')
        if mode_str and isinstance(mode_str, str) and len(mode_str) >= 9:
            perms = mode_str[:9]
        elif mode_str and isinstance(mode_str, int):
            perms = _mode_to_rwx(mode_str)
        else:
            perms = 'rwxr-xr-x' if is_dir else 'rw-r--r--'

        # Get modification time
        mtime = file_info.get('modTime', file_info.get('mtime', ''))
        if mtime:
            if 'T' in mtime:
                mtime = mtime.replace('T', ' ').replace('Z', '').split('.')[0]
            elif len(mtime) > 19:
                mtime = mtime[:19]
        else:
            mtime = 'unknown'

        # Build output
        file_type = 'directory' if is_dir else 'regular file'
        output = f"  File: {name}\n"
        output += f"  Type: {file_type}\n"
        output += f"  Size: {size} bytes\n"
        output += f"  Mode: {perms}\n"
        output += f"  Modified: {mtime}\n"

        process.stdout.write(output.encode('utf-8'))
        return 0

    except Exception as e:
        error_msg = str(e)
        if "No such file or directory" in error_msg or "not found" in error_msg.lower():
            process.stderr.write("stat: No such file or directory\n")
        else:
            process.stderr.write(f"stat: {path}: {error_msg}\n")
        return 1


@command()
def cmd_upload(process: Process) -> int:
    """
    Upload a local file or directory to AGFS

    Usage: upload [-r] <local_path> <agfs_path>
    """
    # Parse arguments
    recursive = False
    args = process.args[:]

    if args and args[0] == '-r':
        recursive = True
        args = args[1:]

    if len(args) != 2:
        process.stderr.write("upload: usage: upload [-r] <local_path> <agfs_path>\n")
        return 1

    local_path = args[0]
    agfs_path = args[1]

    # Resolve agfs_path relative to current working directory
    if not agfs_path.startswith('/'):
        agfs_path = os.path.join(process.cwd, agfs_path)
        agfs_path = os.path.normpath(agfs_path)

    try:
        # Check if local path exists
        if not os.path.exists(local_path):
            process.stderr.write(f"upload: {local_path}: No such file or directory\n")
            return 1

        # Check if destination is a directory
        try:
            dest_info = process.filesystem.get_file_info(agfs_path)
            if dest_info.get('isDir', False):
                # Destination is a directory, append source filename
                source_basename = os.path.basename(local_path)
                agfs_path = os.path.join(agfs_path, source_basename)
                agfs_path = os.path.normpath(agfs_path)
        except Exception:
            # Destination doesn't exist, use as-is
            pass

        if os.path.isfile(local_path):
            # Upload single file
            return _upload_file(process, local_path, agfs_path)
        elif os.path.isdir(local_path):
            if not recursive:
                process.stderr.write(f"upload: {local_path}: Is a directory (use -r to upload recursively)\n")
                return 1
            # Upload directory recursively
            return _upload_dir(process, local_path, agfs_path)
        else:
            process.stderr.write(f"upload: {local_path}: Not a file or directory\n")
            return 1

    except Exception as e:
        error_msg = str(e)
        process.stderr.write(f"upload: {error_msg}\n")
        return 1


def _upload_file(process: Process, local_path: str, agfs_path: str, show_progress: bool = True) -> int:
    """Helper: Upload a single file to AGFS"""
    try:
        with open(local_path, 'rb') as f:
            data = f.read()
            process.filesystem.write_file(agfs_path, data, append=False)

        if show_progress:
            process.stdout.write(f"Uploaded {len(data)} bytes to {agfs_path}\n")
            process.stdout.flush()
        return 0

    except Exception as e:
        process.stderr.write(f"upload: {local_path}: {str(e)}\n")
        return 1


def _upload_dir(process: Process, local_path: str, agfs_path: str) -> int:
    """Helper: Upload a directory recursively to AGFS"""
    import stat as stat_module

    try:
        # Create target directory in AGFS if it doesn't exist
        try:
            info = process.filesystem.get_file_info(agfs_path)
            if not info.get('isDir', False):
                process.stderr.write(f"upload: {agfs_path}: Not a directory\n")
                return 1
        except Exception:
            # Directory doesn't exist, create it
            try:
                # Use mkdir command to create directory
                from pyagfs import AGFSClient
                process.filesystem.client.mkdir(agfs_path)
            except Exception as e:
                process.stderr.write(f"upload: cannot create directory {agfs_path}: {str(e)}\n")
                return 1

        # Walk through local directory
        for root, dirs, files in os.walk(local_path):
            # Calculate relative path
            rel_path = os.path.relpath(root, local_path)
            if rel_path == '.':
                current_agfs_dir = agfs_path
            else:
                current_agfs_dir = os.path.join(agfs_path, rel_path)
                current_agfs_dir = os.path.normpath(current_agfs_dir)

            # Create subdirectories in AGFS
            for dirname in dirs:
                dir_agfs_path = os.path.join(current_agfs_dir, dirname)
                dir_agfs_path = os.path.normpath(dir_agfs_path)
                try:
                    process.filesystem.client.mkdir(dir_agfs_path)
                except Exception:
                    # Directory might already exist, ignore
                    pass

            # Upload files
            for filename in files:
                local_file = os.path.join(root, filename)
                agfs_file = os.path.join(current_agfs_dir, filename)
                agfs_file = os.path.normpath(agfs_file)

                result = _upload_file(process, local_file, agfs_file)
                if result != 0:
                    return result

        return 0

    except Exception as e:
        process.stderr.write(f"upload: {str(e)}\n")
        return 1


@command()
def cmd_download(process: Process) -> int:
    """
    Download an AGFS file or directory to local filesystem

    Usage: download [-r] <agfs_path> <local_path>
    """
    # Parse arguments
    recursive = False
    args = process.args[:]

    if args and args[0] == '-r':
        recursive = True
        args = args[1:]

    if len(args) != 2:
        process.stderr.write("download: usage: download [-r] <agfs_path> <local_path>\n")
        return 1

    agfs_path = args[0]
    local_path = args[1]

    # Resolve agfs_path relative to current working directory
    if not agfs_path.startswith('/'):
        agfs_path = os.path.join(process.cwd, agfs_path)
        agfs_path = os.path.normpath(agfs_path)

    try:
        # Check if source path is a directory
        info = process.filesystem.get_file_info(agfs_path)

        # Check if destination is a local directory
        if os.path.isdir(local_path):
            # Destination is a directory, append source filename
            source_basename = os.path.basename(agfs_path)
            local_path = os.path.join(local_path, source_basename)

        if info.get('isDir', False):
            if not recursive:
                process.stderr.write(f"download: {agfs_path}: Is a directory (use -r to download recursively)\n")
                return 1
            # Download directory recursively
            return _download_dir(process, agfs_path, local_path)
        else:
            # Download single file
            return _download_file(process, agfs_path, local_path)

    except FileNotFoundError:
        process.stderr.write(f"download: {local_path}: Cannot create file\n")
        return 1
    except PermissionError:
        process.stderr.write(f"download: {local_path}: Permission denied\n")
        return 1
    except Exception as e:
        error_msg = str(e)
        if "No such file or directory" in error_msg or "not found" in error_msg.lower():
            process.stderr.write(f"download: {agfs_path}: No such file or directory\n")
        else:
            process.stderr.write(f"download: {error_msg}\n")
        return 1


def _download_file(process: Process, agfs_path: str, local_path: str, show_progress: bool = True) -> int:
    """Helper: Download a single file from AGFS"""
    try:
        stream = process.filesystem.read_file(agfs_path, stream=True)
        bytes_written = 0

        with open(local_path, 'wb') as f:
            for chunk in stream:
                if chunk:
                    f.write(chunk)
                    bytes_written += len(chunk)

        if show_progress:
            process.stdout.write(f"Downloaded {bytes_written} bytes to {local_path}\n")
            process.stdout.flush()
        return 0

    except Exception as e:
        process.stderr.write(f"download: {agfs_path}: {str(e)}\n")
        return 1


def _download_dir(process: Process, agfs_path: str, local_path: str) -> int:
    """Helper: Download a directory recursively from AGFS"""
    try:
        # Create local directory if it doesn't exist
        os.makedirs(local_path, exist_ok=True)

        # List AGFS directory
        entries = process.filesystem.list_directory(agfs_path)

        for entry in entries:
            name = entry['name']
            is_dir = entry.get('isDir', False)

            agfs_item = os.path.join(agfs_path, name)
            agfs_item = os.path.normpath(agfs_item)
            local_item = os.path.join(local_path, name)

            if is_dir:
                # Recursively download subdirectory
                result = _download_dir(process, agfs_item, local_item)
                if result != 0:
                    return result
            else:
                # Download file
                result = _download_file(process, agfs_item, local_item)
                if result != 0:
                    return result

        return 0

    except Exception as e:
        process.stderr.write(f"download: {str(e)}\n")
        return 1


@command()
def cmd_cp(process: Process) -> int:
    """
    Copy files between local filesystem and AGFS

    Usage:
        cp [-r] <source> <dest>
        cp [-r] local:<path> <agfs_path>   # Upload from local to AGFS
        cp [-r] <agfs_path> local:<path>   # Download from AGFS to local
        cp [-r] <agfs_path1> <agfs_path2>  # Copy within AGFS
    """
    # Parse arguments
    recursive = False
    args = process.args[:]

    if args and args[0] == '-r':
        recursive = True
        args = args[1:]

    if len(args) != 2:
        process.stderr.write("cp: usage: cp [-r] <source> <dest>\n")
        return 1

    source = args[0]
    dest = args[1]

    # Parse source and dest to determine operation type
    source_is_local = source.startswith('local:')
    dest_is_local = dest.startswith('local:')

    if source_is_local:
        source = source[6:]  # Remove 'local:' prefix
    if dest_is_local:
        dest = dest[6:]  # Remove 'local:' prefix

    # Determine operation type
    if source_is_local and not dest_is_local:
        # Upload: local -> AGFS
        return _cp_upload(process, source, dest, recursive)
    elif not source_is_local and dest_is_local:
        # Download: AGFS -> local
        return _cp_download(process, source, dest, recursive)
    elif not source_is_local and not dest_is_local:
        # Copy within AGFS
        return _cp_agfs(process, source, dest, recursive)
    else:
        # local -> local (not supported, use system cp)
        process.stderr.write("cp: local to local copy not supported, use system cp command\n")
        return 1


def _cp_upload(process: Process, local_path: str, agfs_path: str, recursive: bool = False) -> int:
    """Helper: Upload local file or directory to AGFS"""
    # Resolve agfs_path relative to current working directory
    if not agfs_path.startswith('/'):
        agfs_path = os.path.join(process.cwd, agfs_path)
        agfs_path = os.path.normpath(agfs_path)

    try:
        if not os.path.exists(local_path):
            process.stderr.write(f"cp: {local_path}: No such file or directory\n")
            return 1

        # Check if destination is a directory
        try:
            dest_info = process.filesystem.get_file_info(agfs_path)
            if dest_info.get('isDir', False):
                # Destination is a directory, append source filename
                source_basename = os.path.basename(local_path)
                agfs_path = os.path.join(agfs_path, source_basename)
                agfs_path = os.path.normpath(agfs_path)
        except Exception:
            # Destination doesn't exist, use as-is
            pass

        if os.path.isfile(local_path):
            # Show progress
            process.stdout.write(f"local:{local_path} -> {agfs_path}\n")
            process.stdout.flush()

            # Upload file
            with open(local_path, 'rb') as f:
                process.filesystem.write_file(agfs_path, f.read(), append=False)
            return 0

        elif os.path.isdir(local_path):
            if not recursive:
                process.stderr.write(f"cp: {local_path}: Is a directory (use -r to copy recursively)\n")
                return 1
            # Upload directory recursively
            return _upload_dir(process, local_path, agfs_path)

        else:
            process.stderr.write(f"cp: {local_path}: Not a file or directory\n")
            return 1

    except Exception as e:
        process.stderr.write(f"cp: {str(e)}\n")
        return 1


def _cp_download(process: Process, agfs_path: str, local_path: str, recursive: bool = False) -> int:
    """Helper: Download AGFS file or directory to local"""
    # Resolve agfs_path relative to current working directory
    if not agfs_path.startswith('/'):
        agfs_path = os.path.join(process.cwd, agfs_path)
        agfs_path = os.path.normpath(agfs_path)

    try:
        # Check if source is a directory
        info = process.filesystem.get_file_info(agfs_path)

        # Check if destination is a local directory
        if os.path.isdir(local_path):
            # Destination is a directory, append source filename
            source_basename = os.path.basename(agfs_path)
            local_path = os.path.join(local_path, source_basename)

        if info.get('isDir', False):
            if not recursive:
                process.stderr.write(f"cp: {agfs_path}: Is a directory (use -r to copy recursively)\n")
                return 1
            # Download directory recursively
            return _download_dir(process, agfs_path, local_path)
        else:
            # Show progress
            process.stdout.write(f"{agfs_path} -> local:{local_path}\n")
            process.stdout.flush()

            # Download single file
            stream = process.filesystem.read_file(agfs_path, stream=True)
            with open(local_path, 'wb') as f:
                for chunk in stream:
                    if chunk:
                        f.write(chunk)
            return 0

    except FileNotFoundError:
        process.stderr.write(f"cp: {local_path}: Cannot create file\n")
        return 1
    except PermissionError:
        process.stderr.write(f"cp: {local_path}: Permission denied\n")
        return 1
    except Exception as e:
        error_msg = str(e)
        if "No such file or directory" in error_msg or "not found" in error_msg.lower():
            process.stderr.write(f"cp: {agfs_path}: No such file or directory\n")
        else:
            process.stderr.write(f"cp: {str(e)}\n")
        return 1


def _cp_agfs(process: Process, source_path: str, dest_path: str, recursive: bool = False) -> int:
    """Helper: Copy within AGFS"""
    # Resolve paths relative to current working directory
    if not source_path.startswith('/'):
        source_path = os.path.join(process.cwd, source_path)
        source_path = os.path.normpath(source_path)

    if not dest_path.startswith('/'):
        dest_path = os.path.join(process.cwd, dest_path)
        dest_path = os.path.normpath(dest_path)

    try:
        # Check if source is a directory
        info = process.filesystem.get_file_info(source_path)

        # Check if destination is a directory
        try:
            dest_info = process.filesystem.get_file_info(dest_path)
            if dest_info.get('isDir', False):
                # Destination is a directory, append source filename
                source_basename = os.path.basename(source_path)
                dest_path = os.path.join(dest_path, source_basename)
                dest_path = os.path.normpath(dest_path)
        except Exception:
            # Destination doesn't exist, use as-is
            pass

        if info.get('isDir', False):
            if not recursive:
                process.stderr.write(f"cp: {source_path}: Is a directory (use -r to copy recursively)\n")
                return 1
            # Copy directory recursively
            return _cp_agfs_dir(process, source_path, dest_path)
        else:
            # Show progress
            process.stdout.write(f"{source_path} -> {dest_path}\n")
            process.stdout.flush()

            # Copy single file - read all at once to avoid append overhead
            data = process.filesystem.read_file(source_path, stream=False)
            process.filesystem.write_file(dest_path, data, append=False)

            return 0

    except Exception as e:
        error_msg = str(e)
        if "No such file or directory" in error_msg or "not found" in error_msg.lower():
            process.stderr.write(f"cp: {source_path}: No such file or directory\n")
        else:
            process.stderr.write(f"cp: {str(e)}\n")
        return 1


def _cp_agfs_dir(process: Process, source_path: str, dest_path: str) -> int:
    """Helper: Recursively copy directory within AGFS"""
    try:
        # Create destination directory if it doesn't exist
        try:
            info = process.filesystem.get_file_info(dest_path)
            if not info.get('isDir', False):
                process.stderr.write(f"cp: {dest_path}: Not a directory\n")
                return 1
        except Exception:
            # Directory doesn't exist, create it
            try:
                process.filesystem.client.mkdir(dest_path)
            except Exception as e:
                process.stderr.write(f"cp: cannot create directory {dest_path}: {str(e)}\n")
                return 1

        # List source directory
        entries = process.filesystem.list_directory(source_path)

        for entry in entries:
            name = entry['name']
            is_dir = entry.get('isDir', False)

            src_item = os.path.join(source_path, name)
            src_item = os.path.normpath(src_item)
            dst_item = os.path.join(dest_path, name)
            dst_item = os.path.normpath(dst_item)

            if is_dir:
                # Recursively copy subdirectory
                result = _cp_agfs_dir(process, src_item, dst_item)
                if result != 0:
                    return result
            else:
                # Show progress
                process.stdout.write(f"{src_item} -> {dst_item}\n")
                process.stdout.flush()

                # Copy file - read all at once to avoid append overhead
                data = process.filesystem.read_file(src_item, stream=False)
                process.filesystem.write_file(dst_item, data, append=False)

        return 0

    except Exception as e:
        process.stderr.write(f"cp: {str(e)}\n")
        return 1


@command()
def cmd_sleep(process: Process) -> int:
    """
    Pause execution for specified seconds

    Usage: sleep SECONDS

    Examples:
        sleep 1      # Sleep for 1 second
        sleep 0.5    # Sleep for 0.5 seconds
        sleep 5      # Sleep for 5 seconds
    """
    import time

    if not process.args:
        process.stderr.write("sleep: missing operand\n")
        process.stderr.write("Usage: sleep SECONDS\n")
        return 1

    try:
        seconds = float(process.args[0])
        if seconds < 0:
            process.stderr.write("sleep: invalid time interval\n")
            return 1

        time.sleep(seconds)
        return 0
    except ValueError:
        process.stderr.write(f"sleep: invalid time interval '{process.args[0]}'\n")
        return 1
    except KeyboardInterrupt:
        process.stderr.write("\nsleep: interrupted\n")
        return 130


@command()
def cmd_plugins(process: Process) -> int:
    """
    Manage external plugins

    Usage: plugins <subcommand> [arguments]

    Subcommands:
        load <path>       Load external plugin from AGFS or HTTP(S)
        unload <path>     Unload external plugin
        list              List loaded external plugins

    Path formats for load:
        <agfs_path>        - Load from AGFS (default)
        http(s)://<url>    - Load from HTTP(S) URL

    Examples:
        plugins list                                  # List loaded external plugins
        plugins load /mnt/plugins/myplugin.so         # Load from AGFS
        plugins load https://example.com/myplugin.so  # Load from HTTP(S)
        plugins unload /mnt/plugins/myplugin.so       # Unload plugin
    """
    if not process.filesystem:
        process.stderr.write("plugins: filesystem not available\n")
        return 1

    # No arguments - show usage
    if len(process.args) == 0:
        process.stderr.write("Usage: plugins <subcommand> [arguments]\n")
        process.stderr.write("\nSubcommands:\n")
        process.stderr.write("  load <path>    - Load external plugin\n")
        process.stderr.write("  unload <path>  - Unload external plugin\n")
        process.stderr.write("  list           - List loaded external plugins\n")
        process.stderr.write("\nPath formats for load:\n")
        process.stderr.write("  <agfs_path>      - Load from AGFS (default)\n")
        process.stderr.write("  http(s)://<url>  - Load from HTTP(S) URL\n")
        process.stderr.write("\nExamples:\n")
        process.stderr.write("  plugins list\n")
        process.stderr.write("  plugins load /mnt/plugins/myplugin.so\n")
        process.stderr.write("  plugins load https://example.com/myplugin.so\n")
        return 1

    # Handle plugin subcommands
    subcommand = process.args[0].lower()

    if subcommand == "load":
        if len(process.args) < 2:
            process.stderr.write("Usage: plugins load <path>\n")
            process.stderr.write("\nPath formats:\n")
            process.stderr.write("  <agfs_path>      - Load from AGFS (default)\n")
            process.stderr.write("  http(s)://<url>  - Load from HTTP(S) URL\n")
            process.stderr.write("\nExamples:\n")
            process.stderr.write("  plugins load /mnt/plugins/myplugin.so        # From AGFS\n")
            process.stderr.write("  plugins load https://example.com/myplugin.so # From HTTP(S)\n")
            return 1

        path = process.args[1]

        # Determine path type
        is_http = path.startswith('http://') or path.startswith('https://')

        # Process path based on type
        if is_http:
            # HTTP(S) URL: use as-is, server will download it
            library_path = path
        else:
            # Default: treat as AGFS path, add agfs:// prefix
            library_path = f"agfs://{path}"

        try:
            # Load the plugin
            result = process.filesystem.client.load_plugin(library_path)
            plugin_name = result.get("plugin_name", "unknown")
            process.stdout.write(f"Loaded external plugin: {plugin_name}\n")
            process.stdout.write(f"  Source: {path}\n")
            return 0
        except Exception as e:
            error_msg = str(e)
            process.stderr.write(f"plugins load: {error_msg}\n")
            return 1

    elif subcommand == "unload":
        if len(process.args) < 2:
            process.stderr.write("Usage: plugins unload <library_path>\n")
            return 1

        library_path = process.args[1]

        try:
            process.filesystem.client.unload_plugin(library_path)
            process.stdout.write(f"Unloaded external plugin: {library_path}\n")
            return 0
        except Exception as e:
            error_msg = str(e)
            process.stderr.write(f"plugins unload: {error_msg}\n")
            return 1

    elif subcommand == "list":
        try:
            plugins = process.filesystem.client.list_plugins()

            if not plugins:
                process.stdout.write("No external plugins loaded\n")
                return 0

            # Get mount information to correlate with loaded plugins
            try:
                mounts = process.filesystem.client.mounts()
                # Build a map of plugin names to mount points
                plugin_mounts = {}
                for mount in mounts:
                    plugin_name = mount.get('pluginName', '')
                    if plugin_name:
                        if plugin_name not in plugin_mounts:
                            plugin_mounts[plugin_name] = []
                        plugin_mounts[plugin_name].append(mount.get('path', ''))
            except:
                plugin_mounts = {}

            process.stdout.write(f"Loaded External Plugins: ({len(plugins)})\n")
            for plugin_path in plugins:
                # Extract just the filename for display
                filename = os.path.basename(plugin_path)
                process.stdout.write(f"  {filename}\n")

                # Try to show which plugin types are available from this file
                # by checking if any mounts use a plugin with similar name
                found_mounts = False
                for plugin_name, mount_paths in plugin_mounts.items():
                    # Check if this plugin_name might come from this file
                    # (simple heuristic: check if filename contains plugin name or vice versa)
                    if plugin_name.lower() in filename.lower() or filename.lower().replace('.wasm', '').replace('.so', '').replace('.dylib', '') in plugin_name.lower():
                        process.stdout.write(f"    Plugin type: {plugin_name}\n")
                        if mount_paths:
                            process.stdout.write(f"    Mounted at: {', '.join(mount_paths)}\n")
                        found_mounts = True

                if not found_mounts:
                    process.stdout.write(f"    (Not currently mounted)\n")

            return 0
        except Exception as e:
            error_msg = str(e)
            process.stderr.write(f"plugins list: {error_msg}\n")
            return 1

    else:
        process.stderr.write(f"plugins: unknown subcommand: {subcommand}\n")
        process.stderr.write("\nUsage:\n")
        process.stderr.write("  plugins load <library_path|url|pfs://..> - Load external plugin\n")
        process.stderr.write("  plugins unload <library_path>            - Unload external plugin\n")
        process.stderr.write("  plugins list                             - List loaded external plugins\n")
        return 1


@command()
def cmd_mount(process: Process) -> int:
    """
    Mount a plugin dynamically or list mounted filesystems

    Usage: mount [<fstype> <path> [key=value ...]]

    Without arguments: List all mounted filesystems
    With arguments: Mount a new filesystem

    Examples:
        mount                    # List all mounted filesystems
        mount memfs /test/mem
        mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db
        mount s3fs /test/s3 bucket=my-bucket region=us-west-1 access_key_id=xxx secret_access_key=yyy
    """
    if not process.filesystem:
        process.stderr.write("mount: filesystem not available\n")
        return 1

    # No arguments - list mounted filesystems
    if len(process.args) == 0:
        try:
            mounts_list = process.filesystem.client.mounts()

            if not mounts_list:
                process.stdout.write("No plugins mounted\n")
                return 0

            # Print mounts in Unix mount style: <fstype> on <mountpoint> (options...)
            for mount in mounts_list:
                path = mount.get("path", "")
                plugin = mount.get("pluginName", "")
                config = mount.get("config", {})

                # Build options string from config
                options = []
                for key, value in config.items():
                    # Hide sensitive keys
                    if key in ["secret_access_key", "password", "token"]:
                        options.append(f"{key}=***")
                    else:
                        # Convert value to string, truncate if too long
                        value_str = str(value)
                        if len(value_str) > 50:
                            value_str = value_str[:47] + "..."
                        options.append(f"{key}={value_str}")

                # Format output line
                if options:
                    options_str = ", ".join(options)
                    process.stdout.write(f"{plugin} on {path} (plugin: {plugin}, {options_str})\n")
                else:
                    process.stdout.write(f"{plugin} on {path} (plugin: {plugin})\n")

            return 0
        except Exception as e:
            error_msg = str(e)
            process.stderr.write(f"mount: {error_msg}\n")
            return 1

    # With arguments - mount a new filesystem
    if len(process.args) < 2:
        process.stderr.write("mount: missing operands\n")
        process.stderr.write("Usage: mount <fstype> <path> [key=value ...]\n")
        process.stderr.write("\nExamples:\n")
        process.stderr.write("  mount memfs /test/mem\n")
        process.stderr.write("  mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db\n")
        process.stderr.write("  mount s3fs /test/s3 bucket=my-bucket region=us-west-1\n")
        return 1

    fstype = process.args[0]
    path = process.args[1]
    config_args = process.args[2:] if len(process.args) > 2 else []

    # Parse key=value config arguments
    config = {}
    for arg in config_args:
        if '=' in arg:
            key, value = arg.split('=', 1)
            config[key.strip()] = value.strip()
        else:
            process.stderr.write(f"mount: invalid config argument: {arg}\n")
            process.stderr.write("Config arguments must be in key=value format\n")
            return 1

    try:
        # Use AGFS client to mount the plugin
        process.filesystem.client.mount(fstype, path, config)
        process.stdout.write(f"Mounted {fstype} at {path}\n")
        return 0
    except Exception as e:
        error_msg = str(e)
        process.stderr.write(f"mount: {error_msg}\n")
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
    'cd': cmd_cd,
    'mkdir': cmd_mkdir,
    'rm': cmd_rm,
    'export': cmd_export,
    'env': cmd_env,
    'unset': cmd_unset,
    'test': cmd_test,
    '[': cmd_test,  # [ is an alias for test
    'stat': cmd_stat,
    'jq': cmd_jq,
    'upload': cmd_upload,
    'download': cmd_download,
    'cp': cmd_cp,
    'sleep': cmd_sleep,
    'plugins': cmd_plugins,
    'mount': cmd_mount,
}


def get_builtin(command: str):
    """Get a built-in command executor"""
    return BUILTINS.get(command)
