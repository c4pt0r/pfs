"""Main CLI entry point for agfs-shell2"""

import sys
from .shell import Shell


def main():
    """Main entry point for the shell"""
    shell = Shell()

    # Check if we have arguments (non-interactive mode)
    if len(sys.argv) > 1:
        # Execute command from arguments
        command = ' '.join(sys.argv[1:])
        # Read stdin if available AND not using input redirection
        stdin_data = None
        # Only read from stdin if it's not a tty AND command doesn't have input redirection
        # Check for input redirection operators (< but not >, >>, 2>, 2>>)
        import re
        has_input_redir = bool(re.search(r'\s<\s', command))
        if not sys.stdin.isatty() and not has_input_redir:
            stdin_data = sys.stdin.buffer.read()
        exit_code = shell.execute(command, stdin_data=stdin_data)
        sys.exit(exit_code)
    else:
        # Interactive REPL mode
        shell.repl()


if __name__ == '__main__':
    main()
