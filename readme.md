# PocketBase Calculated Fields Plugin

This plugin for [PocketBase](https://pocketbase.io) introduces a dynamic **calculated fields** system on a collection "computed_fields",each record in a dedicated collection represents a formula (cell) whose value is automatically computed based on dependencies from other records.

---
## 🚀 Quick Start

This is the simplest procedure to try the plugin immediately.

### 1️⃣ Start PocketBase 

In your terminal, inside the project root:

```bash
go run .
```

PocketBase will start with the calculated fields hook already active.

---

### 2️⃣ Log in as superuser

Open in your browser:

```
http://127.0.0.1:8090/_/
```

Use the default credentials:

- **Email:** `admin@admin.com`  
- **Password:** `adminadmin`

---

### 3️⃣ Create your first calculated field

1. Go to the **calculated_fields** collection  
2. Click **Create Record**  
3. In the **formula** field, enter:

```
2 + 1
```

4. Save the record.

The **value** field will automatically become `3`.

---

### 4️⃣ Try more complex formulas

You can use any expression supported by **expr-lang**, for example:

```
(10 / 2) + 4
abs(-3) + pow(2, 3)
```

---

### 5️⃣ Reference other calculated fields (Excel-style)

Every `calculated_fields` record can use the **ID of another record** as a variable.

Example:

1. Create a first record with the formula:
   ```
   5 * 2
   ```
   (suppose its ID is `A1xyz0123456789`)
2. Create a second record with the formula:
   ```
   A1xyz0123456789 + 3
   ```

The second record will automatically read the value of the first one — just like an Excel cell.

When the record `A1xyz0123456789` changes, the dependent record will also be recalculated.

---

This is enough to start experimenting with the calculation engine and dependency propagation.


## 🧩 Overview

The plugin adds hooks to a specific collection, typically called `calculated_fields`. Each record in this collection acts like a cell in a spreadsheet:
- It contains a **formula** that may reference other records (by ID).
- The **value** is automatically computed based on the formula.
- An optional **update_target** can be specified to update a field in another collection when this field is recalculated.

---

## 📦 Features

- ⚙️ Auto-calculation on create/update/delete of records.
- 🔁 Dependency graph traversal (BFS) to propagate changes.
- 🔒 Validation and error handling for missing or invalid references.
- 🧠 Caching of computed values for optimization.
- 📤 Optional propagation of changes to external collections via `update_target`.

---

## 📂 Data Model

Collection: `calculated_fields`

| Field           | Type     | Description |
|----------------|----------|-------------|
| `formula`       | text     | The formula expression, using record IDs as variable names. |
| `value`         | json     | The computed result. |
| `error`         | text     | Error message, if evaluation fails. |
| `depends_on`    | relation | References to other `calculated_fields` records this field depends on. |
| `update_target` | text     | Optional `collection.id.field` to be updated with the computed value. |

---

## 🧪 Formula Syntax

Formulas are compiled and executed using [expr-lang](https://github.com/expr-lang/expr).

Examples:
```go
"abc123 + def456"
"if(ghi789 > 10, ghi789 * 2, 0)"
```

References must match valid record IDs in the `calculated_fields` collection.

For example abc123 should be the id of a record in the calculated_fields collection. During formula calculation the id will be replaced by the corresponding value in "value" field. 

---

## ⚙️ Execution Flow

```text
🟦 OnCalculatedFieldsCreate / Update
 │
 ├─ Start transaction 
 │     │
 │     ├─ check if formula or value have changed to continue
 │     │
 │     ├─ call ResolveDepsAndTxSave(txApp, e.Record) 
 │     │       ├─ checks formula identifiers and updates "depends_on" field 
 │     │       ├─ check self-refereces to avoid loops
 │     │       ├─ prepares the env with values for formula eval
 │     │       
 │     └─ call evaluateFormulaGraph(txApp, e.Record, env)
 │          │
 │          ├─ Evaluate formula of root node
 │          ├─ BFS over children via calculated_fields_via_depends_on
 │          └─ For each:
 │               ├─ expand depends_on
 │               ├─ update env
 │               ├─ evaluate
 │               └─ applyResultAndSave() if dirty
 │                   ├─ if update_target field has a valid value, updates the foreign field

```

---

## 🔁 `update_target`: Forcing External Record Updates

The optional `update_target` field allows a record in the `calculated_fields` collection to **force the update of another record**, even if it’s not directly related.

This is useful when you want to trigger downstream updates in other collections.

### 📘 Practical Example

Suppose you have a collection called `Cells`, and you want to attach a computed field (`fx`) to it.

Steps:

1. Add a relation field called `fx` to the `Cells` collection, pointing to the `calculated_fields` collection.
2. In the related formula record, set the `update_target` to something like: cells.RECORD_ID.fieldName
3. This forces PocketBase to write the current `types.NowDateTime()` to the specified field (e.g. a `last_updated` field in the `Cells` record), triggering any update hooks or refresh logic.

---

## 🧯 Error Codes

| Code   | Meaning                              |
|--------|--------------------------------------|
| `1002` | Self-reference in formula            |
| `1003` | Circular dependency detected         |
| `1004` | Syntax error in formula              |
| `1005` | Referenced record not found          |
| `1006` | Runtime error during evaluation      |
| `1007` | Variable not found in DAG traversal  |
| `1008` | `update_target` misconfigured         |

---

TODO
- DESIGN ACCESS API RULES AND TEST