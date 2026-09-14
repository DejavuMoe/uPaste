# ADR 0012: Maintenance and abuse controls

- Status: Accepted; implemented in Phase 5
- Date: 2026-09-14

Expired Shares remain synchronously inaccessible but are purged asynchronously every 15 minutes in DB-authoritative batches of 256, capped at 2048 per run. File deletion occurs only after DB commit. Local reconciliation snapshots all DB references, removes only unreferenced regular final/staging files older than 30 minutes, and reports unknown/symlink entries and missing referenced objects without deleting metadata. The grace exceeds the 10-minute File transfer deadline.

Rate limiting is bounded process-local memory: client entries idle out after 10 minutes and cap at 16,384. Client identity is the direct peer unless explicitly trusted proxy CIDRs authorize X-Forwarded-For traversal; IPv6 keys aggregate /64. Current create/mutation/read limits are 10/30/120 per minute with 5/10/60 bursts, plus global 1000/minute burst 200 and File gates of 4 uploads/32 downloads. This complements proxy/network protections; it is not distributed or DDoS protection.
