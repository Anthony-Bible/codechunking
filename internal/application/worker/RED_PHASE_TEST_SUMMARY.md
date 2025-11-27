# RED Phase Test Summary - Batch Job Queueing

## Overview
This document summarizes the failing tests created for the JobProcessor batch queueing feature. All tests are currently in RED phase and fail because the current implementation immediately submits batches to Gemini API instead of queuing them for later submission.

## Test File
**Location**: `internal/application/worker/job_processor_batch_queueing_test.go`

## Current vs Expected Behavior

### Current Behavior (Why Tests Fail)
```go
// Line 2234 in job_processor.go
batchJob, err := p.batchEmbeddingService.CreateBatchEmbeddingJobWithRequests(...)
progress.MarkSubmittedToGemini(batchJob.JobID)
p.batchProgressRepo.Save(ctx, progress)
```

### Expected Behavior (After Implementation)
```go
// Serialize request data
requestData, err := json.Marshal(requests)

// Mark as pending submission (NOT processing)
progress.MarkPendingSubmission(requestData)

// Save - BatchSubmitter will handle actual submission
return p.batchProgressRepo.Save(ctx, progress)
```

## Test Suite (9 Tests)

### 1. Core Behavior Tests

#### `TestJobProcessor_SubmitBatchJobAsync_QueuesForSubmission`
**Purpose**: Verify that batches are queued instead of immediately submitted

**Expected Behavior**:
- ✅ Chunks are saved to database
- ✅ Batch progress status = `pending_submission`
- ✅ Request data is stored in batch progress
- ✅ Submission attempts = 0
- ✅ `CreateBatchEmbeddingJobWithRequests()` is NOT called

**Current Failure**:
```
panic: mock: I don't know what to return because the method call was unexpected.
    CreateBatchEmbeddingJobWithRequests(context.backgroundCtx,[]*outbound.BatchEmbeddingRequest,...)
```

**Why It Fails**: Current code immediately calls Gemini API instead of queuing

---

#### `TestJobProcessor_SubmitBatchJobAsync_SerializesRequestData`
**Purpose**: Verify request data is properly serialized and deserializable

**Expected Behavior**:
- ✅ Request data is valid JSON
- ✅ Deserializes to `[]*outbound.BatchEmbeddingRequest`
- ✅ Each request has correct chunk ID encoded as `request_id`
- ✅ Each request has correct text content

**Current Failure**: Request data is `nil` because it's not stored

**Key Assertion**:
```go
requestData := capturedProgress.BatchRequestData()
require.NotNil(t, requestData, "Request data should be stored")

var requests []*outbound.BatchEmbeddingRequest
err = json.Unmarshal(requestData, &requests)
require.NoError(t, err)
```

---

#### `TestJobProcessor_SubmitBatchJobAsync_RequestDataFormat`
**Purpose**: Verify exact format of serialized request data

**Expected Behavior**:
- ✅ RequestID format: `chunk_<uuid_without_hyphens>`
- ✅ Text field contains chunk content
- ✅ Structure matches Gemini API expectations

**Test Data**:
```go
testUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
expectedRequestID := "chunk_550e8400e29b41d4a716446655440000"
```

**Current Failure**: No request data is stored

---

### 2. Error Handling Tests

#### `TestJobProcessor_SubmitBatchJobAsync_ChunkSaveError`
**Purpose**: Verify error handling when chunk save fails

**Expected Behavior**:
- ✅ Error is propagated
- ✅ No batch progress is created
- ✅ No Gemini API call is made

**Current Status**: Should already pass (error handling exists)

---

#### `TestJobProcessor_SubmitBatchJobAsync_BatchProgressSaveError`
**Purpose**: Verify error handling when batch progress save fails

**Expected Behavior**:
- ✅ Error is propagated
- ✅ Chunks are saved (can't roll back)
- ✅ Gemini API is NOT called (new behavior)

**Current Failure**: Gemini API is called before save, so this test panics

---

#### `TestJobProcessor_SubmitBatchJobAsync_NoBatchEmbeddingService`
**Purpose**: Verify error when batch service is not configured

**Expected Behavior**:
- ✅ Returns error "batch embedding service not configured"
- ✅ No chunks saved
- ✅ No batch progress created

**Current Status**: Already passes (error check exists)

---

### 3. Integration Tests

#### `TestJobProcessor_ProcessJob_WithBatching_QueuesAllBatches`
**Purpose**: Verify end-to-end batching queues all batches

**Test Scenario**:
- 250 chunks with batch size 100 = 3 batches
- All batches should be queued

**Expected Behavior**:
- ✅ 3 batch progress records created
- ✅ All have status = `pending_submission`
- ✅ All have request data stored
- ✅ All have 0 submission attempts
- ✅ No Gemini API calls during processing

**Current Failure**: Gemini API is called for each batch

**Key Assertion**:
```go
assert.Len(t, savedBatches, 3, "Should create 3 batches")
mockBatchEmbeddingService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
```

---

#### `TestJobProcessor_ProcessJob_FallbackToSequential_StillWorks`
**Purpose**: Verify sequential processing still works for small repos

**Test Scenario**:
- 2 chunks (below threshold of 10)

**Expected Behavior**:
- ✅ Sequential `GenerateEmbedding()` is called
- ✅ No batch progress created
- ✅ No batch embedding service called

**Current Status**: Should already pass

---

### 4. Data Integrity Tests

#### `TestJobProcessor_SubmitBatchJobAsync_MultipleChunks_CorrectRequestCount`
**Purpose**: Verify request count matches chunk count

**Test Scenario**:
- 5 chunks should generate 5 requests

**Expected Behavior**:
- ✅ Request array length = 5
- ✅ All request IDs are unique
- ✅ Each request corresponds to a chunk

**Current Failure**: No request data stored

---

## Running the Tests

### Run All Queueing Tests
```bash
go test ./internal/application/worker -run TestJobProcessor_SubmitBatchJobAsync -v -timeout 30s
go test ./internal/application/worker -run TestJobProcessor_ProcessJob_WithBatching -v -timeout 30s
```

### Run Individual Test
```bash
go test ./internal/application/worker -run TestJobProcessor_SubmitBatchJobAsync_QueuesForSubmission -v
```

### Expected Output (RED Phase)
```
FAIL: TestJobProcessor_SubmitBatchJobAsync_QueuesForSubmission
panic: mock: I don't know what to return because the method call was unexpected.
    CreateBatchEmbeddingJobWithRequests(...)
```

## Implementation Checklist

To make these tests pass (GREEN phase), modify `submitBatchJobAsync()`:

- [ ] Remove immediate Gemini API call (`CreateBatchEmbeddingJobWithRequests`)
- [ ] Serialize batch requests to JSON (`json.Marshal(requests)`)
- [ ] Call `progress.MarkPendingSubmission(requestData)` instead of `MarkSubmittedToGemini`
- [ ] Save batch progress with `pending_submission` status
- [ ] Return immediately (let BatchSubmitter handle submission)

## Key Code Changes Required

### Before (Current - Line 2234)
```go
// Create batch embedding job using the batch ID for file tracking
batchJob, err := p.batchEmbeddingService.CreateBatchEmbeddingJobWithRequests(ctx, requests, options, progress.ID())
if err != nil {
    return fmt.Errorf("failed to create batch embedding job: %w", err)
}

// Mark as submitted to Gemini with the job ID
if err := progress.MarkSubmittedToGemini(batchJob.JobID); err != nil {
    return fmt.Errorf("invalid batch job ID from Gemini: %w", err)
}
```

### After (Expected)
```go
// Serialize request data for later submission
requestData, err := json.Marshal(requests)
if err != nil {
    return fmt.Errorf("failed to serialize batch request data: %w", err)
}

// Mark as pending submission (BatchSubmitter will handle actual submission)
progress.MarkPendingSubmission(requestData)
```

## Success Criteria

All 9 tests should pass when:
1. Batch progress status is `pending_submission`
2. Request data is stored and valid JSON
3. Gemini API is NOT called during `submitBatchJobAsync()`
4. BatchSubmitter (separate component) handles actual submission

## Notes

- Tests use existing mock infrastructure from `job_processor_test.go`
- Tests follow table-driven patterns where appropriate
- All tests include detailed RED phase expectations in comments
- Tests verify both positive and negative scenarios
- Integration test covers end-to-end flow with multiple batches
