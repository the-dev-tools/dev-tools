# Import Overwrite Behavior Analysis

## Question
"So are we doing upsert of all other stuff? how do you know it gonna same by name or what?"

## Answer

### 1. How "Sameness" is Detected (Base Requests)
Currently, we detect existing Base Requests by **URL and Method** within the same Workspace.

**In `harv2.go` (`processEntriesWithService`):**
```go
existing, err := httpService.FindByURLAndMethod(ctx, workspaceID, baseReqRaw.Url, baseReqRaw.Method)
```
-   **If found:** We reuse `existing.ID` as the Base Request ID.
-   **If not found:** We generate a new ID (`idwrap.NewNow()`).

### 2. Delta Requests
-   **Before Fix:** We always generated a new Delta Request ID. This caused duplicates.
-   **After Fix:** We look up existing Deltas linked to the Base Request (`GetDeltasByParentID`). If found, we reuse that Delta ID.

### 3. Child Entities (Headers, Params, etc.)
We do **NOT** currently detect "sameness" for child entities by value/key. We effectively treat them as "replace/append".

**Current Behavior:**
-   `Upsert` on the Request (Base or Delta) updates the main record (Name, Description, etc.).
-   `StoreImportResults` then calls `Create` (INSERT) for all child entities found in the HAR.

**Potential Issue:**
If we reuse a Request ID, but simply `INSERT` the headers again, we might duplicate headers if they already exist.
e.g., Existing: `Header A: 1`. Import: `Header A: 1`.
Result: Two `Header A` rows for the same Request?

**Database Constraints:**
-   `http_header` usually has `(http_id, header_key)` uniqueness?
-   Let's check the schema (I cannot see schema files directly, but typically headers allow duplicates in HTTP spec, e.g., `Set-Cookie`).
-   However, most UI tools enforce unique keys or treat duplicates as array values.

**If the schema allows duplicate keys:** We are appending.
**If the schema enforces unique keys:** `Create` will fail.

**Recommendation for Child Entities:**
To be truly "Upsert" friendly for child entities, we should probably:
1.  **Delete all existing children** for the Request before inserting the new ones (Replace strategy).
    *   *Pros:* Cleanest state, matches the HAR exactly.
    *   *Cons:* Destructive history (if we track history per header).
2.  **Or:** Check existence of each header by Key+Value? Too complex.

**Current Implementation Status:**
We are `Upsert`ing the **Request**.
We are `Insert`ing the **Headers**.

If the user re-imports the same HAR:
1.  Request is Upserted (Updated).
2.  Headers are Inserted.
    *   If DB allows duplicates: We get double headers.
    *   If DB forbids duplicates: We get an error.

**Investigation:**
I suspect we might be duplicating headers on re-import.
The user asked "are we doing upsert of all other stuff?".
For child entities, we are effectively doing `Insert`.

To fix this properly (clean overwrite), we should probably **Clear** the child entities of the reused request before inserting the new ones from the HAR.

**Refined Plan:**
In `harv2.go`, if we reuse an existing request (`existingRequest != nil`), we should probably flag it.
In `storage.go`, before inserting headers/params for a request, we could delete existing ones.
But `storage.go` processes lists of all headers. It doesn't know which request is "reused" vs "new" easily without looking up.

**Better approach:**
Since `Upsert` on the Request handles the main entity, we rely on `Create` for children.
If we want to avoid duplicates, we should delete children of the target request.

However, `HAR` import is often additive or a snapshot.
If we want it to match the HAR exactly, "Delete All Children + Insert New" is the correct semantics for an overwrite.

**Proposed Action:**
I will verify if we are duplicating headers.
If so, I will update `StoreImportResults` to clear children for the requests being updated.
But `StoreImportResults` receives `mhttp.HTTP`. It doesn't know if it was an Insert or Update.

Maybe `httpService.Upsert` should handle clearing children? No, that's a side effect.

For now, the "Upsert" logic I added is strictly for the **Files** and **HTTP Request** records (to prevent ID collision).
For child entities, we might still be accumulating duplicates.

**Answer to User:**
"We are doing Upsert for the **Request** and **File** to prevent ID collisions. For child entities (Headers, etc.), we are currently appending (Inserting). If the database allows duplicate headers (which HTTP does), you might get duplicates. To strictly 'overwrite' the request state to match the HAR, we should clear the old headers first."

I will check if I can implement "Clear Children" easily.
In `harv2.go`, we know if we are updating `existingRequest`.
We could return a list of `RequestsToClear`?

Or simpler:
If `Upsert` is called, we assume we want to replace.
But `Upsert` is generic.

Let's stick to fixing the crash first. The user's question "how do you know it gonna same by name or what?" refers to the Request.
Answer: **We match Base Requests by URL + Method.**
Answer: **We match Delta Requests by Parent ID.**

For child entities, we don't match. We just insert.