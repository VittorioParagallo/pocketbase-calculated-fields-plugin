# PocketBase Calculated Fields

This project implements a formula engine inside PocketBase using custom record hooks and relations. Each record acts like a spreadsheet cell, where values can be computed based on references to other records.

## Features

- Evaluate dynamic expressions using the `expr` library.
- Support for dependency tracking and automatic recalculation.
- Circular dependency detection.
- Reference deletion propagation (`#REF!`).
- Optional `update_target` field to update external fields on value change.

## How it works

The system is centered around a custom `calculated_fields` collection where each record acts as a "cell" in a graph of expressions.

### Collection schema

| Field         | Type       | Description |
|---------------|------------|-------------|
| `formula`     | Text       | An expression referencing other record IDs. |
| `value`       | JSON       | Computed value (evaluated from formula). |
| `error`       | Text       | Error message if formula evaluation fails. |
| `depends_on`  | Relation[] | References to other `calculated_fields` used in the formula. |
| `update_target` | Text     | Optional field in the format `collection.recordId.field`, updated with the new value on change. it updates the referenced record field whith the new date time on every calculation. Useful in case of an external collection watch |

### Evaluation model

- On creation or update of a `calculated_fields` record:
  - Formula is parsed to identify dependencies.
  - Dependencies are saved in the `depends_on` field.
  - The formula is evaluated using the current environment.
  - If the value or error changes, it propagates to all children using a BFS traversal.

- On deletion:
  - All formulas referencing the deleted node are updated with `#REF!`.
  - The referencing nodes are recalculated accordingly.

### Usage example

```go
app.OnRecordCreate("calculated_fields").BindFunc(OnCalculatedFieldsCreateUpdate)