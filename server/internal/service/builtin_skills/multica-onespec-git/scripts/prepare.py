"""Prepare an owned OneSpec branch and isolated worktree without remote writes."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import sys


def git(repo, *args):
    result = subprocess.run(["git", "-C", str(repo), *args], text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(result.stderr.strip())
    return result.stdout.strip()


def prepare(repo, task, branch, base, directory):
    repo = pathlib.Path(repo).resolve()
    destination = pathlib.Path(directory).resolve()
    git(repo, "check-ref-format", "--branch", branch)
    if not task or not branch.startswith(("release_", "hotfix_", "docs_", "main_")):
        raise ValueError("provide a task identity and a OneSpec branch name")
    commit = git(repo, "rev-parse", "--verify", "--end-of-options", base + "^{commit}")
    common = pathlib.Path(git(repo, "rev-parse", "--git-common-dir"))
    if not common.is_absolute():
        common = repo / common
    records = common.resolve() / "onespec"
    records.mkdir(exist_ok=True)
    identity = hashlib.sha256(branch.encode()).hexdigest()
    record = records / (identity + ".json")
    expected = {"task_id": task, "branch": branch, "base_commit": commit, "directory": str(destination)}
    if record.exists():
        previous = json.loads(record.read_text(encoding="utf-8"))
        if previous != expected:
            raise RuntimeError("existing branch belongs to another task, base or worktree")
        if destination.is_dir() and git(destination, "branch", "--show-current") == branch:
            git(destination, "merge-base", "--is-ancestor", commit, "HEAD")
            return previous
        raise RuntimeError("recorded worktree is missing or changed; inspect before recovery")
    # Exclusive creation also serializes two preparers for the same branch.
    with record.open("x", encoding="utf-8") as stream:
        json.dump(expected, stream, ensure_ascii=False)
    try:
        git(repo, "worktree", "add", "-b", branch, str(destination), commit)
    except Exception:
        record.unlink()
        raise
    return expected


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for field in ("repo", "task", "branch", "base", "directory"):
        parser.add_argument("--" + field, required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(prepare(**vars(args)), ensure_ascii=False))
    except (RuntimeError, ValueError, OSError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
