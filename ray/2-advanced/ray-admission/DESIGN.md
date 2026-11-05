# Queue permit lifecycle and storage

## Lifecycle

| State | Resource charge | Durable data | Allowed transitions |
| --- | --- | --- | --- |
| WAITING | None | None; Ray retries the admission RPC after actor failure | RESERVED, RELEASED |
| RESERVED | Full declared resources | Ticket, quota namespace, Job ID, resources | TRACKED, RELEASED |
| TRACKED | Full declared resources | Reservation plus task ObjectRefs or actor ID | RELEASED |
| RELEASED | None | Ticket tombstone | Terminal |

The controller writes a reservation before replying to `acquire`. The submitter
then submits to Ray and calls `track` or `track_actor`. The controller persists
the execution identity before acknowledging tracking. Task returns becoming
ready, actor state becoming DEAD, or an explicit release transitions the ticket
to RELEASED. That transition is persisted before the controller admits more work.
Requests without resource demand do not need a resource reservation.

A repeated acquire for a RESERVED or TRACKED ticket returns its original approval.
Concurrent WAITING calls with the same ticket share one future. Repeated tracking
of the same execution object is a no-op; replacing that object is rejected.
Repeated release is a no-op, and a RELEASED ticket cannot be acquired again.
Tombstones survive reconstruction so delayed retries cannot recreate a charge.

## Failure behavior

KV timeouts are ambiguous: a write may have committed before its response was
lost. On any transaction failure, the controller marks storage unhealthy,
rejects further mutations and admissions, and fails pending waiters. In-memory
usage must not be treated as authoritative in this condition. Restart the actor
with `ray.kill(controller, no_restart=False)` after fixing storage availability;
the constructor reads committed records before admitting work again.

Completion polling retries transient errors with a bounded exponential delay
(0.1 to 5 seconds). `status.health` exposes storage and monitor errors. Unknown
actor state retains the charge. Task polling retains all original return refs
until all are ready, so a retried tracking RPC still has the same identity.

There is no atomic transaction spanning admission, Ray task submission, and
tracking. If a submitter disappears between RESERVED and TRACKED, the controller
cannot distinguish an unsubmitted task from one successfully submitted but not
reported. It retains that reservation. An administrator must establish that no
work can still run before releasing the ticket; a timeout alone is insufficient.
`status.permits` exposes RESERVED and TRACKED tickets for this diagnosis.

Waiting order and diagnostic counters are not durable. Automatic reconstruction
preserves resource accounting, but retried waiters can arrive in a different order.

## Transactional record storage

The implementation uses the etcd v3 JSON gateway. Each quota, permit, and client
session has its own key. A normal transition serializes and writes only that
record, O(B) in the record payload size, independent of the active permit count.
Each record carries a revision. Controller mutations are serialized by an async
lock and committed by a bounded writer (32 queued batches, at most 32 operations
per transaction). Blocking HTTP calls run in a thread, outside the actor event
loop. Ready grants and completed releases are committed in batches of up to 32.
Memory is published and grant futures are resolved only after commit succeeds.

Each controller incarnation installs a unique generation token. Every transaction
compares that token atomically with its mutations; a superseded controller cannot
write. Retried approvals from cached permits also check the token. Startup reads
records in revision-pinned pages of 256 and verifies fencing before accepting
requests. A timeout or cancellation during a write is treated as an ambiguous
commit and stops admission until reconstruction.

## Client acknowledgment and compaction

Each driver/worker process creates a random session ID on installation and opens
it lazily on its first submission. Tickets contain that session and a monotonically
increasing sequence. Once submission and tracking finish, or release is confirmed
after a submission failure, the client advances its contiguous completion watermark.
Out-of-order completions are held locally until the gap closes. A failed acknowledgment
is resent cumulatively on the next completion and cannot release running work.

The controller persists the watermark before deleting RELEASED records below it.
Active permits remain charged even below the watermark; once they finish, their
records can be deleted directly. Delayed acquire RPCs at or below the durable
watermark are rejected even after restart. Cleanup runs in bounded transactions
and resumes after a crash. Legacy manually supplied UUID tickets have no session
watermark and therefore retain their tombstones.

Session watermarks are retained (one small record per installed process), including
after a client dies. They cannot be expired safely while unlimited retries remain
possible. A dead client or a submission gap can retain unacknowledged tombstones;
this implementation does not infer acknowledgment from elapsed time. RESERVED
permits without execution identities still require explicit reconciliation.

## Recovery and deployment boundaries

Actor completion checks use one batched GCS actor-table read per polling pass.
This removes one RPC per tracked actor, but the table read itself scales with the
cluster actor count. Task completion uses saved ObjectRefs. Controller recovery
scales with the number of retained records and does not depend on replaying a log.

Set RAY_ADMISSION_ETCD_ENDPOINT and RAY_ADMISSION_ETCD_PREFIX before starting the
queue, or pass --etcd-endpoint and --etcd-prefix after `queue start`. Use one stable,
exclusive prefix per Ray cluster lifetime, reachable from every possible controller
node. HTTPS verifies certificates; --etcd-ca-file supports a mounted custom CA.
This client currently supports gateway endpoints without etcd user/password or
client-certificate authentication; provision access accordingly. The queue admin
token controls queue RPCs and is not an etcd credential.

This is a storage-format change from the experimental GCS snapshot implementation.
No live migration is provided. Drain the old service, export its quota configuration,
and initialize the new backend in a fresh prefix before enabling new submissions.
Do not switch storage backends with outstanding permits. etcd survives Ray teardown,
but persisted ObjectRefs are only valid for their originating Ray cluster; use a
new prefix and reapply quotas for a replacement cluster.

## Local benchmark

`benchmarks/storage.py` measures 100 updates of one record per active-set size
against an isolated, single-node local etcd. On this development machine:

| Active records | JSON transaction bytes/update | p50 (ms) | p95 (ms) | Recovery (ms) |
| --- | --- | --- | --- | --- |
| 10 | 442 | 11.782 | 18.904 | 10.283 |
| 100 | 442 | 12.150 | 20.053 | 14.161 |
| 1000 | 442 | 9.974 | 16.583 | 22.916 |

Bytes exclude transport headers. These are backend microbenchmarks, not end-to-end
Ray throughput or an HA etcd capacity claim. Each normal submission also incurs
tracking and acknowledgment transactions. Tests additionally verify that 96 queued
grants use three transactions, fencing rejects stale writers, and watermark cleanup
does not resurrect tickets or delete active permits.
