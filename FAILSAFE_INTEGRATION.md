# Integrating Failsafe-go with go-redis for Custom Retry Logic

## Overview

This document describes how to modify the go-redis library to remove its built-in retry/backoff mechanisms and replace them with failsafe-go, while maintaining the ability to use a custom `ShouldRetry` function that has access to Redis responses.

## Current State

The go-redis library currently has retry and backoff configuration directly in the `Options` struct:
- `MaxRetries`: maximum number of retries
- `MinRetryBackoff`: minimum backoff between retries
- `MaxRetryBackoff`: maximum backoff between retries
- `DialerRetryBackoff`: controls delay between dial retry attempts

These are used throughout the codebase in:
- `options.go` - Options struct and related retry logic
- `ring.go` - RingOptions struct (copies many Options fields)
- Internal retry mechanisms in various client types

## Proposed Changes

### 1. Remove Retry/Backoff Fields from Options

Remove these fields from the `Options` struct in `options.go`:
- `MaxRetries`
- `MinRetryBackoff` 
- `MaxRetryBackoff`
- `DialerRetryBackoff` (keep DialerRetries and DialerRetryTimeout for connection dialing only)
- `DialerRetries` (keep for connection establishment only)
- `DialerRetryTimeout` (keep for connection establishment only)

### 2. Add Failsafe Integration

Add a new field to `Options`:
```go
// Failsafe provides retry policy configuration.
// If nil, no automatic retries are performed (except for connection establishment).
Failsafe *failsafe.Options
```

### 3. Modify RingOptions

Apply similar changes to `RingOptions` in `ring.go`:
- Remove copied retry/backoff fields
- Add `Failsafe *failsafe.Options` field
- Update `clientOptions()` method to not set MaxRetries to -1 (let failsafe handle it)

### 4. Implement Custom ShouldRetry Function

The key requirement is to allow a custom `ShouldRetry` function that can examine Redis responses. We'll achieve this by:

1. Adding a field to `failsafe.Options` (or creating a wrapper) that accepts a retry decision function
2. This function will receive the error from Redis and can return whether to retry
3. The function will have access to the full error context, including any Lua script error messages

### 5. Update Retry Logic Throughout Codebase

Replace all direct uses of:
- `opt.MaxRetries`
- `opt.MinRetryBackoff`/`opt.MaxRetryBackoff` 
- `retryBackoff()` function
- Manual retry loops

With failsafe-go execution wrappers.

### 6. Connection Establishment Retries

Keep the existing dialer retry logic (`DialerRetries`, `DialerRetryTimeout`) for establishing initial connections, as this is separate from command execution retries.

## Implementation Details

### Failsafe-go Integration

Failsafe-go provides policies like:
- RetryPolicy: defines when and how to retry
- CircuitBreaker: prevents calls when service is failing
- Bulkhead: limits concurrent executions
- Timeout: applies timeouts to operations
- Fallback: provides alternative execution paths

For our use case, we'll primarily use `RetryPolicy` with a custom retry condition function.

### Custom ShouldRetry Access

The custom retry function will need access to:
- The error returned by Redis
- Potentially the command that was executed (for context)
- Any response data from Lua scripts

We can design the function signature as:
```go
type ShouldRetryFunc func(error, *Command) bool
```

Or if we want to keep it simpler and just pass the error:
```go
type ShouldRetryFunc func(error) bool
```

The error type from go-redis (`Nil`, `Cmdable`, etc.) can be inspected to determine if it's retryable based on Lua script responses.

### Backward Compatibility

To maintain some backward compatibility during transition:
- Provide helper functions to convert old Options to Failsafe options
- Or keep the fields marked as deprecated but ignored when Failsafe is set

## Files to Modify

1. `options.go` - Remove retry fields, add Failsafe field
2. `ring.go` - Apply same changes to RingOptions
3. Various client files where retry logic is implemented (need to identify these)
4. Internal retry helper functions

## Testing Approach

1. Ensure existing functionality works with nil Failsafe (no retries)
2. Verify Failsafe integration works with standard retry policies
3. Test custom ShouldRetry function with Lua script error scenarios
4. Ensure connection establishment retries still work separately
5. Test Ring client behavior with multiple shards

## Benefits

1. Separation of concerns: go-redis focuses on Redis protocol, failsafe handles resilience
2. More flexible retry policies (jitter, exponential backoff, etc.) via failsafe
3. Ability to plug in other resilience patterns (circuit breaker, timeout, etc.)
4. Custom retry logic can examine full error context including Lua script responses
5. Reduced complexity in go-redis codebase

## Next Steps

1. Identify all locations where retry logic is currently implemented
2. Design the Failsafe integration interface
3. Implement changes incrementally, starting with Options struct
4. Update retry logic to use failsafe-go
5. Test thoroughly with various error scenarios