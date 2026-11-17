# 0.1U: Anchor infrastructure options

No class below is a fix for P3B. PR #390 Scenario A needs an anchor that the BINTRANS restore did not reach. Scenario B remains possible against every class if that class is rolled back too. Cost figures are `NOT_VERIFIED`. No price in this repository was treated as current.

The protocol that any class must be able to serve is contract v1 in `RECOVERY_ANCHOR_PROTOCOL.md` (PR #390): synchronous confirmation of one record before the issuer opens a PostgreSQL transaction; exact successor only; no lower; pin of `(namespace, incarnation, seq, digest)`; fail closed when the anchor cannot be read.

## Comparison

| Question | A. Existing BINTRANS resources | B. Independent managed anchor | C. Dedicated anchor infrastructure | D. Quorum store |
| --- | --- | --- | --- | --- |
| What it would use | S1, S2, the E2E design host, `cr.selcloud.ru`, or a Selectel bucket in the current operating set. | An append-only log run by an administrator who is not a BINTRANS operator, with its own storage and backup. No product is chosen. | A service whose host, disk, backup, and root are outside the BINTRANS Selectel operating set. A new VM in the same account is not this class. | Several members, each in its own failure domain, with a documented majority and membership-change rule. |
| Scenario A (BINTRANS stores roll back, anchor does not) | `NOT_SUITABLE`. Nothing in the inventory is outside the restore story of the same operators. | `TECHNICALLY_FEASIBLE` only after the provider shows that a BINTRANS restore cannot move the log. `REQUIRES_PROVIDER_VERIFICATION`. | `TECHNICALLY_FEASIBLE` on the same condition. `REQUIRES_PROVIDER_VERIFICATION`. | `NOT_SUITABLE` as a next step. There are not three domains to place members in. |
| Scenario B (anchor rolls back too) | Expected, not exceptional. | Still open if the managed operator can rewrite history, or if BINTRANS holds the admin credential. The class does not close B. | Still open if the dedicated host's admin or snapshot system is the BINTRANS one. | Raises the number of domains that must be rolled back together. Does not remove B. |
| Shared admin with BINTRANS | Yes, on the evidence. S2 is `root` under the staging key. | Must be no. Not shown for any named service. | Must be no. A third server ordered in the same panel is still shared admin. | Shared if the members share one panel. |
| Shared failure domain | Yes until proven otherwise. Same provider family is documented for S1, S2, and the registry. | Intended to be separate. `NOT_VERIFIED`. | Intended to be separate. `NOT_VERIFIED`. | Separate only if the members are. They are not available. |
| Separate credentials | Not shown. Key names differ. Custody does not. | Required. `NOT_VERIFIED`. | Required. `NOT_VERIFIED`. | Required per member. `NOT_VERIFIED`. |
| Synchronous digest and sequence ACK | No API of that shape is documented on S1, S2, the registry, or an unconfigured bucket. | The class can be built to do it. No provider here has been shown to do it. | Same. The protocol would be ours to implement. Durability before ACK is `NOT_PROVEN` until tested. | Same, plus a quorum round before ACK. Latency `NOT_VERIFIED`. |
| Loss of connectivity | Staging does not fail closed an issuer, because there is no issuer gate in production. | Must refuse issuance. Design only. | Must refuse issuance. Design only. | Minority must not ACK. Design only. |
| Recovery of a missing segment | A disk snapshot restores whatever was on the disk, including a lowered position. | Possible if the service keeps every pin and BINTRANS cannot delete them. `NOT_VERIFIED`. | Possible if the host keeps the log and its backup cannot lower it alone. `NOT_VERIFIED`. | Possible if a majority still has the pins. No such majority exists. |
| Operating cost | No new invoice is visible. The cost is an undetected P3B. | `NOT_VERIFIED`. | `NOT_VERIFIED`. | `NOT_VERIFIED`, and higher by construction. |
| Verdict | `NOT_SUITABLE` | `REQUIRES_PROVIDER_VERIFICATION` | `REQUIRES_PROVIDER_VERIFICATION` | `NOT_SUITABLE` now |

## Mandatory requirements against each class

| Requirement | A | B | C | D |
| --- | --- | --- | --- | --- |
| Anchor ACK before PostgreSQL `BEGIN` | Absent | Not shown | Not shown | Not shown |
| Monotonic append | Absent | Not shown | Not shown | Not shown |
| No lowering of a confirmed position | Absent. Snapshots lower disks. | Must be a provider property | Must be an administrative property | Must survive membership change |
| Confirm `(namespace, incarnation, seq, digest)` | Absent | Not shown | Not shown | Not shown |
| Replay protection (fresh challenge) | Absent | Not shown | Not shown | Not shown |
| Refuse when unavailable | Absent | Design rule only | Design rule only | Design rule only |
| Independent recovery domain | `FAIL` on current inventory | `NOT_VERIFIED` | `NOT_VERIFIED` | `FAIL` until three domains exist |
| Separate administrators | `FAIL` | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED` |
| Verifiable history after restore | `FAIL` for a joint snapshot | `NOT_VERIFIED` | `NOT_VERIFIED` | `NOT_VERIFIED` |
| Gate stays closed after restore | Design rule only. No service enforces it. | Same | Same | Same |

A pass in the right-hand columns means "the class is allowed to be examined." It does not mean a product was selected or that P3B is closed.
