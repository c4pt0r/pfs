#!/usr/bin/env python3
"""
Parallel Research - Broadcast research tasks to multiple agent queues
and collect results from S3FS
"""

import argparse
import sys
import time
import uuid
from datetime import datetime
from typing import Any, Dict, List, Optional
from pyagfs import AGFSClient


class TaskBroadcaster:
    """AGFS QueueFS task broadcaster for multiple agent queues"""

    def __init__(
        self,
        agent_queues: List[str],
        agfs_api_baseurl: Optional[str] = "http://localhost:8080",
    ):
        """
        Initialize task broadcaster

        Args:
            agent_queues: List of agent queue paths (e.g., ["/queuefs/agent1", "/queuefs/agent2"])
            agfs_api_baseurl: AGFS API server URL (optional)
        """
        self.agent_queues = agent_queues
        self.agfs_api_baseurl = agfs_api_baseurl
        self.client = AGFSClient(agfs_api_baseurl)

    def enqueue_task(self, queue_path: str, task_data: str) -> bool:
        """
        Enqueue a task to a specific queue

        Args:
            queue_path: Queue path (e.g., "/queuefs/agent1")
            task_data: Task data to enqueue

        Returns:
            True if successful, False otherwise
        """
        enqueue_path = f"{queue_path}/enqueue"

        try:
            # Write task data to enqueue path using pyagfs client
            self.client.write(enqueue_path, task_data.encode('utf-8'))
            return True

        except Exception as e:
            print(f"Error enqueueing to {queue_path}: {e}", file=sys.stderr)
            return False

    def broadcast_task(self, task_data: str) -> Dict[str, bool]:
        """
        Broadcast a task to all agent queues

        Args:
            task_data: Task data to broadcast

        Returns:
            Dictionary mapping queue paths to success status
        """
        results = {}

        print(f"\n{'='*80}")
        print(f"📡 BROADCASTING TASK TO {len(self.agent_queues)} AGENTS")
        print(f"{'='*80}")
        print(f"Task: {task_data}")
        print(f"{'='*80}\n")

        for queue_path in self.agent_queues:
            print(f"📤 Sending to {queue_path}...", end=" ")
            success = self.enqueue_task(queue_path, task_data)
            results[queue_path] = success

            if success:
                print("✅ Success")
            else:
                print("❌ Failed")

        print()
        return results


class ResultsCollector:
    """Collect and monitor results from S3FS"""

    def __init__(
        self,
        results_path: str,
        agfs_api_baseurl: Optional[str] = "http://localhost:8080",
    ):
        """
        Initialize results collector

        Args:
            results_path: S3FS path where results are stored
            agfs_api_baseurl: AGFS API server URL (optional)
        """
        self.results_path = results_path
        self.agfs_api_baseurl = agfs_api_baseurl
        self.client = AGFSClient(agfs_api_baseurl)

    def list_results(self) -> List[str]:
        """
        List all result files in the results directory

        Returns:
            List of result file paths
        """
        try:
            # List directory and extract file names
            files = self.client.ls(self.results_path)
            return [f['name'] for f in files if not f.get('isDir', False)]
        except Exception:
            return []

    def read_result(self, result_file: str) -> Optional[str]:
        """
        Read a result file

        Args:
            result_file: Result file name

        Returns:
            File content, None if failed
        """
        file_path = f"{self.results_path}/{result_file}"
        try:
            content = self.client.cat(file_path)
            return content.decode('utf-8')
        except Exception:
            return None

    def wait_for_results(
        self,
        expected_count: int,
        timeout: int = 600,
        poll_interval: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Wait for all agents to complete and collect results

        Args:
            expected_count: Number of results to wait for
            timeout: Maximum wait time in seconds
            poll_interval: How often to check for new results (in seconds)

        Returns:
            List of result dictionaries
        """
        print(f"\n{'='*80}")
        print(f"⏳ WAITING FOR {expected_count} AGENT RESULTS")
        print(f"{'='*80}")
        print(f"Results path: {self.results_path}")
        print(f"Timeout: {timeout}s")
        print(f"{'='*80}\n")

        start_time = time.time()
        collected_results = []
        seen_files = set()

        while len(collected_results) < expected_count:
            # Check timeout
            elapsed = time.time() - start_time
            if elapsed > timeout:
                print(f"\n⏱️ Timeout reached after {elapsed:.0f}s")
                print(f"Collected {len(collected_results)}/{expected_count} results")
                break

            # List current results
            result_files = self.list_results()

            # Process new files
            for file_name in result_files:
                if file_name not in seen_files:
                    content = self.read_result(file_name)
                    if content:
                        collected_results.append({
                            "file_name": file_name,
                            "content": content,
                            "timestamp": datetime.now().isoformat()
                        })
                        seen_files.add(file_name)

                        print(f"📥 Result {len(collected_results)}/{expected_count}: {file_name}")

            # Check if we have all results
            if len(collected_results) >= expected_count:
                break

            # Wait before next check
            remaining = expected_count - len(collected_results)
            print(f"[{datetime.now().strftime('%H:%M:%S')}] "
                  f"Waiting for {remaining} more result(s)... "
                  f"(elapsed: {elapsed:.0f}s)")
            time.sleep(poll_interval)

        print(f"\n{'='*80}")
        print(f"✅ COLLECTION COMPLETE: {len(collected_results)}/{expected_count} results")
        print(f"{'='*80}\n")

        return collected_results


def main():
    """Main function: broadcast research tasks to multiple agents"""

    parser = argparse.ArgumentParser(
        description="Broadcast research tasks to multiple agent queues and collect results"
    )

    # Task parameters
    parser.add_argument(
        "task",
        type=str,
        help="Research task description to broadcast"
    )
    parser.add_argument(
        "--task-id",
        type=str,
        default=None,
        help="Task ID (auto-generated if not specified)"
    )

    # Agent queue parameters
    parser.add_argument(
        "--agents",
        type=str,
        default="agent1,agent2,agent3",
        help="Comma-separated list of agent names (default: agent1,agent2,agent3)"
    )
    parser.add_argument(
        "--queue-prefix",
        type=str,
        default="/queuefs",
        help="Queue path prefix (default: /queuefs)"
    )

    # Results parameters
    parser.add_argument(
        "--results-path",
        type=str,
        default="/s3fs/aws/results",
        help="S3FS path for storing results (default: /s3fs/aws/results)"
    )
    parser.add_argument(
        "--wait",
        action="store_true",
        help="Wait for all agents to complete and collect results"
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=600,
        help="Timeout for waiting results in seconds (default: 600)"
    )
    parser.add_argument(
        "--poll-interval",
        type=int,
        default=5,
        help="Interval for checking results in seconds (default: 5)"
    )

    # AGFS API parameters
    parser.add_argument(
        "--api-url",
        type=str,
        default=None,
        help="AGFS API server URL (optional)"
    )

    args = parser.parse_args()

    # Generate task ID if not provided
    task_id = args.task_id or str(uuid.uuid4())

    # Parse agent names and create queue paths
    agent_names = [name.strip() for name in args.agents.split(",")]
    agent_queues = [f"{args.queue_prefix}/{name}" for name in agent_names]

    # Create task broadcaster
    broadcaster = TaskBroadcaster(
        agent_queues=agent_queues,
        agfs_api_baseurl=args.api_url
    )

    # Create results path for this task
    task_results_path = f"{args.results_path}/{task_id}"

    # Build the task prompt
    task_prompt = f"""Research Task ID: {task_id}

Research Topic: {args.task}

Instructions:
1. Research the topic thoroughly from your assigned perspective
2. Provide detailed findings, insights, and recommendations
3. Save your complete results to: {task_results_path}/agent-${{YOUR_AGENT_NAME}}.txt

Make sure to include:
- Your research methodology
- Key findings and insights
- References or sources (if applicable)
- Your conclusions and recommendations
"""

    print("\n" + "="*80)
    print("🔬 PARALLEL RESEARCH TASK BROADCASTER")
    print("="*80)
    print(f"Task ID:      {task_id}")
    print(f"Research:     {args.task}")
    print(f"Agents:       {', '.join(agent_names)} ({len(agent_names)} total)")
    print(f"Results path: {task_results_path}")
    print(f"Wait mode:    {'Enabled' if args.wait else 'Disabled'}")
    print("="*80)

    # Broadcast task to all agents
    results = broadcaster.broadcast_task(task_prompt)

    # Count successful broadcasts
    success_count = sum(1 for success in results.values() if success)

    print(f"\n{'='*80}")
    print(f"📊 BROADCAST SUMMARY")
    print(f"{'='*80}")
    print(f"Total agents:      {len(agent_queues)}")
    print(f"Successful:        {success_count}")
    print(f"Failed:            {len(agent_queues) - success_count}")
    print(f"{'='*80}\n")

    if success_count == 0:
        print("❌ No tasks were successfully broadcasted!")
        sys.exit(1)

    # Wait for results if requested
    if args.wait:
        collector = ResultsCollector(
            results_path=task_results_path,
            agfs_api_baseurl=args.api_url
        )

        collected_results = collector.wait_for_results(
            expected_count=success_count,
            timeout=args.timeout,
            poll_interval=args.poll_interval
        )

        # Display collected results
        if collected_results:
            print(f"\n{'='*80}")
            print(f"📋 COLLECTED RESULTS")
            print(f"{'='*80}\n")

            for i, result in enumerate(collected_results, 1):
                print(f"\n--- Result {i}: {result['file_name']} ---")
                print(f"Timestamp: {result['timestamp']}")
                print(f"\nContent:\n{result['content']}")
                print("-" * 80)
        else:
            print("\n⚠️  No results were collected within the timeout period")
    else:
        print("💡 Tip: Use --wait to automatically collect results when agents complete")
        print(f"💡 Results will be saved to: {task_results_path}/")

    print("\n✅ Done!")


if __name__ == "__main__":
    main()
