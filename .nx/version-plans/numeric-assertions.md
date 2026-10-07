---
cli: patch
---

Assertions and conditions can now compare numbers from JSON responses: `response.body.qty == 3`, `response.body.price > 100` and arithmetic work. Response numbers used to compare as text, so equality was always false and `>`/`<` failed with a type error. Values copied into later requests with `{{ }}` keep their exact original text, as before.
