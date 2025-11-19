"""Main CLI entry point for agfs-shell2"""

import sys
import argparse
from .shell import Shell
from .config import Config


def main():
    """Main entry point for the shell"""
    # Parse command line arguments
    parser = argparse.ArgumentParser(
        description='agfs-shell2 - Experimental shell with AGFS integration',
        add_help=False  # We'll handle help ourselves
    )
    parser.add_argument('--server', '-s',
                        help='AGFS server URL (default: http://localhost:8080 or $AGFS_SERVER_URL)',
                        default=None)
    parser.add_argument('--help', '-h', action='store_true',
                        help='Show this help message')
    parser.add_argument('command', nargs='*', help='Command to execute')

    args = parser.parse_args()

    # Show help if requested
    if args.help:
        parser.print_help()
        sys.exit(0)

    # Create configuration
    config = Config.from_args(server_url=args.server)

    # Initialize shell with configuration
    shell = Shell(server_url=config.server_url)

    # Join command parts if provided
    command_parts = args.command
    if command_parts:
        # Non-interactive mode: execute command
        command = ' '.join(command_parts)
        # Read stdin if available AND not using input redirection
        stdin_data = None
        # Only read from stdin if it's not a tty AND command doesn't have input redirection
        # Check for input redirection operators (< but not >, >>, 2>, 2>>)
        import re
        import select
        has_input_redir = bool(re.search(r'\s<\s', command))
        # Check if stdin has data available (non-blocking)
        if not sys.stdin.isatty() and not has_input_redir:
            # Use select to check if stdin has data (with 0 timeout for non-blocking)
            if select.select([sys.stdin], [], [], 0.0)[0]:
                stdin_data = sys.stdin.buffer.read()
        exit_code = shell.execute(command, stdin_data=stdin_data)
        sys.exit(exit_code)
    else:
        # Interactive REPL mode
        shell.repl()


if __name__ == '__main__':
    main()
