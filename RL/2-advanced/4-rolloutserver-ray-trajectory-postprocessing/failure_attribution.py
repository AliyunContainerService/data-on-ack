"""Evidence-bound attribution for completed Harbor runs."""
import hashlib
import io
import json
import re
import tarfile

VERSION = "verifier-failure-v3"
MAX_MEMBER = 20 * 1024 * 1024


def _member(tar, run_id, suffix):
    member = tar.getmember(f"{run_id}/{suffix}")
    if not member.isfile() or member.issym() or member.islnk() or member.size > MAX_MEMBER:
        raise ValueError(f"invalid {suffix} member")
    return tar.extractfile(member).read()


def _step_evidence(step):
    return {
        "step_id": step["step_id"],
        "agent_message": str(step.get("message", ""))[:350],
        "commands": [str(c.get("arguments", {}).get("command", ""))[:500] for c in step.get("tool_calls", []) if isinstance(c, dict)],
    }


def attribute_archive(archive, run_id, signal_uri):
    archive_hash = hashlib.sha256(archive).hexdigest()
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tar:
        trajectory = json.loads(_member(tar, run_id, "agent/trajectory.json"))
        ctrf = json.loads(_member(tar, run_id, "verifier/ctrf.json"))
        reward = float(_member(tar, run_id, "verifier/reward.txt").decode().strip())
        verifier_stdout = _member(tar, run_id, "verifier/test-stdout.txt").decode(errors="replace")
    steps = trajectory.get("steps")
    tests = ctrf.get("results", {}).get("tests")
    if not isinstance(steps, list) or not isinstance(tests, list) or not steps:
        raise ValueError("missing agent steps or verifier tests")
    summary = ctrf["results"].get("summary", {})
    base = {
        "run_id": run_id,
        "analysis_kind": "verifier_step_attribution",
        "analysis_version": VERSION,
        "signal_uri": signal_uri,
        "input_sha256": archive_hash,
        "trajectory_uri": f"{signal_uri}#agent/trajectory.json",
        "verifier_reward": reward,
        "verifier_summary": {k: summary.get(k) for k in ("tests", "passed", "failed", "skipped")},
        "verifier_stdout_sha256": hashlib.sha256(verifier_stdout.encode()).hexdigest(),
        "step_count": len(steps),
    }
    failures = [t for t in tests if str(t.get("status", "")).lower() in ("failed", "fail", "error")]
    if reward > 0 and not failures:
        base["conclusion"] = "success"
        base["reason"] = "Verifier tests passed and the recorded reward is positive."
        base["findings"] = []
        return base
    base["conclusion"] = "failure"
    findings = []
    for test in failures:
        trace = str(test.get("trace") or test.get("message") or "")
        name = str(test.get("name", ""))
        finding = {
            "failed_test": name,
            "verifier_evidence": "verifier/ctrf.json",
            "verifier_trace_excerpt": trace[-1800:],
            "verifier_stdout_file": "verifier/test-stdout.txt",
            "root_cause": "unknown",
            "agent_evidence": [],
            "uncertainty": "The verifier failure is observed; the contributing agent step has not been established.",
        }
        # Files left in the target directory are a concrete verifier mismatch.
        expected = re.search(r"Expected only ([\w.]+), found: \[([^\]]+)\]", trace)
        if expected:
            wanted = expected.group(1)
            extras = [x.strip(" '\"\n") for x in expected.group(2).split(",")]
            extras = [x for x in extras if x and x != wanted]
            refs = []
            for step in steps:
                if not isinstance(step, dict) or not isinstance(step.get("step_id"), int):
                    continue
                commands = [str(c.get("arguments", {}).get("command", "")) for c in step.get("tool_calls", []) if isinstance(c, dict)]
                if any(("rustc" in cmd or "g++" in cmd) and any(re.search(r"(?:^|[/ ])" + re.escape(extra) + r"(?:\s|$)", cmd) for extra in extras) for cmd in commands):
                    refs.append(_step_evidence(step))
            finding["root_cause"] = f"Agent left generated files {extras} in the target directory; verifier requires only {wanted}."
            finding["agent_evidence"] = refs
            finding["uncertainty"] = "The verifier shows the extra files at test time; referenced commands explain how they were created. Later cleanup, if any, was not observed in the trajectory."
        findings.append(finding)
    if not findings:
        findings.append({"root_cause": "unknown", "verifier_evidence": "verifier/ctrf.json", "uncertainty": "Reward is zero but CTRF has no failed test details."})
    base["findings"] = findings
    return base
