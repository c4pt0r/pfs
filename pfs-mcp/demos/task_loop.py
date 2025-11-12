#!/usr/bin/env python3
"""
Task Loop - Fetch tasks from PFS QueueFS and execute with Claude Code
"""

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime
from typing import Any, Dict, Optional


class TaskQueue:
    """PFS QueueFS task queue client"""

    def __init__(
        self,
        queue_path: str = "/queuefs/agent",
        pfs_api_baseurl: Optional[str] = "http://localhost:8080/api/v1",
    ):
        """
        Initialize task queue client

        Args:
            queue_path: QueueFS mount path
            pfs_api_baseurl: PFS API server URL (optional)
        """
        self.queue_path = queue_path
        self.pfs_api_baseurl = pfs_api_baseurl
        self.dequeue_path = f"{queue_path}/dequeue"
        self.size_path = f"{queue_path}/size"
        self.peek_path = f"{queue_path}/peek"

    def _run_pfs_command(self, args: list[str]) -> Optional[str]:
        """
        Execute PFS command

        Args:
            args: PFS command arguments list

        Returns:
            Command output, None if failed
        """
        cmd = ["uv", "run", "pfs"]

        # Add custom API URL if specified
        if self.pfs_api_baseurl:
            cmd.extend(["--pfs-api-baseurl", self.pfs_api_baseurl])

        cmd.extend(args)

        try:
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

            if result.returncode == 0:
                return result.stdout.strip()
            else:
                print(f"Error: PFS command failed: {result.stderr}", file=sys.stderr)
                return None

        except subprocess.TimeoutExpired:
            print("Error: PFS command timed out", file=sys.stderr)
            return None
        except FileNotFoundError:
            print(
                "Error: 'uv' command not found, please ensure uv is installed",
                file=sys.stderr,
            )
            return None
        except Exception as e:
            print(
                f"Error: Exception occurred while executing PFS command: {e}",
                file=sys.stderr,
            )
            return None

    def get_queue_size(self) -> Optional[int]:
        """
        Get queue size

        Returns:
            Number of messages in queue, None if failed
        """
        output = self._run_pfs_command(["cat", self.size_path])
        if output:
            try:
                return int(output)
            except ValueError:
                print(f"Warning: Cannot parse queue size: {output}", file=sys.stderr)
        return None

    def peek_task(self) -> Optional[Dict[str, Any]]:
        """
        Peek at next task without removing it

        Returns:
            Task data dictionary, None if failed
        """
        output = self._run_pfs_command(["cat", self.peek_path])
        if output:
            try:
                return json.loads(output)
            except json.JSONDecodeError:
                print(f"Warning: Cannot parse JSON: {output}", file=sys.stderr)
        return None

    def dequeue_task(self) -> Optional[Dict[str, Any]]:
        """
        Get a task from queue (removes it)

        Returns:
            Task data dictionary with format: {"id": "...", "data": "...", "timestamp": "..."}
            Returns None if queue is empty or operation failed
        """
        output = self._run_pfs_command(["cat", self.dequeue_path])
        if output:
            try:
                return json.loads(output)
            except json.JSONDecodeError:
                print(f"Warning: Cannot parse JSON: {output}", file=sys.stderr)
        return None


class ClaudeCodeExecutor:
    """Execute tasks using Claude Code in headless mode"""

    def __init__(
        self,
        timeout: int = 600,
        allowed_tools: Optional[list[str]] = None,
        name: str = "",
    ):
        """
        Initialize Claude Code executor

        Args:
            timeout: Maximum execution time in seconds (default: 600)
            allowed_tools: List of allowed tools (None = all tools allowed)
        """
        self.timeout = timeout
        self.allowed_tools = allowed_tools
        self.agent_name = name

    def execute_task(
        self, task_prompt: str, working_dir: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Execute a task using Claude Code in headless mode

        Args:
            task_prompt: The task prompt to send to Claude Code
            working_dir: Working directory for Claude Code (optional)

        Returns:
            Dictionary with execution results including:
            - success: bool
            - result: str (Claude's response)
            - error: str (error message if failed)
            - duration_ms: int
            - total_cost_usd: float
            - session_id: str
        """
        cmd = [
            "claude",
            "-p",
            task_prompt,
            "--output-format",
            "json",
            "--permission-mode=bypassPermissions",
        ]

        # Add allowed tools if specified
        if self.allowed_tools:
            cmd.extend(["--allowedTools", ",".join(self.allowed_tools)])

        try:
            print(f"\n[Executing Claude Code...]")
            start_time = time.time()

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=self.timeout,
                cwd=working_dir,
            )

            execution_time = (time.time() - start_time) * 1000  # Convert to ms

            if result.returncode == 0:
                try:
                    output = json.loads(result.stdout)
                    return {
                        "success": True,
                        "result": output.get("result", ""),
                        "error": None,
                        "duration_ms": output.get("duration_ms", execution_time),
                        "total_cost_usd": output.get("total_cost_usd", 0.0),
                        "session_id": output.get("session_id", ""),
                    }
                except json.JSONDecodeError as e:
                    return {
                        "success": False,
                        "result": result.stdout,
                        "error": f"Failed to parse JSON output: {e}",
                        "duration_ms": execution_time,
                        "total_cost_usd": 0.0,
                        "session_id": "",
                    }
            else:
                return {
                    "success": False,
                    "result": "",
                    "error": f"Claude Code exited with code {result.returncode}: {result.stderr}",
                    "duration_ms": execution_time,
                    "total_cost_usd": 0.0,
                    "session_id": "",
                }

        except subprocess.TimeoutExpired:
            return {
                "success": False,
                "result": "",
                "error": f"Execution timed out after {self.timeout} seconds",
                "duration_ms": self.timeout * 1000,
                "total_cost_usd": 0.0,
                "session_id": "",
            }
        except FileNotFoundError:
            return {
                "success": False,
                "result": "",
                "error": "'claude' command not found. Please ensure Claude Code is installed.",
                "duration_ms": 0,
                "total_cost_usd": 0.0,
                "session_id": "",
            }
        except Exception as e:
            return {
                "success": False,
                "result": "",
                "error": f"Unexpected error: {e}",
                "duration_ms": 0,
                "total_cost_usd": 0.0,
                "session_id": "",
            }


def main():
    """Main function: loop to fetch tasks and output to console"""

    # Parse command line arguments
    parser = argparse.ArgumentParser(
        description="Fetch tasks from PFS QueueFS and execute with Claude Code"
    )
    parser.add_argument(
        "--queue-path",
        type=str,
        default="/queuefs/agent",
        help="QueueFS mount path (default: /queuefs/agent)",
    )
    parser.add_argument(
        "--api-url", type=str, default=None, help="PFS API server URL (optional)"
    )
    parser.add_argument(
        "--poll-interval",
        type=int,
        default=2,
        help="Poll interval in seconds when queue is empty (default: 2)",
    )
    parser.add_argument(
        "--claude-timeout",
        type=int,
        default=600,
        help="Claude Code execution timeout in seconds (default: 600)",
    )
    parser.add_argument(
        "--allowed-tools",
        type=str,
        default=None,
        help="Comma-separated list of allowed tools for Claude Code (default: all tools)",
    )
    parser.add_argument(
        "--working-dir",
        type=str,
        default=None,
        help="Working directory for Claude Code execution (default: current directory)",
    )

    parser.add_argument("--name", type=str, default=None, help="agent name")

    args = parser.parse_args()

    # Parse allowed tools if specified
    allowed_tools = None
    if args.allowed_tools:
        allowed_tools = [tool.strip() for tool in args.allowed_tools.split(",")]

    # Create task queue client
    queue = TaskQueue(queue_path=args.queue_path, pfs_api_baseurl=args.api_url)

    # Create Claude Code executor
    executor = ClaudeCodeExecutor(
        timeout=args.claude_timeout, allowed_tools=allowed_tools
    )

    print("=== PFS Task Loop with Claude Code ===")
    print(f"Monitoring queue: {queue.queue_path}")
    if args.api_url:
        print(f"PFS API URL: {args.api_url}")
    print(f"Poll interval: {args.poll_interval}s")
    print(f"Claude timeout: {args.claude_timeout}s")
    if allowed_tools:
        print(f"Allowed tools: {', '.join(allowed_tools)}")
    if args.working_dir:
        print(f"Working directory: {args.working_dir}")
    print("Press Ctrl+C to exit\n")

    try:
        while True:
            # Check queue size
            size = queue.get_queue_size()
            if size is not None and size > 0:
                print(f"[Queue size: {size}]")

            # Fetch task
            task = queue.dequeue_task()

            if task:
                task_id = task.get("id", "N/A")
                task_data = task.get("data", "")
                task_timestamp = task.get("timestamp", "N/A")

                print("\n" + "=" * 80)
                print(f"📥 NEW TASK RECEIVED")
                print("=" * 80)
                print(f"Task ID:    {task_id}")
                print(f"Timestamp:  {task_timestamp}")
                print(f"Prompt:     {task_data}")
                print("=" * 80)

                # Build complete prompt with task information and result upload instruction
                full_prompt = f"""Task ID: {task_id}
                Task: {task_data}
                Your name is: {args.name}
                After completing this task, upload the result and all intermediate information to the PFS system file located at **/s3fs/aws/result-{task_id}.txt**."""

                # Execute task with Claude Code
                result = executor.execute_task(
                    task_prompt=full_prompt, working_dir=args.working_dir
                )

                # Display results
                print("\n" + "=" * 80)
                print(f"📤 TASK EXECUTION RESULT")
                print("=" * 80)
                print(f"Task ID:    {task_id}")
                print(
                    f"Status:     {'✅ SUCCESS' if result['success'] else '❌ FAILED'}"
                )
                print(f"Duration:   {result['duration_ms']:.0f}ms")
                if result["total_cost_usd"] > 0:
                    print(f"Cost:       ${result['total_cost_usd']:.4f}")
                if result["session_id"]:
                    print(f"Session ID: {result['session_id']}")
                print("-" * 80)

                if result["success"]:
                    print("Result:")
                    print(result["result"])
                else:
                    print(f"Error: {result['error']}")

                print("=" * 80)
                print()

            else:
                # Queue is empty, wait before retrying
                print(
                    f"[{datetime.now().strftime('%H:%M:%S')}] Queue is empty, waiting for new tasks..."
                )
                time.sleep(args.poll_interval)

    except KeyboardInterrupt:
        print("\n\n⏹️  Program stopped by user")
        sys.exit(0)


if __name__ == "__main__":
    main()
