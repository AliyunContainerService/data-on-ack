"""Batch exact-trajectory deduplication with sharded Ray actors.

Each input is a (run_id, completed Harbor archive bytes) pair. Run-specific
timestamps, session IDs and token metrics are excluded from the fingerprint.
The result is an exact duplicate rate over distinct run IDs, not an estimate
of semantic similarity.
"""

import hashlib
import io
import json
import os
import sqlite3
import tarfile
import tempfile
from pathlib import Path

import ray

MAX_TRAJECTORY_BYTES = 20 * 1024 * 1024


def trajectory_digest(archive: bytes, run_id: str) -> str:
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tar:
        member = tar.getmember(f"{run_id}/agent/trajectory.json")
        if not member.isfile() or member.issym() or member.islnk() or member.size > MAX_TRAJECTORY_BYTES:
            raise ValueError(f"invalid agent trajectory for {run_id}")
        trajectory = json.load(tar.extractfile(member))
    steps = trajectory.get("steps") if isinstance(trajectory, dict) else trajectory
    if not isinstance(steps, list) or not steps:
        raise ValueError(f"empty or invalid agent trajectory for {run_id}")
    normalized = []
    for step in steps:
        if not isinstance(step, dict):
            raise ValueError(f"malformed step in {run_id}")
        normalized.append({
            "source": step.get("source") or step.get("role"),
            "message": step.get("message") or step.get("content"),
            "tool_calls": [
                {"function_name": call.get("function_name"), "arguments": call.get("arguments")}
                for call in step.get("tool_calls", []) if isinstance(call, dict)
            ],
            "observation": step.get("observation"),
        })
    canonical = json.dumps(normalized, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(canonical).hexdigest()


@ray.remote(num_cpus=0.1)
def fingerprint(run_id: str, archive: bytes):
    try:
        return run_id, trajectory_digest(archive, run_id), None
    except (ValueError, KeyError, tarfile.TarError, json.JSONDecodeError) as exc:
        return run_id, None, str(exc)


@ray.remote(num_cpus=0.2)
class DedupShard:
    def __init__(self, shard_id: int, work_dir: str):
        root = Path(work_dir)
        root.mkdir(parents=True, exist_ok=True)
        self.db = sqlite3.connect(root / f"shard-{shard_id}.sqlite3")
        self.db.execute("CREATE TABLE IF NOT EXISTS members (run_id TEXT PRIMARY KEY, digest TEXT NOT NULL)")
        self.db.execute("CREATE INDEX IF NOT EXISTS members_digest ON members(digest)")
        self.db.commit()

    def add(self, run_id: str, digest: str):
        prior = self.db.execute("SELECT digest FROM members WHERE run_id=?", (run_id,)).fetchone()
        if prior and prior[0] != digest:
            raise ValueError(f"run ID {run_id} changed fingerprint")
        if prior:
            return False
        self.db.execute("INSERT INTO members VALUES (?, ?)", (run_id, digest))
        self.db.commit()
        return True

    def counts(self):
        total = self.db.execute("SELECT COUNT(*) FROM members").fetchone()[0]
        unique = self.db.execute("SELECT COUNT(DISTINCT digest) FROM members").fetchone()[0]
        return total, unique

    def finalize(self):
        self.db.execute("CREATE TABLE IF NOT EXISTS groups AS SELECT digest, MIN(run_id) representative FROM members GROUP BY digest")
        self.db.execute("CREATE UNIQUE INDEX IF NOT EXISTS groups_digest ON groups(digest)")
        self.db.commit()

    def duplicates(self, after_digest: str, after_run_id: str, limit: int):
        return self.db.execute("""
            SELECT m.run_id, m.digest, g.representative
            FROM members m
            JOIN groups g ON m.digest=g.digest
            WHERE m.run_id != g.representative
              AND (m.digest > ? OR (m.digest = ? AND m.run_id > ?))
            ORDER BY m.digest, m.run_id LIMIT ?
        """, (after_digest, after_digest, after_run_id, limit)).fetchall()


def deduplicate(records, output_jsonl: str, shards: int = 8, batch_size: int = 32):
    """Consume a streaming iterable; keep at most one batch of archives in flight."""
    if shards < 1 or batch_size < 1:
        raise ValueError("shards and batch_size must be positive")
    work_dir = tempfile.mkdtemp(prefix="ray-trajectory-dedup-")
    actors = [DedupShard.remote(i, work_dir) for i in range(shards)]
    batch = []
    output = Path(output_jsonl)
    output.parent.mkdir(parents=True, exist_ok=True)
    reject_path = output.with_suffix(output.suffix + ".rejected.jsonl")
    reject_dest = reject_path.open("w")
    rejected_count = 0

    def flush():
        nonlocal rejected_count
        if not batch:
            return
        fingerprints = ray.get([fingerprint.remote(run_id, archive) for run_id, archive in batch])
        for run_id, digest, error in fingerprints:
            if error:
                reject_dest.write(json.dumps({"run_id": run_id, "reason": error}, ensure_ascii=False) + "\n")
                rejected_count += 1
        ray.get([actors[int(digest[:16], 16) % shards].add.remote(run_id, digest)
                 for run_id, digest, error in fingerprints if digest])
        batch.clear()

    for record in records:
        batch.append(record)
        if len(batch) >= batch_size:
            flush()
    flush()
    reject_dest.close()
    ray.get([actor.finalize.remote() for actor in actors])
    counts = ray.get([actor.counts.remote() for actor in actors])
    total = sum(c[0] for c in counts)
    unique = sum(c[1] for c in counts)
    with output.open("w") as dest:
        for actor in actors:
            cursor = ("", "")
            while True:
                rows = ray.get(actor.duplicates.remote(*cursor, 500))
                if not rows:
                    break
                for run_id, digest, representative in rows:
                    dest.write(json.dumps({"run_id": run_id, "representative_run_id": representative,
                                           "trajectory_sha256": digest}, ensure_ascii=False) + "\n")
                cursor = (rows[-1][1], rows[-1][0])
    return {"total_trajectories": total, "unique_trajectories": unique,
            "duplicate_trajectories": total - unique,
            "duplicate_rate": (total - unique) / total if total else 0.0,
            "duplicate_manifest": str(output), "rejected_archives": rejected_count,
            "rejected_manifest": str(reject_path),
            "comparison": "exact_normalized_trajectory"}


def oss_records(uri: str, region: str, manifest: str = None):
    """Read an explicit OSS manifest or list a prefix when RRSA permits ListObjects."""
    from urllib.parse import urlparse
    import oss2
    from alibabacloud_credentials.client import Client
    from alibabacloud_credentials.models import Config

    parsed = urlparse(uri) if uri else None
    if not manifest and (parsed.scheme != "oss" or not parsed.netloc or not parsed.path.lstrip("/")):
        raise ValueError("--oss-uri must be oss://bucket/prefix/")
    provider = Client(Config(
        type="oidc_role_arn",
        role_arn=os.environ["ALIBABA_CLOUD_ROLE_ARN"],
        oidc_provider_arn=os.environ["ALIBABA_CLOUD_OIDC_PROVIDER_ARN"],
        oidc_token_file_path=os.environ["ALIBABA_CLOUD_OIDC_TOKEN_FILE"],
        role_session_name="ack-rl-batch-dedup",
        sts_endpoint=f"sts.{region}.aliyuncs.com",
    ))
    if manifest:
        with open(manifest) as source:
            for line in source:
                object_uri = line.strip()
                if not object_uri or object_uri.startswith("#"):
                    continue
                obj = urlparse(object_uri)
                if obj.scheme != "oss" or not obj.netloc or not obj.path.endswith(".tar.gz"):
                    raise ValueError(f"invalid archive URI: {object_uri}")
                credentials = provider.get_credential()
                auth = oss2.StsAuth(credentials.access_key_id, credentials.access_key_secret, credentials.security_token)
                bucket = oss2.Bucket(auth, f"https://oss-{region}-internal.aliyuncs.com", obj.netloc)
                yield Path(obj.path).name.removesuffix(".tar.gz"), bucket.get_object(obj.path.lstrip("/")).read()
        return
    prefix = parsed.path.lstrip("/")
    token = ""
    while True:
        credentials = provider.get_credential()
        auth = oss2.StsAuth(credentials.access_key_id, credentials.access_key_secret, credentials.security_token)
        bucket = oss2.Bucket(auth, f"https://oss-{region}-internal.aliyuncs.com", parsed.netloc)
        page = bucket.list_objects_v2(prefix=prefix, continuation_token=token, max_keys=100)
        for obj in page.object_list:
            if obj.key.endswith(".tar.gz"):
                run_id = Path(obj.key).name.removesuffix(".tar.gz")
                yield run_id, bucket.get_object(obj.key).read()
        if not page.is_truncated:
            break
        token = page.next_continuation_token


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archives", nargs="*", type=Path)
    parser.add_argument("--oss-uri", help="OSS prefix of completed run archives")
    parser.add_argument("--oss-manifest", help="file with one OSS archive URI per line; requires GetObject only")
    parser.add_argument("--region", help="OSS region, required when reading OSS archives")
    parser.add_argument("--output", required=True)
    parser.add_argument("--shards", type=int, default=8)
    parser.add_argument("--batch-size", type=int, default=32)
    args = parser.parse_args()
    if sum((bool(args.archives), bool(args.oss_uri), bool(args.oss_manifest))) != 1:
        parser.error("provide local archives, --oss-uri, or --oss-manifest")
    if (args.oss_uri or args.oss_manifest) and not args.region:
        parser.error("--region is required for OSS archives")
    ray.init(address=os.environ.get("RAY_ADDRESS", "auto"))
    def records():
        for path in args.archives:
            yield path.name.removesuffix(".tar.gz"), path.read_bytes()
    source = oss_records(args.oss_uri, args.region, args.oss_manifest) if (args.oss_uri or args.oss_manifest) else records()
    print(json.dumps(deduplicate(source, args.output, args.shards, args.batch_size), indent=2))
