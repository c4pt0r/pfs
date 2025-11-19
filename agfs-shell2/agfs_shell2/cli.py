"""Main CLI entry point for agfs-shell2"""

import sys
import os
import argparse
from .shell import Shell
from .config import Config


def execute_script_file(shell, script_path):
    """Execute a script file line by line"""
    try:
        with open(script_path, 'r') as f:
            lines = f.readlines()

        exit_code = 0
        for line_num, line in enumerate(lines, 1):
            line = line.strip()

            # Skip empty lines and comments
            if not line or line.startswith('#'):
                continue

            # Execute the command
            try:
                exit_code = shell.execute(line)
                # If a command fails, stop execution
                if exit_code != 0:
                    sys.stderr.write(f"Error at line {line_num}: command failed with exit code {exit_code}\n")
                    return exit_code
            except Exception as e:
                sys.stderr.write(f"Error at line {line_num}: {str(e)}\n")
                return 1

        return exit_code
    except FileNotFoundError:
        sys.stderr.write(f"agfs-shell2: {script_path}: No such file or directory\n")
        return 127
    except Exception as e:
        sys.stderr.write(f"agfs-shell2: {script_path}: {str(e)}\n")
        return 1


def main():
    """Main entry point for the shell"""
    # Parse command line arguments
    parser = argparse.ArgumentParser(
        description='agfs-shell2 - Experimental shell with AGFS integration',
        add_help=False  # We'll handle help ourselves
    )
    parser.add_argument('--agfs-api-url',
                        dest='agfs_api_url',
                        help='AGFS API URL (default: http://localhost:8080 or $AGFS_API_URL)',
                        default=None)
    parser.add_argument('-c',
                        dest='command_string',
                        help='Execute command string',
                        default=None)
    parser.add_argument('--help', '-h', action='store_true',
                        help='Show this help message')
    parser.add_argument('script', nargs='?', help='Script file to execute')
    parser.add_argument('args', nargs='*', help='Arguments to script (or command if no script)')

    args = parser.parse_args()

    # Show help if requested
    if args.help:
        parser.print_help()
        sys.exit(0)

    # Create configuration
    config = Config.from_args(server_url=args.agfs_api_url)

    # Initialize shell with configuration
    shell = Shell(server_url=config.server_url)

    # Determine mode of execution
    # Priority: -c flag > script file > command args > interactive

    if args.command_string:
        # Mode 1: -c "command string"
        command = args.command_string
        stdin_data = None
        import re
        import select
        has_input_redir = bool(re.search(r'\s<\s', command))
        if not sys.stdin.isatty() and not has_input_redir:
            if select.select([sys.stdin], [], [], 0.0)[0]:
                stdin_data = sys.stdin.buffer.read()
        exit_code = shell.execute(command, stdin_data=stdin_data)
        sys.exit(exit_code)

    elif args.script and os.path.isfile(args.script):
        # Mode 2: script file
        exit_code = execute_script_file(shell, args.script)
        sys.exit(exit_code)

    elif args.script:
        # Mode 3: command with arguments
        command_parts = [args.script] + args.args
        command = ' '.join(command_parts)
        stdin_data = None
        import re
        import select
        has_input_redir = bool(re.search(r'\s<\s', command))
        if not sys.stdin.isatty() and not has_input_redir:
            if select.select([sys.stdin], [], [], 0.0)[0]:
                stdin_data = sys.stdin.buffer.read()
        exit_code = shell.execute(command, stdin_data=stdin_data)
        sys.exit(exit_code)

    else:
        # Mode 4: Interactive REPL
        shell.repl()


if __name__ == '__main__':
    main()
