#!/usr/bin/env python3
"""
HackerNews Research - Fetch top HackerNews stories, distribute to agents for summarization,
and compile a comprehensive report
"""

import argparse
import json
import sys
import time
import uuid
from datetime import datetime
from typing import Any, Dict, List, Optional

import requests
from bs4 import BeautifulSoup
from pyagfs import AGFSClient


def fetch_hackernews_top_stories(count: int = 10) -> List[Dict[str, Any]]:
    """
    Fetch top stories from HackerNews

    Args:
        count: Number of stories to fetch (default: 10)

    Returns:
        List of story dictionaries with title, url, score, etc.
    """
    print(f"\n{'=' * 80}")
    print(f"🔍 FETCHING TOP {count} HACKERNEWS STORIES")
    print(f"{'=' * 80}\n")

    try:
        # Fetch top story IDs from HackerNews API
        response = requests.get(
            "https://hacker-news.firebaseio.com/v0/topstories.json", timeout=10
        )
        response.raise_for_status()
        story_ids = response.json()[:count]

        stories = []
        for i, story_id in enumerate(story_ids, 1):
            try:
                # Fetch story details
                story_response = requests.get(
                    f"https://hacker-news.firebaseio.com/v0/item/{story_id}.json",
                    timeout=10,
                )
                story_response.raise_for_status()
                story = story_response.json()

                if story and "url" in story:
                    stories.append(
                        {
                            "id": story_id,
                            "title": story.get("title", "No title"),
                            "url": story.get("url", ""),
                            "score": story.get("score", 0),
                            "by": story.get("by", "unknown"),
                            "time": story.get("time", 0),
                            "descendants": story.get("descendants", 0),
                        }
                    )

                    print(f"✅ [{i}/{count}] {story.get('title', 'No title')}")
                    print(f"    URL: {story.get('url', 'N/A')}")
                    print(
                        f"    Score: {story.get('score', 0)} | "
                        f"Comments: {story.get('descendants', 0)}\n"
                    )

            except Exception as e:
                print(f"⚠️  [{i}/{count}] Failed to fetch story {story_id}: {e}\n")
                continue

        print(f"{'=' * 80}")
        print(f"✅ Successfully fetched {len(stories)} stories")
        print(f"{'=' * 80}\n")

        return stories

    except Exception as e:
        print(f"❌ Error fetching HackerNews stories: {e}")
        return []


def distribute_stories_to_agents(
    stories: List[Dict[str, Any]],
    agent_names: List[str],
    task_id: str,
    results_path: str,
    queue_prefix: str = "/queuefs",
    agfs_api_url: Optional[str] = None,
) -> Dict[str, int]:
    """
    Distribute stories among agents for parallel processing

    Args:
        stories: List of story dictionaries
        agent_names: List of agent names
        task_id: Task ID for this research job
        results_path: S3FS path for results
        queue_prefix: Queue path prefix
        agfs_api_url: AGFS API URL

    Returns:
        Dictionary mapping agent names to number of stories assigned
    """
    print(f"\n{'=' * 80}")
    print(f"📡 DISTRIBUTING {len(stories)} STORIES TO {len(agent_names)} AGENTS")
    print(f"{'=' * 80}\n")

    # Distribute stories evenly among agents
    stories_per_agent = {}
    for i, story in enumerate(stories):
        agent_idx = i % len(agent_names)
        agent_name = agent_names[agent_idx]

        if agent_name not in stories_per_agent:
            stories_per_agent[agent_name] = []

        stories_per_agent[agent_name].append(story)

    # Send tasks to each agent
    assignment = {}
    for agent_name, agent_stories in stories_per_agent.items():
        # Build task prompt
        task_prompt = f"""HackerNews Research Task ID: {task_id}
Agent: {agent_name}

You have been assigned {len(agent_stories)} HackerNews articles to analyze and summarize.

STORIES TO ANALYZE:
"""
        for idx, story in enumerate(agent_stories, 1):
            task_prompt += f"""
{idx}. {story["title"]}
   URL: {story["url"]}
   Score: {story["score"]} | Author: {story["by"]} | Comments: {story["descendants"]}
"""

        task_prompt += f"""

INSTRUCTIONS:
1. For each story URL, fetch and read the content
2. Create a comprehensive summary including:
   - Main topic and key points
   - Technical insights (if applicable)
   - Significance and implications
   - Your analysis and commentary
   - Using Chinese to summary

3. Format your response as JSON with this structure:
{{
    "agent": "{agent_name}",
    "task_id": "{task_id}",
    "summaries": [
        {{
            "story_id": <id>,
            "title": "<title>",
            "url": "<url>",
            "summary": "<your summary>",
            "key_points": ["point1", "point2", ...],
            "analysis": "<your analysis>"
        }},
        ...
    ]
}}

4. Save your complete JSON results to !!!!agfs!!!! not local file system (use agfs tool to upload): {results_path}/{task_id}/agent-{agent_name}.json

Use the WebFetch tool to retrieve article content. Focus on extracting meaningful insights.
"""

        # Enqueue task
        queue_path = f"{queue_prefix}/{agent_name}"
        success = enqueue_task(queue_path, task_prompt, agfs_api_url)

        if success:
            assignment[agent_name] = len(agent_stories)
            print(f"✅ {agent_name}: {len(agent_stories)} stories assigned")
        else:
            assignment[agent_name] = 0
            print(f"❌ {agent_name}: Failed to assign stories")

    print(f"\n{'=' * 80}")
    print(f"✅ Distribution complete")
    print(f"{'=' * 80}\n")

    return assignment


def enqueue_task(
    queue_path: str, task_data: str, agfs_api_url: Optional[str] = None
) -> bool:
    """Enqueue a task to a specific queue"""
    enqueue_path = f"{queue_path}/enqueue"

    try:
        # Initialize AGFS client
        api_url = agfs_api_url or "http://localhost:8080"
        client = AGFSClient(api_url)

        # Write task data to enqueue path
        client.write(enqueue_path, task_data.encode("utf-8"))
        return True

    except Exception as e:
        print(f"Error enqueueing to {queue_path}: {e}", file=sys.stderr)
        return False


def wait_for_results(
    results_path: str,
    expected_count: int,
    timeout: int = 600,
    poll_interval: int = 5,
    agfs_api_url: Optional[str] = None,
) -> List[Dict[str, Any]]:
    """Wait for all agents to complete and collect results"""
    print(f"\n{'=' * 80}")
    print(f"⏳ WAITING FOR {expected_count} AGENT RESULTS")
    print(f"{'=' * 80}")
    print(f"Results path: {results_path}")
    print(f"Timeout: {timeout}s")
    print(f"{'=' * 80}\n")

    start_time = time.time()
    collected_results = []
    seen_files = set()

    while len(collected_results) < expected_count:
        elapsed = time.time() - start_time
        if elapsed > timeout:
            print(f"\n⏱️  Timeout reached after {elapsed:.0f}s")
            print(f"Collected {len(collected_results)}/{expected_count} results")
            break

        # List current results
        result_files = list_files(results_path, agfs_api_url)

        # Process new files
        for file_name in result_files:
            if file_name not in seen_files and file_name.endswith(".json"):
                content = read_file(f"{results_path}/{file_name}", agfs_api_url)
                if content:
                    try:
                        result_data = json.loads(content)
                        collected_results.append(
                            {
                                "file_name": file_name,
                                "data": result_data,
                                "timestamp": datetime.now().isoformat(),
                            }
                        )
                        seen_files.add(file_name)
                        print(
                            f"📥 Result {len(collected_results)}/{expected_count}: {file_name}"
                        )
                    except json.JSONDecodeError:
                        print(f"⚠️  Failed to parse JSON from {file_name}")

        if len(collected_results) >= expected_count:
            break

        remaining = expected_count - len(collected_results)
        print(
            f"[{datetime.now().strftime('%H:%M:%S')}] "
            f"Waiting for {remaining} more result(s)... (elapsed: {elapsed:.0f}s)"
        )
        time.sleep(poll_interval)

    print(f"\n{'=' * 80}")
    print(f"✅ COLLECTION COMPLETE: {len(collected_results)}/{expected_count} results")
    print(f"{'=' * 80}\n")

    return collected_results


def list_files(path: str, agfs_api_url: Optional[str] = None) -> List[str]:
    """List files in a AGFS directory"""
    try:
        # Initialize AGFS client
        api_url = agfs_api_url or "http://localhost:8080"
        client = AGFSClient(api_url)

        # List directory and extract file names
        files = client.ls(path)
        return [f["name"] for f in files if not f.get("isDir", False)]
    except Exception:
        pass
    return []


def read_file(file_path: str, agfs_api_url: Optional[str] = None) -> Optional[str]:
    """Read a file from AGFS"""
    try:
        # Initialize AGFS client
        api_url = agfs_api_url or "http://localhost:8080"
        client = AGFSClient(api_url)

        # Read file content
        content = client.cat(file_path)
        return content.decode("utf-8")
    except Exception:
        pass
    return None


def compile_final_report(
    results: List[Dict[str, Any]], stories: List[Dict[str, Any]], task_id: str
) -> str:
    """Compile all agent results into a final comprehensive report"""
    print(f"\n{'=' * 80}")
    print(f"📝 COMPILING FINAL REPORT")
    print(f"{'=' * 80}\n")

    report = f"""# HackerNews Top Stories Research Report
Task ID: {task_id}
Generated: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}

## Overview
This report summarizes the top {len(stories)} stories from HackerNews, analyzed by {len(results)} AI agents working in parallel.

---

## Story Summaries

"""

    # Organize summaries by story
    story_summaries = {}
    for result in results:
        agent_name = result["data"].get("agent", "unknown")
        summaries = result["data"].get("summaries", [])

        for summary in summaries:
            story_id = summary.get("story_id")
            if story_id not in story_summaries:
                story_summaries[story_id] = []
            story_summaries[story_id].append({"agent": agent_name, "summary": summary})

    # Build report for each story
    for i, story in enumerate(stories, 1):
        story_id = story["id"]
        report += f"\n### {i}. {story['title']}\n\n"
        report += f"**URL:** {story['url']}\n\n"
        report += f"**Stats:** {story['score']} points | "
        report += f"by {story['by']} | "
        report += f"{story['descendants']} comments\n\n"

        if story_id in story_summaries:
            for agent_summary in story_summaries[story_id]:
                agent = agent_summary["agent"]
                summary_data = agent_summary["summary"]

                report += f"#### Analysis by {agent}\n\n"
                report += f"**Summary:** {summary_data.get('summary', 'N/A')}\n\n"

                if summary_data.get("key_points"):
                    report += f"**Key Points:**\n"
                    for point in summary_data["key_points"]:
                        report += f"- {point}\n"
                    report += "\n"

                if summary_data.get("analysis"):
                    report += f"**Analysis:** {summary_data['analysis']}\n\n"

                report += "---\n\n"
        else:
            report += "*No analysis available for this story.*\n\n---\n\n"

    report += f"""
## Summary

- Total stories analyzed: {len(stories)}
- Agents involved: {len(results)}
- Task ID: {task_id}
- Completion time: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}

---

*Generated by AGFS Parallel Research System*
"""

    print(f"✅ Report compiled successfully")
    print(f"{'=' * 80}\n")

    return report


def save_report(
    report: str, report_path: str, agfs_api_url: Optional[str] = None
) -> bool:
    """Save the final report to AGFS"""
    print(f"💾 Saving report to: {report_path}")

    try:
        # Initialize AGFS client
        api_url = agfs_api_url or "http://localhost:8080"
        client = AGFSClient(api_url)

        # Write report content
        client.write(report_path, report.encode("utf-8"))
        print(f"✅ Report saved successfully\n")
        return True

    except Exception as e:
        print(f"❌ Error saving report: {e}\n")
        return False


def main():
    """Main function"""
    parser = argparse.ArgumentParser(
        description="Fetch and analyze top HackerNews stories using parallel agents"
    )

    parser.add_argument(
        "--count",
        type=int,
        default=10,
        help="Number of top stories to fetch (default: 10)",
    )
    parser.add_argument(
        "--agents",
        type=str,
        default="agent1,agent2,agent3,agent4,agent5,agent6,agent7,agent8,agent9,agent10",
        help="Comma-separated list of agent names (default: agent1,agent2,agent3,agent4,agent5,agent6,agent7,agent8,agent9,agent10)",
    )
    parser.add_argument(
        "--queue-prefix",
        type=str,
        default="/queuefs",
        help="Queue path prefix (default: /queuefs)",
    )
    parser.add_argument(
        "--results-path",
        type=str,
        default="/s3fs/aws/hackernews-results",
        help="S3FS path for storing results (default: /s3fs/aws/hackernews-results)",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=900,
        help="Timeout for waiting results in seconds (default: 900)",
    )
    parser.add_argument(
        "--api-url", type=str, default=None, help="AGFS API server URL (optional)"
    )

    args = parser.parse_args()

    # Generate task ID
    task_id = str(uuid.uuid4())[:8]

    print("\n" + "=" * 80)
    print("🔬 HACKERNEWS PARALLEL RESEARCH")
    print("=" * 80)
    print(f"Task ID:      {task_id}")
    print(f"Stories:      {args.count}")
    print(f"Agents:       {args.agents}")
    print(f"Results path: {args.results_path}/{task_id}")
    print("=" * 80)

    # Step 1: Fetch HackerNews stories
    stories = fetch_hackernews_top_stories(args.count)

    if not stories:
        print("❌ No stories fetched. Exiting.")
        sys.exit(1)

    # Step 2: Distribute to agents
    agent_names = [name.strip() for name in args.agents.split(",")]
    task_results_path = f"{args.results_path}/{task_id}"

    assignment = distribute_stories_to_agents(
        stories=stories,
        agent_names=agent_names,
        task_id=task_id,
        results_path=args.results_path,
        queue_prefix=args.queue_prefix,
        agfs_api_url=args.api_url,
    )

    successful_agents = sum(1 for count in assignment.values() if count > 0)

    if successful_agents == 0:
        print("❌ Failed to assign tasks to any agents. Exiting.")
        sys.exit(1)

    # Step 3: Wait for results
    results = wait_for_results(
        results_path=task_results_path,
        expected_count=successful_agents,
        timeout=args.timeout,
        poll_interval=10,
        agfs_api_url=args.api_url,
    )

    # Step 4: Compile final report
    if results:
        final_report = compile_final_report(results, stories, task_id)

        # Print report to console
        print("\n" + "=" * 80)
        print("📄 FINAL REPORT")
        print("=" * 80 + "\n")
        print(final_report)

        # Save report to AGFS
        report_path = f"{task_results_path}/FINAL_REPORT.md"
        save_report(final_report, report_path, args.api_url)

        print("=" * 80)
        print(f"✅ Research complete!")
        print(f"📁 Report saved to: {report_path}")
        print("=" * 80 + "\n")
    else:
        print("\n⚠️  No results collected. Cannot compile report.")
        sys.exit(1)


if __name__ == "__main__":
    main()
