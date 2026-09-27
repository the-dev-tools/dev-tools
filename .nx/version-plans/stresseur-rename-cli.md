---
cli: minor
---

DevTools CLI is now Stresseur CLI. The new stresseur command works exactly like devtools, and running the install script again adds it next to devtools. The devtools and devtoolscli commands keep working, with the same output and exit codes, but print a one-line note on stderr suggesting stresseur; set STRESSEUR_NO_RENAME_NOTICE=1 to hide it. STRESSEUR_MODE is accepted alongside DEVTOOLS_MODE. For Stresseur Studio users: If you ever go back to a 1.1.x version, it won't see your data, because it now lives in the Stresseur Studio folder; the old folder is kept when the move had to copy.
