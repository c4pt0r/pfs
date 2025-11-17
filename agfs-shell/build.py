#!/usr/bin/env python3
"""
Build script for agfs-shell
Creates a portable distribution with embedded dependencies using virtual environment
Requires Python 3.10+ on target system, but includes all dependencies
"""
import os
import sys
import subprocess
import shutil
from pathlib import Path
from datetime import datetime

def get_git_hash():
    """Get current git commit hash"""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            check=True
        )
        return result.stdout.strip()
    except:
        return "unknown"

def inject_version_info(script_dir):
    """Inject git hash and build date into version.py"""
    version_file = script_dir / "agfscli" / "version.py"
    git_hash = get_git_hash()
    build_date = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    # Read current version file
    with open(version_file, 'r') as f:
        content = f.read()

    # Replace placeholders
    content = content.replace('__git_hash__ = "dev"', f'__git_hash__ = "{git_hash}"')
    content = content.replace('__build_date__ = "dev"', f'__build_date__ = "{build_date}"')

    # Write back
    with open(version_file, 'w') as f:
        f.write(content)

    print(f"Injected version info: git={git_hash}, date={build_date}")

def restore_version_file(script_dir):
    """Restore version.py to dev state"""
    version_file = script_dir / "agfscli" / "version.py"

    with open(version_file, 'r') as f:
        content = f.read()

    # Restore to dev placeholders
    import re
    content = re.sub(r'__git_hash__ = ".*?"', '__git_hash__ = "dev"', content)
    content = re.sub(r'__build_date__ = ".*?"', '__build_date__ = "dev"', content)

    with open(version_file, 'w') as f:
        f.write(content)


def main():
    # Get the directory containing this script
    script_dir = Path(__file__).parent.absolute()
    dist_dir = script_dir / "dist"
    portable_dir = dist_dir / "agfs-portable"

    print("Building portable agfs distribution...")

    # Clean previous builds
    if portable_dir.exists():
        shutil.rmtree(portable_dir)
    portable_dir.mkdir(parents=True, exist_ok=True)

    # Inject version information
    inject_version_info(script_dir)

    try:
        # Check if uv is available
        has_uv = shutil.which("uv") is not None

        if not has_uv:
            print("Error: uv is required for building")
            print("Install uv: curl -LsSf https://astral.sh/uv/install.sh | sh")
            sys.exit(1)

        print("Installing dependencies to portable directory...")
        # Install dependencies directly to a lib directory (no venv)
        lib_dir = portable_dir / "lib"

        # First copy pyagfs SDK source directly (bypass uv's editable mode)
        pyagfs_src_dir = script_dir.parent / "agfs-sdk" / "python" / "pyagfs"
        if pyagfs_src_dir.exists():
            print(f"Copying local pyagfs from {pyagfs_src_dir}...")
            pyagfs_dest_dir = lib_dir / "pyagfs"
            shutil.copytree(pyagfs_src_dir, pyagfs_dest_dir)

            # Also install pyagfs dependencies
            pyagfs_project_dir = script_dir.parent / "agfs-sdk" / "python"
            subprocess.check_call([
                "uv", "pip", "install",
                "--target", str(lib_dir),
                "--python", sys.executable,
                "requests>=2.31.0"  # Install pyagfs's dependencies with their transitive deps
            ], cwd=str(script_dir))
        else:
            print(f"Warning: pyagfs SDK not found at {pyagfs_src_dir}")

        # Then install agfs-cli and remaining dependencies
        subprocess.check_call([
            "uv", "pip", "install",
            "--target", str(lib_dir),
            "--python", sys.executable,
            "--no-deps",  # Don't install dependencies, we'll do it separately
            str(script_dir)
        ], cwd=str(script_dir))

        # Install agfs-cli dependencies (excluding pyagfs which we already copied)
        subprocess.check_call([
            "uv", "pip", "install",
            "--target", str(lib_dir),
            "--python", sys.executable,
            "prompt-toolkit>=3.0.0",
            "rich>=13.0.0",
            "click>=8.0.0"
        ], cwd=str(script_dir))

        # Create launcher script
        print("Creating launcher scripts...")
        launcher_script = portable_dir / "agfs"
        launcher_content = '''#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""AGFS CLI Launcher
Portable launcher script that uses system Python but bundled dependencies
"""
import sys
import os

# Resolve the real path of this script (follow symlinks)
script_path = os.path.realpath(__file__)
script_dir = os.path.dirname(script_path)

# Add lib directory to Python path
lib_dir = os.path.join(script_dir, 'lib')
sys.path.insert(0, lib_dir)

# Run the CLI
from agfscli.cli import main

if __name__ == '__main__':
    main()
'''
        with open(launcher_script, 'w') as f:
            f.write(launcher_content)
        os.chmod(launcher_script, 0o755)

        # Create Windows launcher
        launcher_bat = portable_dir / "agfs.bat"
        with open(launcher_bat, 'w') as f:
            f.write("""@echo off
REM AGFS CLI Launcher for Windows
python "%~dp0agfs" %%*
""")

        # Create README
        readme = portable_dir / "README.txt"
        version_info = get_version_string()
        with open(readme, 'w') as f:
            f.write(f"""AGFS CLI - Portable Distribution
================================

Version: {version_info}
Built: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}
Git: {get_git_hash()}

This is a portable distribution of agfs-cli that includes all dependencies
in a bundled virtual environment.

Requirements:
- Python 3.10 or higher on the system
- No additional Python packages needed

Usage:
  ./agfs shell
  ./agfs ls /
  ./agfs tree /

Installation:
  You can move this entire directory anywhere and run ./agfs directly.
  Optionally, add it to your PATH or symlink ./agfs to /usr/local/bin/agfs

Environment Variables:
  AGFS_API_URL - Override default API endpoint (default: http://localhost:8080/api/v1)

Example:
  AGFS_API_URL=http://remote-server:8080/api/v1 ./agfs shell
""")

        # Calculate size
        total_size = sum(f.stat().st_size for f in portable_dir.rglob('*') if f.is_file())

        print(f"\nBuild successful!")
        print(f"Portable directory: {portable_dir}")
        print(f"Size: {total_size / 1024 / 1024:.2f} MB")
        print(f"\nUsage:")
        print(f"  {portable_dir}/agfs --help")
        print(f"  {portable_dir}/agfs shell")
        print(f"\nTo install, run: ./install.sh")

    finally:
        # Always restore version file to dev state
        restore_version_file(script_dir)

def get_version_string():
    """Get version string for README"""
    try:
        # Read from version.py
        version_file = Path(__file__).parent / "agfscli" / "version.py"
        namespace = {}
        with open(version_file) as f:
            exec(f.read(), namespace)

        version = namespace.get('__version__', '1.0.0')
        git_hash = namespace.get('__git_hash__', 'dev')
        build_date = namespace.get('__build_date__', 'dev')

        if git_hash == 'dev':
            return f"{version} (dev)"
        return f"{version} (git: {git_hash}, built: {build_date})"
    except:
        return "1.0.0"

if __name__ == "__main__":
    main()
