---
desktop: minor
---

Studio now works with Stresseur Cloud, records your app's API calls and has a launchpad for first runs.

- Sign in to Stresseur from the app. Signed in, a Cloud section lists your repos and their flows.
- Run a flow on your machine or in Stresseur Cloud from the Run button. Send edits back to the repo with Propose change.
- Record your app's API calls into flows with the bundled `stresseur` binary. Save them to Files, a cloud repo, or both.
- First launch opens a launchpad: pick your repo, record, hand off to your coding agent, add a CI check and open a pull request. No app at hand? Try it on the sample shop.
- The flow editor agent can use your included AI credits when you're signed in.
- New environment picker with a peek at its variables. Secret values are masked.
- Sidebar sections collapse.
- Importing YAML or HAR keeps AI checks (`expect:`, `judge:`, `quality:`, `iterations:`) and streaming settings (`stream:`, `stream_timeout_ms`), and exporting writes them back. HAR entries with event-stream responses get a `stream:` preset. The app doesn't show or run these yet.
