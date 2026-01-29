# PocketBase Calculated Fields Plugin

This plugin adds **server-side calculated fields** to PocketBase collections.

Each calculated field is a record in the `calculated_fields` collection and is always attached to a real **owner record** (for example: `booking_queue`, or any other collection).

Formulas are automatically evaluated, dependency graphs are built, and updates propagate transactionally across dependent calculated fields — similar to spreadsheet behavior, but fully integrated with PocketBase collections, permissions and hooks.

---

## ✨ Key Concepts

A calculated field is defined by:

- a **formula**
- an **owner collection**
- an **owner record**
- an **owner field**

Example:  
> “This calculated field computes `min_fx` for booking_queue `queue_a0001` using a formula that depends on other calculated fields.”

---

## 📦 Features

- ⚙️ Automatic evaluation on create / update / delete  
- 🔁 Dependency graph resolution (DAG, BFS propagation)  
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
| `value` | json | Computed value |
| `error` | text | Error message if evaluation fails |
| `depends_on` | relation (self) | Referenced calculated_fields |
| `owner_collection` | text | Collection name of the owner |
| `owner_row` | text | Record ID of the owner |
| `owner_field` | text | Field name in the owner record |

Each calculated field belongs to exactly **one owner record**.

---

## 🚀 Quick Start

### 1️⃣ Run PocketBase

```bash
go run .
```

---

### 2️⃣ 🧠 Automatic Owner Synchronization (Collections → calculated_fields)

One of the core features of this plugin is the automatic synchronization between any collection and the calculated_fields collection.

Whenever a record is created or updated in a collection that contains a relation field pointing to calculated_fields, the plugin automatically manages the lifecycle of the corresponding calculated field record.

This makes calculated_fields behave like a true computed field attached to another collection, rather than a standalone table.

🔹 Automatic creation of calculated_fields records

If a collection contains a relation field referencing calculated_fields (for example: min_fx, max_fx, act_fx, etc.):

When a new record is created in that collection:
	•	the plugin automatically creates a corresponding record in calculated_fields
	•	links it back to the owner record
	•	initializes its formula and metadata
	•	sets ownership information

This happens transparently inside the same database transaction.

Example:

Collection: booking_queue
Field: min_fx → relation to calculated_fields

When you create a new booking_queue record:
booking_queue
 └─ min_fx → calculated_fields record is automatically created

### 3️⃣ Edit a calculated field formula

Open the `calculated_fields` collection and update:

```text
formula = 2 + 3
```

The plugin automatically computes:

```text
value = 5
```

and touches the owner record `updated` field.

---

## 🧪 Formula Syntax

Formulas are executed using [expr-lang](https://github.com/expr-lang/expr).

You can reference other calculated fields by ID:

```text
booking_queue_queue_a0001_min + 1
```

You can use functions:

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
- ternary / if blocks  
- nested formulas  

---

## 🔗 Dependency Resolution

When a formula is created or updated:

1. Identifiers are parsed from the formula  
2. Dependencies are extracted (`depends_on`)  
3. Self-reference is rejected  
4. Cycles are detected  
5. DAG is built  
6. Evaluation propagates to dependent nodes  

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
- prevents hijacking calculated fields from foreign records  

This makes calculated fields behave like **true computed properties** of the owner collection.

---

## 🗑 Cascade Delete

When an owner record is deleted:

- all its calculated_fields are deleted automatically  
- dependent formulas are rewritten to `#REF!`  
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

---

## 🧪 Testing

The plugin includes a full test suite covering:

- formula evaluation  
- propagation  
- cycles  
- error handling  
- permissions  
- cascade delete  
- owner updated touch  
- dirty-check optimization  
- null / empty handling  

Tests use an isolated `test_pb_data` database snapshot (no migrations).

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

## 📌 TODO

- Documentation of supported functions  
- Example schemas  
- Performance benchmarks  
- UI helper for formula editing  
