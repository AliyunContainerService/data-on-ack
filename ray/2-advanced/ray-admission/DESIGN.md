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
lost. On any snapshot write failure, the controller marks storage unhealthy,
rejects further mutations and admissions, and fails pending waiters. In-memory
usage must not be treated as authoritative in this condition. Restart the actor
with `ray.kill(controller, no_restart=False)` after fixing storage availability;
the constructor reads the committed snapshot before admitting work again.

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

## Storage cost and proposed replacement

The current implementation still writes a complete GCS KV snapshot for each
resource transition. With N active permits, R retained released tickets, and
payload size B, each transition costs O((N + R) * B) serialization and network
traffic. It also blocks the controller event loop. Retaining tombstones fixes
retry correctness but increases this cost; they must not be silently discarded
while unlimited RPC retries can still arrive. This implementation is not sized
for a long-running, high-throughput admission service.

Replace the snapshot backend with per-ticket records and per-quota records:

1. Store one record per ticket with state, execution identity, and a revision.
   Each lifecycle transition updates O(B) data. Reconstruct usage by scanning
   RESERVED and TRACKED records; validate all records before enabling admission.
2. Use transactional compare-and-set and a controller generation/fencing token
   in a transactional backend such as etcd. A superseded controller must not be
   able to commit state. GCS KV get/put alone is insufficient for that protocol.
3. Serialize transitions through a bounded writer. Group independent writes into
   transactions and acknowledge each grant only after its transaction commits.
   Do not buffer uncommitted grants and return success early.
4. Give clients durable session IDs and monotonically increasing ticket sequence
   numbers. Compact RELEASED records only below a durable client acknowledgment
   watermark or after an explicitly closed/fenced session. A fixed TTL is unsafe
   with the current unlimited retry configuration.
5. Batch actor-state reads and use completion notifications where supported.
   Monitor query failures must retain reservations and expose degraded health.

Before switching backends, benchmark active permit counts, submission rates,
GCS/transaction latency, snapshot bytes, event-loop delay, recovery time, and
tombstone growth. Fault tests must cover commit-response loss, controller fencing,
duplicate/reordered RPCs, and a submitter dying at every lifecycle boundary.

This backend replacement is a proposed follow-up; this change retains the
snapshot backend and prioritizes fail-stop correctness over throughput.
