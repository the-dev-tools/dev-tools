# Flow Engine & Node Specification

## Overview

The Flow system allows users to visually chain API requests and create complex test scenarios without writing code. It consists of a visual builder (Frontend) and an execution engine (Backend/CLI).

## Core Concepts

### 1. Nodes

Nodes are the building blocks of a flow. Each node represents a distinct action or logic step.

- **Request Node:** Executes an HTTP request. Can reference variables from previous steps.
- **Condition Node:** Implements `if/else` logic based on data (e.g., check if a response status is 200).
- **Loop Node:** Iterates over a dataset (e.g., a JSON array from a previous response) or a fixed range.
- **Data Node:** Imports or defines static data (e.g., CSV/Excel imports) to drive the flow.

### 2. Variable System

- **Flow Variables:** Variables scoped to the execution of the flow.
- **Environment Variables:** Global variables (e.g., `BASE_URL`, `API_KEY`) manageable via the Environment system.
- **Chaining:** Responses from Request Nodes can be extracted into variables (e.g., `{{Login.response.body.token}}`) and used in subsequent nodes.

## Architecture

### Execution Engine (`apps/cli/internal/runner`)

The CLI contains the headless execution engine used for CI/CD and running flows locally.

- It traverses the node graph.
- Handles variable substitution.
- Manages execution state (success/failure, retries).

### Server Logic (`packages/server/internal/api/rflowv2`)

- Manages the persistence of Flow definitions.
- Handles run orchestration.

## Data Model

- Flows are stored as directed graphs (or similar structures) in the database.
- Node configurations define their inputs, outputs, and connections to other nodes.
