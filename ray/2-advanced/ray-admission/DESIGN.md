# Queue lifecycle and GCS KV storage

The queue uses Ray's built-in GCS KV. No external database, endpoint, or storage
prefix needs to be configured. The fixed named controller is the only writer to
the `ray-submission-queue` KV namespace.

## Lifecycle

| State | Charge | Durable data |
| --- | --- | --- |
| WAITING | None | Ray retries the pending admission RPC |
| RESERVED | Declared resources | Ticket, quota namespace, job, resources |
| TRACKED | Declared resources | Reservation plus ObjectRefs or actor ID |
| RELEASED | None | Tombstone until covered by acknowledgment |

Reservations are persisted before approval. Tracking retries are idempotent.
Changing an execution object is rejected. Released or acknowledged tickets cannot
be acquired again. Memory, replies, and subsequent admission are published only
after storage completes.

## Per-record writes and redo recovery

Quota, permit, and session records are independent JSON keys. Updating one permit
does not serialize unrelated permits. A bounded background writer moves blocking
GCS calls off the actor event loop; batches contain at most 32 changed records.

GCS KV has no multi-key transaction or compare-and-set fencing. Each batch writes
its complete intent to `pending-v2`, applies idempotent record puts/deletes, then
deletes the journal before acknowledging success. On an error or cancellation,
admission stops. Reconstruction replays the journal before loading records.

These are single-writer recovery semantics, not distributed transactions. The
service depends on Ray terminating the old named actor before restart. Direct
external KV writes or simultaneous controllers are unsupported; record revisions
do not provide CAS protection. There is no independent fencing for stale writers.

K changes cost K+2 KV operations and O(sum of changed payload sizes) bytes. This
reduces serialization and payload size, not necessarily RPC count. Recovery lists
and reads all retained records, O(N). KV state follows GCS persistence rules.

## Acknowledgment and compaction

Each driver/worker lazily opens a random session; tickets use increasing sequence
numbers. After tracking completes, or failed submission cleanup succeeds, clients
advance a contiguous acknowledgment watermark. Out-of-order completions wait for
gaps. Failed acknowledgments are resent cumulatively on later completions.

Persist the watermark before deleting covered RELEASED records. Active permits
remain charged below the watermark and are deleted only after completion. Delayed
acquires below a saved watermark are rejected after restart even without tombstones.
Cleanup resumes after interruption. One small record remains per process session;
dead clients, acknowledgment gaps, and legacy UUID tickets can retain tombstones.
No TTL assumes that unlimited retries have stopped.

## Monitoring and remaining boundaries

Saved ObjectRefs monitor tasks; one batched GCS actor-table query monitors actors.
Transient failures retry with backoff and appear in status health. Missing actor
state retains reservations. A submitter lost between RESERVED and TRACKED leaves
a reservation requiring explicit reconciliation; time alone cannot prove work
was never submitted. Waiting order and counters are not recovered. ObjectRefs
cannot transfer to a replacement Ray cluster.

## Earlier snapshots

Before accepting requests, startup copies `controller-state-v1` into individual
records, preserving quota, permits and tombstones. Migration is replayable after
interruption and assumes the old named controller has stopped. The original
snapshot remains for inspection but becomes non-authoritative after the migration
marker is saved. No external database is involved.

## Verification

Tests use real local Ray GCS: partial batch replay, lost replies, cancelled writes,
active-work restart and acknowledgment cleanup. `benchmarks/storage.py` measures
bytes, GCS operations, latency and recovery at increasing active-set sizes. These
are backend microbenchmarks, not production throughput claims.

Local Ray 2.58 results (100 single-record updates per size):

| Retained records | Written key/value bytes per update | KV mutations | p50 / p95 (ms) | Load (ms) |
| --- | --- | --- | --- | --- |
| 10 | 207.88 | 3 | 0.479 / 0.589 | 1.890 |
| 100 | 207.88 | 3 | 0.443 / 0.495 | 15.151 |
| 1000 | 207.88 | 3 | 0.444 / 0.503 | 151.631 |

Bytes count put keys and values, excluding RPC framing and delete requests.
Reproduce with `PYTHONPATH=src python benchmarks/storage.py` in an environment
with Ray installed. Each run starts an isolated local cluster.
