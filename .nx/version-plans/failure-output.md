---
cli: patch
---

A failing assertion now prints one line that shows what the server answered: `Error: assertion failed: response.status == 200 (got 401: {"error":"Invalid credentials"})`. Non-2xx responses include a short body excerpt; 2xx bodies are left out because they can carry tokens. The flag reference no longer prints under a failing flow, and the error no longer repeats four times.
