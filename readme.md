# PocketBase Calculated Fields Plugin

This plugin adds Excel-style **calculated fields** to PocketBase.  
Each record in the `calculated_fields` collection behaves like a reactive “cell” whose value is automatically computed based on dependencies from other records.

Whenever a field changes, the plugin recalculates the entire dependency graph, propagates results (and errors), and optionally triggers updates in external collections.

---

## 🚀 Quick Start

The fastest way to try the plugin.

### 1️⃣ Start PocketBase

From the project root:

```bash
go run .
```

PocketBase will start with the calculated fields hooks enabled.

---

### 2️⃣ Log in as superuser

Open:

```
http://127.0.0.1:8090/_/
```

Credentials:

- **Email:** `admin@admin.com`  
- **Password:** `adminadmin`

---

### 3️⃣ Create your first calculated field

1. Open the **calculated_fields** collection  
2. Click **Create Record**  
3. Set the **formula** field to:

```
2 + 1
```

4. Save.

The **value** field will automatically become:

```
3
```

---

### 4️⃣ Try more complex formulas

Any expression supported by **expr-lang** works:

```
(10 / 2) + 4
abs(-3) + pow(2, 3)
```

---

### 5️⃣ Reference other calculated fields (Excel-style)

You can reference the **ID of another record** as if it were a variable.

Example:

1. First record:
   ```
   5 * 2
   ```
   (suppose its ID is `A1xyz0123456789`)
2. Second record:
   ```
   A1xyz0123456789 + 3
   ```

The second record will automatically read the value of the first one.  
If the first record changes, the second one is recalculated too.

---

## 🧩 Overview

This plugin turns PocketBase into a simple reactive computation engine:

- Each record has a **formula**
- Dependencies are detected by scanning the formula for record IDs
- Values are evaluated server-side
- All dependent nodes are updated via BFS
- Errors propagate in a spreadsheet-like manner
- Optional `update_target` allows external collections to react to recalculations

---

## 📦 Features

- ⚙️ Automatic computation on create/update/delete  
- 🔁 Dependency graph traversal (BFS)  
- 🛑 Circular dependency and self-reference detection  
- ❗ Spreadsheet-like error codes (#REF!, #DIV/0!, #VALUE!, etc.)  
- 📡 Optional external update trigger via `update_target`  
- 🔐 Fully transactional: every recalculation happens inside a DB transaction  

---

## 📂 Data Model

Collection: **`calculated_fields`**

| Field           | Type     | Description |
|----------------|----------|-------------|
| `formula`       | text     | Expression using record IDs as variables. |
| `value`         | json     | Computed value. |
| `error`         | text     | Error message, if evaluation fails. |
| `depends_on`    | relation | Automatic list of referenced record IDs. |
| `update_target` | text     | Optional `collection.id.field` to touch when recalculated. |

---

## 🧪 Formula Syntax

Formulas are executed using [expr-lang](https://github.com/expr-lang/expr).

Examples:

```
A1xyz01234 + 10
if(B2def > 3, B2def * 5, 0)
pow(XYZ, 3) + abs(-7)
```

Record IDs used in the formula must correspond to valid records in `calculated_fields`.  
During evaluation, each ID is replaced with its stored `value`.

---

## ⚙️ Execution Flow (Simplified)

```
Create/Update event
 │
 ├─ Transaction starts
 │
 ├─ e.Next() persists the record (inside the transaction)
 │
 ├─ Skip if formula/value unchanged
 │
 ├─ ResolveDepsAndTxSave
 │      ├─ Parse identifiers from formula
 │      ├─ Validate references
 │      ├─ Detect self-reference
 │      ├─ Update depends_on
 │      └─ Build initial env with parent values
 │
 └─ evaluateFormulaGraph
         ├─ Evaluate current node
         ├─ If dirty → save + optional update_target
         ├─ BFS over children via calculated_fields_via_depends_on
         └─ For each dependent:
                ├─ Expand dependencies
                ├─ Update env
                ├─ Evaluate
                └─ Save if dirty (and touch update_target)
```

---

## 🔁 `update_target`: Triggering External Updates

`update_target` **does not copy the computed value** to another record.  
Instead, it forces PocketBase to update a datetime field in another collection.

Format:

```
<collection>.<recordId>.<fieldName>
```

Example:

```
cells.ABC123.fx_updated_at
```

What happens:

- whenever the formula changes,
- the plugin sets `fx_updated_at = now()` on the target record,
- PocketBase emits a realtime update for that external record.

Useful when:

- a collection contains a relation to a `calculated_fields` record  
- clients/watchers observe the external collection, not the calculated one  
- you want changes to trigger UI reloads or further server hooks  

---

## 🧯 Error Codes

| Code   | Meaning                              |
|--------|--------------------------------------|
| `1002` | Self-reference in formula            |
| `1003` | Circular dependency detected         |
| `1004` | Syntax error in formula              |
| `1005` | Referenced record not found          |
| `1006` | Runtime error during evaluation      |
| `1007` | Variable not found during DAG walk   |
| `1008` | `update_target` misconfigured        |

---

## 📌 TODO

- Define Access API Rules  
- Add test suite  