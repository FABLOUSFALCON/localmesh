# Go Temporal DSL Workflow & Activity Rules

A lightweight, declarative DSL in Go for defining Temporal workflows. This approach uses Go structs to represent workflow logic, separating the definition from the activity implementation.

> **Note:** Temporal is useful for complex async workflows with retries, timeouts, and saga patterns. Consider using this for plugin workflows that need durability (e.g., cloud sync, batch processing).

## Core Concepts

- **Declarative**: Define workflows using Go structs for `Sequence`, `Parallel`, and `ActivityInvocation`
- **Separation of Concerns**: Keep workflow logic (`Workflow` struct) separate from activity execution code
- **Data Flow**: Manage state and pass data between steps using a shared `bindings` map

## Core DSL Structures

We model the workflow using Go structs defined in the `dsl` package:

- `Workflow`: Top-level struct holding initial `Variables` and the `Root` `Statement`
- `Statement`: Represents a single step, containing either an `ActivityInvocation`, `Sequence`, or `Parallel`
- `ActivityInvocation`: Defines how to call a specific Temporal Activity (`Name`, `Arguments`, `Result`)
- `Sequence`: A slice of `Statement`s executed sequentially
- `Parallel`: A slice of `Statement`s executed concurrently

## DSL Go Structures

```go
package dsl

import (
    "time"
    "go.temporal.io/sdk/workflow"
)

type (
    // Workflow is the type used to express the workflow definition.
    // Variables are a map of valuables that can be used as input to Activity.
    Workflow struct {
        Variables map[string]string
        Root      Statement
    }

    // Statement is the building block of dsl workflow.
    // A Statement can be a simple ActivityInvocation or it could be a Sequence or Parallel.
    Statement struct {
        Activity *ActivityInvocation
        Sequence *Sequence
        Parallel *Parallel
    }

    // Sequence consist of a collection of Statements that runs in sequential.
    Sequence struct {
        Elements []*Statement
    }

    // Parallel can be a collection of Statements that runs in parallel.
    Parallel struct {
        Branches []*Statement
    }

    // ActivityInvocation is used to express invoking an Activity.
    // The Arguments define expected arguments as input to the Activity.
    // The Result specifies the name of variable to store the result,
    // which can then be used as arguments to subsequent ActivityInvocation.
    ActivityInvocation struct {
        Name      string
        Arguments []string
        Result    string
    }

    executable interface {
        execute(ctx workflow.Context, bindings map[string]string) error
    }
)
```

## Execution Flow

Workflows defined with this DSL are executed by the `SimpleDSLWorkflow` function:

- Takes `workflow.Context` and the `dsl.Workflow` struct as input
- Initializes a `bindings` map from `Workflow.Variables`
- Recursively executes the `Statement`s starting from `Root`
- Uses `workflow.ExecuteActivity` to invoke activities based on `ActivityInvocation`
- Handles sequential execution for `Sequence` and concurrent execution (with cancellation) for `Parallel`

## Data Handling (`bindings`)

- The `bindings map[string]string` acts as the shared state
- Initial values come from `Workflow.Variables`
- `ActivityInvocation.Arguments` specifies which keys from `bindings` provide input
- `ActivityInvocation.Result` specifies the key in `bindings` to store the activity's output

## Example: Serial Workflow

```yaml
# Execute 3 steps in sequence:
# 1) sampleActivity1: takes arg1 as input, stores result as result1
# 2) sampleActivity2: takes result1 as input, stores result as result2
# 3) sampleActivity3: takes arg2 and result2 as input, stores result as result3

variables:
  arg1: value1
  arg2: value2

root:
  sequence:
    elements:
      - activity:
          name: SampleActivity1
          arguments:
            - arg1
          result: result1
      - activity:
          name: SampleActivity2
          arguments:
            - result1
          result: result2
      - activity:
          name: SampleActivity3
          arguments:
            - arg2
            - result2
          result: result3
```

## Example: Parallel Workflow

```yaml
# Execute with parallel branches:
# 1) activity1: takes arg1, stores result1
# 2) parallel block with two branches:
#    2.1) activity2 -> activity3
#    2.2) activity4 -> activity5
# 3) activity1: takes results from both branches

variables:
  arg1: value1
  arg2: value2
  arg3: value3

root:
  sequence:
    elements:
      - activity:
          name: SampleActivity1
          arguments:
            - arg1
          result: result1
      - parallel:
          branches:
            - sequence:
                elements:
                  - activity:
                      name: SampleActivity2
                      arguments:
                        - result1
                      result: result2
                  - activity:
                      name: SampleActivity3
                      arguments:
                        - arg2
                        - result2
                      result: result3
            - sequence:
                elements:
                  - activity:
                      name: SampleActivity4
                      arguments:
                        - result1
                      result: result4
                  - activity:
                      name: SampleActivity5
                      arguments:
                        - arg3
                        - result4
                      result: result5
      - activity:
          name: SampleActivity1
          arguments:
            - result3
            - result5
          result: result6
```

## Activity Implementation Guidelines

Activities are standard Go functions/methods registered with Temporal:

```go
type Activities struct {
    // Dependencies
    db     *sql.DB
    client *http.Client
}

func (a *Activities) SampleActivity1(ctx context.Context, input string) (string, error) {
    logger := activity.GetLogger(ctx)
    logger.Info("SampleActivity1 started", "input", input)
    
    // Do work...
    result := strings.ToUpper(input)
    
    return result, nil
}
```

### Activity Best Practices

1. Accept `context.Context` as first argument
2. Use `activity.GetLogger(ctx)` for logging
3. Return a result and an error
4. Keep activities idempotent when possible
5. Use heartbeating for long-running activities

## Agentic Workflow Building Steps

| Step | Human | AI | Comment |
|:-----|:-----:|:--:|:--------|
| 1. Requirements | ★★★ | ★☆☆ | Humans define the overall business process and goals |
| 2. High-Level Flow | ★★☆ | ★★☆ | Humans outline main steps, AI suggests DSL structure |
| 3. Activities | ★★☆ | ★★☆ | Humans specify required Activities, AI proposes implementations |
| 4. Data Flow | ★☆☆ | ★★★ | AI determines Variables, Arguments, Result based on flow |
| 5. DSL Implementation | ★☆☆ | ★★★ | AI generates the Workflow struct |
| 6. Review & Refine | ★★☆ | ★★☆ | Humans review, AI refines based on feedback |
| 7. Activity Implementation | ★☆☆ | ★★★ | AI implements Activity functions |

## When to Use Temporal DSL in LocalMesh

Consider using this pattern for:
- **Cloud Sync**: Multi-step sync with rollback on failure
- **Batch Processing**: Processing large datasets with retry logic
- **Complex Plugin Workflows**: Multi-step operations that need durability
- **Saga Patterns**: Distributed transactions across services

For simple request/response operations, standard HTTP handlers are sufficient.
