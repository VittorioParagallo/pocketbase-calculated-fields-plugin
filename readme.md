# PocketBase Calculated Fields Plugin

This plugin adds **server-side calculated fields** to PocketBase collections.

A calculated field is stored as a record in the `calculated_fields` collection and is always attached to a real **owner record** (for example: `booking_queue`, or any other collection). Formulas are automatically evaluated, dependency graphs are built, and updates propagate transactionally across dependent calculated fields — similar to spreadsheet behavior, but fully integrated with PocketBase collections, permissions and hooks.

> **Important**: users of your app should not “manage” the `calculated_fields` collection directly.
> In normal usage it behaves like an implementation detail behind your owner collections.

---

## ✨ Key Concepts

A calculated field is defined by:

- a **formula**
- an **owner collection**
- an **owner record**
- an **owner field**

The owner field is a **single-select relation** from the owner collection to `calculated_fields` (ex: `min_fx`, `max_fx`, `act_fx`, etc.).

---

## 📦 Features

- ⚙️ Automatic evaluation on create / update / delete
- 🔁 Dependency graph resolution (DAG + BFS propagation)
- 🛑 Self-reference and circular dependency detection
- ❗ Spreadsheet-like error handling (`#REF!`, `#DIV/0!`, `#VALUE!`, etc.)
- 🔐 Permission-aware: update allowed only if owner record is writable
- 🧹 Cascade delete when owner record is deleted
- ⏱ Touches `owner.updated` only when value actually changes
- 🧪 Full test suite with isolated test database
- 💯 Transactional: all recalculations happen inside one DB transaction

---

## 📂 Data Model

Collection: **`calculated_fields`**

| Field | Type | Description |
|------|------|-------------|
| `formula` | text | Expression evaluated with expr-lang |
| `value` | json | Computed value (JSON-encoded) |
| `error` | text | Error message if evaluation fails |
| `depends_on` | relation (self) | Referenced calculated_fields |
| `owner_collection` | text | Collection name of the owner |
| `owner_row` | text | Record ID of the owner |
| `owner_field` | text | Field name in the owner record |

Each calculated field belongs to exactly **one owner record** (enforced by the plugin; the owner triplet is immutable once set).

---

## 🚀 Quick Start

### 1️⃣ Install / wire the plugin in your PocketBase app

Import the package and bind the hooks at startup:

```go
// example main.go
import (
  "github.com/pocketbase/pocketbase"
  "github.com/VittorioParagallo/pocketbase-calculated-fields-plugin"
)

func main() {
  app := pocketbase.New()

  // binds all guards + create/update/delete hooks
  if err := calculatedfields.BindCalculatedFieldsHooks(app); err != nil {
    panic(err)
  }

  // ...start your app
}
```

or 



---

## 🚀 Installation (pbx / Go plugin integration)

This repository is a Go module that you import into your PocketBase project.

### 1) Add dependency

```bash
go get github.com/VittorioParagallo/pocketbase-calculated-fields-plugin
go mod tidy
```

### 2) Register the plugin in your PocketBase `main.go`

Example layout (PocketBase “pbx-style” app: a custom `cmd/dev/main.go` or your own PB app entrypoint):

```go
import (
  "github.com/pocketbase/pocketbase"
  // ...
  calculatedfields "github.com/VittorioParagallo/pocketbase-calculated-fields-plugin"
)

func main() {
  app := pocketbase.New()

  calculatedfields.Register(app)

  // start PB
  app.Start()
}
```

### 2️⃣ The plugin creates/ensures the `calculated_fields` collection

You **do not** need to create `calculated_fields` from the Admin UI.

On startup, the plugin ensures that a **non-system** collection named `calculated_fields` exists with the required schema (fields + indexes).
This keeps installation simple and avoids manual schema import steps.

> If you already have a `calculated_fields` collection, the plugin will validate/ensure the required schema.

### 3️⃣ Add computed relations to any owner collection

In the PocketBase Admin UI (or via schema import) add a **relation field** in your owner collection pointing to `calculated_fields`.

Rules (enforced by `CalculatedFieldsOwnersSchemaGuards`):
- the relation must target `calculated_fields`
- it must be **single-select** (`maxSelect = 1`)

Example owner collection: `booking_queue`
- `min_fx` → relation to `calculated_fields` (single-select)
- `max_fx` → relation to `calculated_fields` (single-select)
- `act_fx` → relation to `calculated_fields` (single-select)

### 4️⃣ Create an owner record: calculated fields are created automatically

When you create a new owner record, the plugin scans the owner schema and for every relation field pointing to `calculated_fields`:

- if the relation field is **empty**, it automatically creates a `calculated_fields` record:
  - `formula = "0"`
  - `owner_collection = <owner collection name>`
  - `owner_row = <owner record id>`
  - `owner_field = <relation field name>`
- then it links the new `calculated_fields` record back into the owner relation field

This happens **inside the same DB transaction**.

#### Anti-hijack behavior

If a client tries to **pre-fill** the relation field with an existing `calculated_fields` record id, the plugin verifies that:

- the referenced record exists
- and it belongs to the **same owner record** and **same owner field** (`owner_collection/owner_row/owner_field` must match)

Otherwise the request is rejected (hijack attempt).

---

## 🧠 Editing a formula

Normally you expose formula editing from your own UI (editing the owner record), or you allow editing `calculated_fields` from Admin UI for debugging.

When a `calculated_fields` record formula changes:
- dependencies are resolved (`depends_on` updated)
- the graph is evaluated transactionally
- dependents are recalculated (BFS)
- owners get their `updated` touched **only if** `(value, error)` changes

---

### About the `id`
- Use PocketBase defaults (15 chars, starts with a letter).
- The plugin **rejects client-provided `id`** on create/update requests for `calculated_fields` (server-generated IDs only).
  (Internal server code may still seed explicit IDs if you bypass hooks.)
------

## 🧪 Formula Syntax

Formulas are executed using [expr-lang](https://github.com/expr-lang/expr).

You can reference other calculated fields by **ID**:

```text
someCalculatedFieldId + 1
```

You can use functions (depending on your expr env setup):

```text
sum([A, B, 3])
max(X, Y)
if(A > 10) { 1 } else { 0 }
len(my_array)
```

---

## 🔗 Dependency Resolution

When a formula is created or updated:

1. identifiers are parsed from the formula
2. dependencies are extracted and saved to `depends_on`
3. self-reference is rejected (`1002`)
4. cycles are detected (`1003`)
5. evaluation starts from the changed node
6. propagation continues to dependent nodes (BFS)

Only nodes whose `(value, error)` actually changed are persisted (dirty-check optimization).

---

## ⚙️ Execution Flow (Simplified)

```text
Create/Update calculated_field
 │
 ├─ Transaction starts
 │
 ├─ Validate owner + immutability of owner triplet
 ├─ Extract identifiers from new formula
 ├─ Resolve deps and save depends_on
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
- otherwise: `app.CanAccessRecord(owner, updateRule)` must succeed
- additionally, formula evaluation is guarded so that referenced dependencies must be viewable (transitively), otherwise values are masked as `#AUTH!` on read/list

This makes calculated fields behave like **true computed properties** of the owner collection.

---

## 🗑 Cascade Delete

When an owner record is deleted:

- the plugin deletes all `calculated_fields` referenced by its computed relation fields
- the deletion triggers dependent updates:
  - references in formulas are rewritten to `#REF!`
  - errors propagate safely

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
| `1010` | Owner triplet is immutable |
| `1011` | Hijack / invalid prefilled reference |
| `1012` | Computed value cannot be serialized |

---

## 🧪 Testing

Tests live under `./tests` and use an isolated `pb_data` snapshot.

Run:

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
- be deterministic and transactional
- be safe in multi-collection environments
- remain generic and reusable

---

## 🧩 PBX / plugin builds

If you are using a PocketBase build system that bundles plugins (often referred to as “pbx” builds), the integration stays the same:
- add this module to your `go.mod`
- import it in your PocketBase `main.go`
- call `BindCalculatedFieldsHooks(app)` during bootstrap

Because the plugin **ensures the `calculated_fields` collection automatically**, you don’t need extra “install steps” beyond compiling your PocketBase binary with the plugin included.

---

## 📌 TODO

- Document the full list of supported functions (expr env)
- Provide example schemas (owner collections)
- Performance benchmarks
- Optional UI helper for formula editing
