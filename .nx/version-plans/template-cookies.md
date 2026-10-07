---
cli: minor
---

Flows can read the cookies they've received: `{{ cookies.csrftoken }}`, or `{{ cookies["XSRF-TOKEN"] }}` for names that aren't identifiers. This makes double-submit CSRF protection (Django, Laravel, Angular, many Express apps) testable: copy the CSRF cookie into the header the server checks. Values are URL-decoded the way client code reads them, the latest value of each cookie wins, and a flow variable named `cookies` takes precedence.
