"""Shell command parser for pipeline syntax"""

import shlex
import re
from typing import List, Tuple, Dict, Optional


class Redirection:
    """Represents a redirection operation"""
    def __init__(self, operator: str, target: str, fd: int = None):
        self.operator = operator  # '<', '>', '>>', '2>', '2>>', '&>', etc.
        self.target = target      # filename
        self.fd = fd             # file descriptor (0=stdin, 1=stdout, 2=stderr)


class CommandParser:
    """Parse shell command strings into pipeline components"""

    @staticmethod
    def parse_command_line(command_line: str) -> Tuple[List[Tuple[str, List[str]]], Dict]:
        """
        Parse a complete command line with pipelines and redirections

        Args:
            command_line: Full command line string

        Returns:
            Tuple of (pipeline_commands, global_redirections)
        """
        # First, extract global redirections (those at the end of the pipeline)
        command_line, redirections = CommandParser.parse_redirection(command_line)

        # Then parse the pipeline
        commands = CommandParser.parse_pipeline(command_line)

        return commands, redirections

    @staticmethod
    def parse_pipeline(command_line: str) -> List[Tuple[str, List[str]]]:
        """
        Parse a command line into pipeline components

        Args:
            command_line: Command line string (e.g., "cat file.txt | grep pattern | wc -l")

        Returns:
            List of (command, args) tuples

        Example:
            >>> parser.parse_pipeline("cat file.txt | grep pattern")
            [('cat', ['file.txt']), ('grep', ['pattern'])]
        """
        if not command_line.strip():
            return []

        # Split by pipe symbol
        pipeline_parts = command_line.split('|')

        commands = []
        for part in pipeline_parts:
            part = part.strip()
            if not part:
                continue

            # Use shlex to properly handle quoted strings
            try:
                tokens = shlex.split(part)
            except ValueError as e:
                # If shlex fails (unmatched quotes), fall back to simple split
                tokens = part.split()

            if tokens:
                command = tokens[0]
                args = tokens[1:] if len(tokens) > 1 else []
                commands.append((command, args))

        return commands

    @staticmethod
    def parse_redirection(command_line: str) -> Tuple[str, Dict[str, str]]:
        """
        Parse redirection operators

        Args:
            command_line: Command line with possible redirections

        Returns:
            Tuple of (cleaned command, redirection dict)
            Redirection dict keys: 'stdin', 'stdout', 'stderr', 'stdout_mode'
        """
        redirections = {}

        # Pattern to match redirection operators
        # Match: <, >, >>, 2>, 2>>, &> followed by filename
        patterns = [
            (r'\s+<\s+(\S+)', 'stdin', None),           # < file (input)
            (r'\s+2>>\s+(\S+)', 'stderr', 'append'),    # 2>> file (append stderr)
            (r'\s+2>\s+(\S+)', 'stderr', 'write'),      # 2> file (stderr)
            (r'\s+>>\s+(\S+)', 'stdout', 'append'),     # >> file (append)
            (r'\s+>\s+(\S+)', 'stdout', 'write'),       # > file (output)
        ]

        cleaned_line = command_line

        for pattern, redirect_type, mode in patterns:
            match = re.search(pattern, cleaned_line)
            if match:
                filename = match.group(1)
                # Remove quotes if present
                if (filename.startswith('"') and filename.endswith('"')) or \
                   (filename.startswith("'") and filename.endswith("'")):
                    filename = filename[1:-1]

                redirections[redirect_type] = filename
                if mode and redirect_type in ('stdout', 'stderr'):
                    redirections[f'{redirect_type}_mode'] = mode

                # Remove the redirection from the command line
                cleaned_line = cleaned_line[:match.start()] + cleaned_line[match.end():]

        return cleaned_line.strip(), redirections

    @staticmethod
    def quote_arg(arg: str) -> str:
        """Quote an argument if it contains spaces or special characters"""
        if ' ' in arg or any(c in arg for c in '|&;<>()$`\\"\''):
            return shlex.quote(arg)
        return arg

    @staticmethod
    def unquote_arg(arg: str) -> str:
        """Remove quotes from an argument"""
        if (arg.startswith('"') and arg.endswith('"')) or \
           (arg.startswith("'") and arg.endswith("'")):
            return arg[1:-1]
        return arg
