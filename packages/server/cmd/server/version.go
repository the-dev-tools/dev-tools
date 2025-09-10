package main

// Version is injected at build time using -ldflags "-X main.Version=...".
// Defaults to "dev" when running locally.
var Version = "dev"

