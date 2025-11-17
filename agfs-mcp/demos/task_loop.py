#!/usr/bin/env python3
"""
Task Loop - Fetch tasks from AGFS QueueFS and execute with Claude Code
"""

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime
from typing import Any, Dict, Optional
from pyagfs import AGFSClient


class TaskQueue:
    """AGFS QueueFS task queue client"""

    def __init__(
        self,
        queue_path,
        agfs_api_baseurl: Optional[str] = "http://localhost:8080",
    ):
        """
        Initialize task queue client

        Args:
            queue_path: QueueFS mount path
            agfs_api_baseurl: AGFS API server URL (optional)
        """
        self.queue_path = queue_path
        self.agfs_api_baseurl = agfs_api_baseurl
        self.dequeue_path = f"{queue_path}/dequeue"
        self.size_path = f"{queue_path}/size"
        self.peek_path = f"{queue_path}/peek"
        self.client = AGFSClient(agfs_api_baseurl)

    def ensure_queue_exists(self) -> bool:
        """
        Ensure queue directory exists, create if not

        Returns:
            True if queue exists or was created successfully, False otherwise
        """
        try:
            # Try to create the queue directory
            # QueueFS requires explicit mkdir to create queues
            self.client.mkdir(self.queue_path)
            print(f"Successfully created queue: {self.queue_path}", file=sys.stderr)
            return True
        except Exception as e:
            # If mkdir fails, check if it's because queue already exists
            error_msg = str(e).lower()
            if "exists" in error_msg or "already" in error_msg:
                # Queue already exists, this is fine
                return True
            else:
                # Other error occurred
                print(f"Failed to create queue: {self.queue_path}: {e}", file=sys.stderr)
                return False

    def get_queue_size(self) -> Optional[int]:
        """
        Get queue size

        Returns:
            Number of messages in queue, None if failed
        """
        try:
            content = self.client.cat(self.size_path)
            output = content.decode('utf-8').strip()
            return int(output)
        except ValueError:
            print(f"Warning: Cannot parse queue size: {output}", file=sys.stderr)
            return None
        except Exception:
            return None

    def peek_task(self) -> Optional[Dict[str, Any]]:
        """
        Peek at next task without removing it

        Returns:
            Task data dictionary, None if failed
        """
        try:
            content = self.client.cat(self.peek_path)
            output = content.decode('utf-8')
            return json.loads(output)
        except json.JSONDecodeError:
            print(f"Warning: Cannot parse JSON: {output}", file=sys.stderr)
            return None
        except Exception:
            return None

    def dequeue_task(self) -> Optional[Dict[str, Any]]:
        """
        Get a task from queue (removes it)

        Returns:
            Task data dictionary with format: {"id": "...", "data": "...", "timestamp": "..."}
            Returns None if queue is empty or operation failed
        """
        try:
            content = self.client.cat(self.dequeue_path)
            output = content.decode('utf-8')
            return json.loads(output)
        except json.JSONDecodeError:
            print(f"Warning: Cannot parse JSON: {output}", file=sys.stderr)
            return None
        except Exception:
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
            print(f"\n[Executing Claude Code with streaming output...]")
            print("-" * 80)
            start_time = time.time()

            # Use Popen to enable streaming output
            process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                cwd=working_dir,
            )

            # Stream stderr to console in real-time (Claude Code outputs logs to stderr)
            stdout_lines = []
            stderr_lines = []

            try:
                # Read stderr line by line and print to console
                while True:
                    stderr_line = process.stderr.readline()
                    if stderr_line:
                        print(stderr_line.rstrip(), file=sys.stderr)
                        stderr_lines.append(stderr_line)

                    # Check if process has finished
                    if process.poll() is not None:
                        # Read any remaining output
                        remaining_stderr = process.stderr.read()
                        if remaining_stderr:
                            print(remaining_stderr.rstrip(), file=sys.stderr)
                            stderr_lines.append(remaining_stderr)
                        break

                # Read all stdout (JSON output)
                stdout_data = process.stdout.read()
                stdout_lines.append(stdout_data)

            except KeyboardInterrupt:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                raise

            execution_time = (time.time() - start_time) * 1000  # Convert to ms
            print("-" * 80)

            stdout_output = ''.join(stdout_lines)
            stderr_output = ''.join(stderr_lines)

            if process.returncode == 0:
                try:
                    output = json.loads(stdout_output)
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
                        "result": stdout_output,
                        "error": f"Failed to parse JSON output: {e}",
                        "duration_ms": execution_time,
                        "total_cost_usd": 0.0,
                        "session_id": "",
                    }
            else:
                return {
                    "success": False,
                    "result": "",
                    "error": f"Claude Code exited with code {process.returncode}: {stderr_output}",
                    "duration_ms": execution_time,
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
        description="Fetch tasks from AGFS QueueFS and execute with Claude Code"
    )
    parser.add_argument(
        "--queue-path",
        type=str,
        default="/queuefs/agent",
        help="QueueFS mount path (default: /queuefs/agent)",
    )
    parser.add_argument(
        "--api-url", type=str, default="http://localhost:8080", help="AGFS API server URL (default: http://localhost:8080)"
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
    queue = TaskQueue(queue_path=args.queue_path, agfs_api_baseurl=args.api_url)

    # Ensure queue exists before starting
    if not queue.ensure_queue_exists():
        print(f"Error: Failed to ensure queue exists at {queue.queue_path}", file=sys.stderr)
        sys.exit(1)

    # Create Claude Code executor
    executor = ClaudeCodeExecutor(
        timeout=args.claude_timeout, allowed_tools=allowed_tools
    )

    print("=== AGFS Task Loop with Claude Code ===")
    print(f"Monitoring queue: {queue.queue_path}")
    if args.api_url:
        print(f"AGFS API URL: {args.api_url}")
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
                Your name is: {args.name}"""

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
