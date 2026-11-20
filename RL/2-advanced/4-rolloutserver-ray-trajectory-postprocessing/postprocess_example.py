"""Illustrative, deterministic triage of one Rollout Server archive.

Usage: python postprocess_example.py RUN_ID.tar.gz output.json
The caller supplies the archive (for example after an OSS event) and writes the
result under a versioned output prefix. No model-generated causal claim is made.
"""

import hashlib
import io
import json
import sys
import tarfile
from pathlib import Path


VERSION = "example-v1"
MAX_MEMBER_BYTES = 20 * 1024 * 1024


def read_json_member(tar, suffix):
    matches = [m for m in tar.getmembers() if m.name.endswith("/" + suffix)]
    if len(matches) != 1 or not matches[0].isfile() or matches[0].size > MAX_MEMBER_BYTES:
        raise ValueError(f"expected one regular {suffix} below {MAX_MEMBER_BYTES} bytes")
    return json.load(tar.extractfile(matches[0]))


def steps_from(trajectory):
    if isinstance(trajectory, list):
        return trajectory
    if isinstance(trajectory, dict):
        for key in ("steps", "messages", "turns"):
            if isinstance(trajectory.get(key), list):
                return trajectory[key]
    raise ValueError("trajectory has no steps/messages/turns list")


def failed_tests_from(ctrf):
    tests = ctrf.get("results", {}).get("tests", [])
    if not isinstance(tests, list):
        raise ValueError("CTRF results.tests must be a list")
    return [
        {"name": t.get("name", "<unnamed>"), "message": str(t.get("message") or "")[:1000]}
        for t in tests
        if isinstance(t, dict) and str(t.get("status", "")).lower() in {"failed", "fail", "error"}
    ]


def analyze(archive, run_id):
    digest = hashlib.sha256(archive).hexdigest()
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tar:
        trajectory = read_json_member(tar, "agent/trajectory.json")
        ctrf = read_json_member(tar, "verifier/ctrf.json")
    steps = steps_from(trajectory)
    failures = failed_tests_from(ctrf)

    # Exact content deduplication: stable across JSON key order, but never merge
    # different verifier outcomes or silently discard their run IDs.
    canonical = json.dumps(
        {"trajectory": trajectory, "failed_tests": failures},
        sort_keys=True, ensure_ascii=False, separators=(",", ":"),
    ).encode()
    content_hash = hashlib.sha256(canonical).hexdigest()

    findings = []
    for test in failures:
        # A lexical overlap is only a candidate link, not proof of causation.
        tokens = {w.lower() for w in test["name"].replace("_", " ").split() if len(w) >= 4}
        refs = [
            i for i, step in enumerate(steps)
            if isinstance(step, dict) and tokens.intersection(
                str(step.get("content") or step.get("text") or "").lower().split()
            )
        ][:5]
        findings.append({
            "failed_test": test["name"],
            "verifier_evidence": "verifier/ctrf.json",
            "step_ids": refs,
            "observation": test["message"],
            "likely_cause": "unknown",
            "confidence": "unknown",
        })

    issues = []
    if not steps:
        issues.append("empty_trajectory")
    if any(not isinstance(step, dict) for step in steps):
        issues.append("non_object_step")
    if not isinstance(ctrf.get("results", {}).get("tests"), list):
        issues.append("missing_verifier_tests")
    if failures and not any(t["message"] for t in failures):
        issues.append("failed_tests_lack_messages")

    return {
        "run_id": run_id,
        "analysis_version": VERSION,
        "input_sha256": digest,
        "dedup_key": f"{VERSION}:{content_hash}",
        "quality": {"passed": not issues, "issues": issues},
        "failed_tests": failures,
        "findings": findings,
        "step_count": len(steps),
    }


def main():
    if len(sys.argv) != 3:
        raise SystemExit("usage: postprocess_example.py RUN_ID.tar.gz output.json")
    source, output = map(Path, sys.argv[1:])
    result = analyze(source.read_bytes(), source.name.removesuffix(".tar.gz"))
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(f"quality_passed={result['quality']['passed']} dedup_key={result['dedup_key']}")


if __name__ == "__main__":
    main()
