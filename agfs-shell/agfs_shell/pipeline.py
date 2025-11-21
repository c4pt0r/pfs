"""Pipeline class for chaining processes together"""

from typing import List
from .process import Process
from .streams import InputStream, OutputStream, ErrorStream


class Pipeline:
    """Manages a pipeline of processes connected via stdin/stdout"""

    def __init__(self, processes: List[Process]):
        """
        Initialize a pipeline

        Args:
            processes: List of Process objects to chain together
        """
        self.processes = processes
        self.exit_codes = []

    def execute(self) -> int:
        """
        Execute the entire pipeline

        Connects stdout of each process to stdin of the next process.
        Returns the exit code of the last process.

        Returns:
            Exit code of the last process
        """
        if not self.processes:
            return 0

        self.exit_codes = []

        # Execute processes in sequence, piping output to next input
        for i, process in enumerate(self.processes):
            # If this is not the first process, connect previous stdout to this stdin
            if i > 0:
                prev_process = self.processes[i - 1]
                prev_output = prev_process.get_stdout()
                process.stdin = InputStream.from_bytes(prev_output)

            # Execute the process
            exit_code = process.execute()
            self.exit_codes.append(exit_code)

        # Return exit code of last process
        return self.exit_codes[-1] if self.exit_codes else 0

    def get_stdout(self) -> bytes:
        """Get final stdout from the last process"""
        if not self.processes:
            return b''
        return self.processes[-1].get_stdout()

    def get_stderr(self) -> bytes:
        """Get combined stderr from all processes"""
        stderr_data = b''
        for process in self.processes:
            stderr_data += process.get_stderr()
        return stderr_data

    def get_exit_code(self) -> int:
        """Get exit code of the last process"""
        return self.exit_codes[-1] if self.exit_codes else 0

    def __repr__(self):
        pipeline_str = ' | '.join(str(p) for p in self.processes)
        return f"Pipeline({pipeline_str})"
