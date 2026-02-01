# PocketBase Calculated Fields Plugin

A **PocketBase plugin** that adds **server-side calculated fields** with **spreadsheet-like dependency propagation**.

A calculated field is stored as a record in the `calculated_fields` collection and is attached to a real **owner record**
(e.g. `booking_queue`, but it works with any collection).

When a formula changes, the plugin:
- parses referenced calculated field IDs,
- builds/validates the dependency graph (DAG),
- evaluates affected nodes,
- saves only nodes that actually changed,
- updates (`touches`) the owner record `updated` timestamp only when needed,
- performs everything **transactionally**.

---

## ✨ Key Concepts

A calculated field is defined by:

- **formula**: an expression evaluated with `expr-lang`
- **owner_collection**: name of the owner collection
- **owner_row**: ID of the owner record
- **owner_field**: the owner field logically “holding” the computed value
- **depends_on**: relation to other `calculated_fields` referenced by the formula (derived automatically)

> Think of `calculated_fields` as a backend-only storage for computed values.
> Your app mainly interacts with the **owner collection** fields.

---

## 📦 Features

- ⚙️ Automatic evaluation on create / update / delete
- 🔁 Dependency resolution (DAG + BFS propagation)
- 🛑 Self-reference and circular dependency detection
- ❗ Spreadsheet-like errors (`#REF!`, `#DIV/0!`, `#VALUE!`, …)
- 🔐 Permission-aware: updates allowed only if owner record is writable
- 🧹 Cascade delete when owner record is deleted
- ⏱ Touches `owner.updated` only when `(value, error)` changes
- 💯 Transactional recalculation (single DB transaction)
- 🧪 Full test suite with isolated test database snapshot

---

## 📂 Data Model

Collection: **`calculated_fields`** (non-system)

| Field | Type | Required | Description |
|------|------|----------|-------------|
| `formula` | text | ✅ | Expression to evaluate |
| `value` | json | ❌ | Computed value |
| `error` | text | ❌ | Human-readable error message |
| `depends_on` | relation → `calculated_fields` | ❌ | Dependencies (auto-managed) |
| `owner_collection` | text | ✅ | Owner collection name |
| `owner_row` | text | ✅ | Owner record ID |
| `owner_field` | text | ✅ | Owner field name |

### About the `id`
- Use PocketBase defaults (15 chars, starts with a letter).
- The plugin **rejects client-provided `id`** on create/update requests for `calculated_fields` (server-generated IDs only).
  (Internal server code may still seed explicit IDs if you bypass hooks.)

---

## 🚀 Installation (pbx / Go plugin integration)

This repository is a Go module that you import into your PocketBase project.

### 1) Add dependency

```bash
go get <MODULE_PATH>
go mod tidy
```

### 2) Register the plugin in your PocketBase `main.go`

Example layout (PocketBase “pbx-style” app: a custom `cmd/dev/main.go` or your own PB app entrypoint):

```go
import (
  "github.com/pocketbase/pocketbase"
  // ...
  calculatedfields "<MODULE_PATH>"
)

func main() {
  app := pocketbase.New()

  calculatedfields.Register(app)

  // start PB
  app.Start()
}
```

> Replace `<MODULE_PATH>` with the real module path once published.

---

## 🧩 Setup: create the `calculated_fields` collection

Create a **non-system** collection named `calculated_fields` with the fields listed above.
You can do it from the Admin UI, or import a JSON schema.

### Recommended `id` settings
Keep the PB defaults, i.e.:
- Pattern: `^[a-z][a-z0-9_]*[a-z0-9]$`
- Autogenerate: `[a-z][a-z0-9_]{13}[a-z0-9]`

---

## 🔁 Automatic Owner Synchronization (Owner collections → `calculated_fields`)

If an owner collection contains a **relation field** pointing to `calculated_fields`
(e.g. `min_fx`, `max_fx`, `act_fx`), the plugin automatically manages the related
`calculated_fields` record lifecycle.

### On owner record create
- creates the corresponding `calculated_fields` record,
- links it to the owner record,
- initializes formula + metadata,
- does it in the same transaction.

### On owner record update
- protects against hijacking,
- keeps ownership metadata consistent,
- re-evaluates affected formulas.

This makes calculated fields behave like **true computed fields attached to the owner**, not like a standalone table.

---

## 🧪 Formula Syntax

Formulas are executed with [`expr-lang`](https://github.com/expr-lang/expr).

### Referencing other calculated fields
You reference dependencies by **calculated_field record ID**:

```text
abc123def456ghi + 1
```

### Functions (examples)

```text
sum([A, B, 3])
max(X, Y)
if(A > 10) { 1 } else { 0 }
len(my_array)
```

Supported patterns include:
- numeric operations
- arrays
- aggregate functions
- if blocks / ternary
- nested formulas

---

## 🔗 Dependency Resolution

When a formula is created or updated:

1. Identifiers are parsed from the formula
2. Dependencies are extracted and stored in `depends_on`
3. Self-reference is rejected
4. Cycles are detected (DAG validation)
5. Evaluation propagates only to impacted nodes

Only nodes whose `(value, error)` actually changed are persisted.

---

## ⚙️ Execution Flow (Simplified)

```text
Create/Update calculated_field
 │
 ├─ Transaction starts
 │
 ├─ Validate owner exists
 ├─ Check permission on owner updateRule
 ├─ Parse formula identifiers
 ├─ Update depends_on
 │
 └─ evaluateGraph():
        ├─ evaluate node
        ├─ if dirty → save
        ├─ touch owner.updated
        └─ BFS propagate to children
```

---

## 🔐 Security & Permissions

Updating a calculated field requires permission to update its owner record.

Rules:
- superuser always allowed
- otherwise: `app.CanAccessRecord(owner, updateRule)`
- prevents hijacking calculated fields across owners

---

## 🗑 Cascade Delete

When an owner record is deleted:
- its calculated_fields are deleted automatically,
- dependent formulas become `#REF!`,
- errors propagate safely.

---

## 🧯 Error Codes

| Code | Meaning |
|------|--------|
| `1002` | Self reference in formula |
| `1003` | Circular dependency |
| `1004` | Syntax error |
| `1005` | Referenced record not found |
| `1006` | Runtime evaluation error |
| `1007` | Missing variable during DAG walk |
| `1008` | Invalid owner reference |

---

## 🧪 Testing

The repository includes a test suite under `./tests`.

- Tests use an isolated `tests/pb_data` snapshot (no migrations required).
- Run:

```bash
go test ./... -v
```

---

## 🧭 Design Philosophy

This plugin is not a spreadsheet emulator.
It is a **reactive computation engine integrated into PocketBase’s data model**.

Goals:
- behave like a native field
- respect PocketBase rules and hooks
- deterministic, transactional, and safe
- generic and reusable across projects

---

## 📌 TODO

- Document all supported functions/operators
- Provide minimal example schemas for common use cases
- Add performance notes & benchmarks
- Optional UI helper for formula editing
