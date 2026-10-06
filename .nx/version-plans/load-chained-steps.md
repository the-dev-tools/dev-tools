---
cli: patch
---

Load runs now work for flows whose steps use an earlier step's response, such as a login token passed to the next request. Lean mode used to drop every response body, so those steps failed on every iteration and the run reported the target as unreachable. Bodies a later step reads are now kept; the rest are still dropped to keep memory flat.
