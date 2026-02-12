---
short_description: "Guide for modeling Server-Sent Events (SSE) in OpenAPI for Speakeasy SDK generation"
long_description: "Comprehensive documentation on how to define SSE endpoints in OpenAPI specifications to enable streaming support in Speakeasy-generated SDKs across TypeScript, Python, Java, Go, and C#"
---

# Modeling Server-Sent Events (SSE) in OpenAPI

Server-Sent Events (SSE) allow servers to push real-time updates to clients over a single HTTP connection. This document explains how to model SSE endpoints in your OpenAPI specification for Speakeasy SDK generation.

## Supported Languages

SSE streaming is supported in the following Speakeasy-generated SDKs:

- **TypeScript** - Exposes async iterables for streaming
- **Python** - Supports async iteration over event streams
- **Java** - Provides streaming event handlers
- **Go** - Implements streaming with channels/iterators
- **C#** - Supports async enumerable streaming

## Basic SSE Response Structure

To model an SSE endpoint in OpenAPI, use the `text/event-stream` content type in your response:

```yaml
paths:
  /events/stream:
    get:
      operationId: streamEvents
      summary: Stream events in real-time
      responses:
        '200':
          description: A stream of server-sent events
          content:
            text/event-stream:
              schema:
                $ref: '#/components/schemas/ServerEvent'
```

## Server Event Schema

Each server-sent event can contain up to four fields. Define your event schema to include these fields:

```yaml
components:
  schemas:
    ServerEvent:
      type: object
      properties:
        id:
          type: string
          description: Optional event identifier for reconnection
        event:
          type: string
          description: Optional event type name
        data:
          type: string
          description: The event payload data
        retry:
          type: integer
          format: int64
          description: Optional reconnection time in milliseconds
```

## JSON Data in Events

If your `data` field contains JSON instead of plain strings, model it as an object:

```yaml
components:
  schemas:
    ServerEvent:
      type: object
      properties:
        event:
          type: string
        data:
          $ref: '#/components/schemas/EventPayload'

    EventPayload:
      type: object
      properties:
        message:
          type: string
        timestamp:
          type: string
          format: date-time
        metadata:
          type: object
          additionalProperties: true
```

When `data` is specified as an object, Speakeasy SDKs will automatically deserialize the JSON content into typed objects.

## Multiple Event Types

For APIs that emit different event types, use discriminated unions:

```yaml
components:
  schemas:
    StreamEvent:
      type: object
      required:
        - event
        - data
      properties:
        event:
          type: string
          enum: [message, error, heartbeat, done]
        data:
          oneOf:
            - $ref: '#/components/schemas/MessageData'
            - $ref: '#/components/schemas/ErrorData'
            - $ref: '#/components/schemas/HeartbeatData'
            - $ref: '#/components/schemas/DoneData'
          discriminator:
            propertyName: type

    MessageData:
      type: object
      properties:
        type:
          type: string
          const: message
        content:
          type: string

    ErrorData:
      type: object
      properties:
        type:
          type: string
          const: error
        code:
          type: string
        message:
          type: string
```

## OpenAPI 3.2.0+ Support

OpenAPI 3.2.0 adds first-class SSE support with the `itemSchema` keyword:

```yaml
paths:
  /events/stream:
    get:
      operationId: streamEvents
      responses:
        '200':
          description: A stream of events
          content:
            text/event-stream:
              itemSchema:
                $ref: '#/components/schemas/EventPayload'
```

The `itemSchema` keyword explicitly indicates that the response is a stream of individual items rather than a single response body.

## Complete Example

Here's a complete example of an SSE endpoint for a chat/AI streaming response:

```yaml
openapi: 3.1.0
info:
  title: Streaming API
  version: 1.0.0
paths:
  /chat/completions:
    post:
      operationId: createChatCompletionStream
      summary: Create a streaming chat completion
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ChatRequest'
      responses:
        '200':
          description: Streaming chat completion response
          content:
            text/event-stream:
              schema:
                $ref: '#/components/schemas/ChatCompletionChunk'

components:
  schemas:
    ChatRequest:
      type: object
      required:
        - messages
      properties:
        messages:
          type: array
          items:
            $ref: '#/components/schemas/Message'
        stream:
          type: boolean
          default: true

    Message:
      type: object
      required:
        - role
        - content
      properties:
        role:
          type: string
          enum: [system, user, assistant]
        content:
          type: string

    ChatCompletionChunk:
      type: object
      properties:
        id:
          type: string
        event:
          type: string
        data:
          $ref: '#/components/schemas/ChunkData'

    ChunkData:
      type: object
      properties:
        choices:
          type: array
          items:
            type: object
            properties:
              delta:
                type: object
                properties:
                  content:
                    type: string
              finish_reason:
                type: string
                nullable: true
```

## Common Issues and Solutions

### Issue: "server sent events are not currently supported" warning

**Cause**: The OpenAPI spec may not have the correct schema structure for Speakeasy to recognize the SSE endpoint.

**Solution**: Ensure your response uses:
1. `text/event-stream` as the content type
2. A schema that follows the SSE event structure (with optional `id`, `event`, `data`, `retry` fields)

### Issue: "server-sent event: unknown field error"

**Cause**: The schema contains fields that don't match the expected SSE event structure.

**Solution**:
- The `data` field should be either a `string` or an `object` type
- If using custom field names, ensure they map to standard SSE fields
- Check that field names match exactly: `id`, `event`, `data`, `retry`

### Issue: Operation skipped entirely

**Cause**: The linter may not recognize the SSE pattern if the schema structure is incorrect.

**Solution**:
1. Verify the content type is exactly `text/event-stream`
2. Use a flat event schema at the response level
3. Avoid deeply nested refs that obscure the event structure

## SDK Usage Examples

### TypeScript

```typescript
const stream = await client.chat.createChatCompletionStream({
  messages: [{ role: 'user', content: 'Hello!' }],
});

for await (const event of stream) {
  console.log(event.data?.choices?.[0]?.delta?.content);
}
```

### Python

```python
stream = await client.chat.create_chat_completion_stream(
    messages=[{"role": "user", "content": "Hello!"}]
)

async for event in stream:
    print(event.data.choices[0].delta.content)
```

### C#

```csharp
var stream = await client.Chat.CreateChatCompletionStreamAsync(
    new ChatRequest { Messages = new[] { new Message { Role = "user", Content = "Hello!" } } }
);

await foreach (var evt in stream)
{
    Console.WriteLine(evt.Data?.Choices?[0]?.Delta?.Content);
}
```

## References

- [Speakeasy SSE Documentation](https://www.speakeasy.com/docs/customize/runtime/server-sent-events)
- [OpenAPI SSE Best Practices](https://www.speakeasy.com/openapi/content/server-sent-events)
- [MDN Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
