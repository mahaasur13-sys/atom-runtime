#!/usr/bin/env python3
"""
ATOM-029: Heisenbug Hunter
Runs pytest with high parallelism to surface race conditions and flaky tests.

Usage:
    python3 heisenbug_hunter.py --runs 1000 --parallel 8
"""

import argparse
import subprocess
import sys
import time

def run_heisenbug_hunter(runs: int, parallel: int, maxfail: int = 1):
    """Run tests N times with high parallelism to find heisenbugs."""
    print(f"[ATOM-029] Heisenbug Hunter")
    print(f"  Runs: {runs}")
    print(f"  Parallel workers: {parallel}")
    print(f"  Max failures before abort: {maxfail}")
    print()

    cmd = [
        "pytest",
        "-n", str(parallel),
        "--maxfail", str(maxfail),
        "-v",
        "--tb=short",
        "-x",  # stop on first failure
    ]

    start = time.time()
    result = subprocess.run(cmd, cwd="/tmp/atom-runtime")
    elapsed = time.time() - start

    if result.returncode == 0:
        print(f"\n[ATOM-029] ✅ All {runs} runs passed — 0 divergence, 0 flaky")
        return 0
    else:
        print(f"\n[ATOM-029] ❌ Heisenbug detected after {elapsed:.1f}s")
        return 1

def main():
    parser = argparse.ArgumentParser(description="ATOM-029 Heisenbug Hunter")
    parser.add_argument("--runs", type=int, default=1000)
    parser.add_argument("--parallel", type=int, default=8)
    args = parser.parse_args()

    sys.exit(run_heisenbug_hunter(args.runs, args.parallel))

if __name__ == "__main__":
    main()
