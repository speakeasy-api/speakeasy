# Tasks

### Available Operations

* [CreateTask](#createtask) - Creates a new task.
* [GetTaskByID](#gettaskbyid) - Returns the details of a task.
* [ListTasks](#listtasks) - Returns a list of tasks.
Tasks are sorted by creation date, descending.

* [UpdateTask](#updatetask) - Updates a task's attributes.
This performs a full update that replaces the entire set of attributes.


## CreateTask

Creates a new task.

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.Tasks.CreateTask(ctx, shared.TaskParams{
        AssignTo: "fugit",
        RequestID: "dolorum",
        RequestedResource: "excepturi",
        WorkflowID: sdk.String("tempora"),
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Task != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                              | Type                                                   | Required                                               | Description                                            |
| ------------------------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------ |
| `ctx`                                                  | [context.Context](https://pkg.go.dev/context#Context)  | :heavy_check_mark:                                     | The context to use for the request.                    |
| `request`                                              | [shared.TaskParams](../../models/shared/taskparams.md) | :heavy_check_mark:                                     | The request object to use for the request.             |


### Response

**[*operations.CreateTaskResponse](../../models/operations/createtaskresponse.md), error**


## GetTaskByID

Returns the details of a task.

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.Tasks.GetTaskByID(ctx, operations.GetTaskByIDRequest{
        TaskID: "facilis",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Task != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                      | Type                                                                           | Required                                                                       | Description                                                                    |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| `ctx`                                                                          | [context.Context](https://pkg.go.dev/context#Context)                          | :heavy_check_mark:                                                             | The context to use for the request.                                            |
| `request`                                                                      | [operations.GetTaskByIDRequest](../../models/operations/gettaskbyidrequest.md) | :heavy_check_mark:                                                             | The request object to use for the request.                                     |


### Response

**[*operations.GetTaskByIDResponse](../../models/operations/gettaskbyidresponse.md), error**


## ListTasks

Returns a list of tasks.
Tasks are sorted by creation date, descending.


### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.Tasks.ListTasks(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.Tasks != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.ListTasksResponse](../../models/operations/listtasksresponse.md), error**


## UpdateTask

Updates a task's attributes.
This performs a full update that replaces the entire set of attributes.


### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/operations"
	"openapi/pkg/models/shared"
	"openapi/pkg/types"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.Tasks.UpdateTask(ctx, operations.UpdateTaskRequest{
        Task: shared.Task{
            CreatedAt: types.MustTimeFromString("2022-06-04T09:53:33.742Z"),
            DecisionReason: sdk.String("delectus"),
            ID: "63c969e9-a3ef-4a77-9fb1-4cd66ae395ef",
            RequestID: "quidem",
            RequestableName: "provident",
            RequesterName: "nam",
            Status: shared.TaskStatusDenied,
        },
        TaskID: "blanditiis",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Task != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                    | Type                                                                         | Required                                                                     | Description                                                                  |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `ctx`                                                                        | [context.Context](https://pkg.go.dev/context#Context)                        | :heavy_check_mark:                                                           | The context to use for the request.                                          |
| `request`                                                                    | [operations.UpdateTaskRequest](../../models/operations/updatetaskrequest.md) | :heavy_check_mark:                                                           | The request object to use for the request.                                   |


### Response

**[*operations.UpdateTaskResponse](../../models/operations/updatetaskresponse.md), error**

