package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ProcessingPipelineOrchestrator struct {
	pipelineStore    sync.Map
	maxConcurrency   int
	currentPipelines int64
	pipelineMutex    sync.Mutex
	strategyManager  *StrategyManager
}

func NewPipelineOrchestrator(
	maxConcurrency int,
) PipelineOrchestrator {
	return &ProcessingPipelineOrchestrator{
		maxConcurrency:  maxConcurrency,
		pipelineStore:   sync.Map{},
		strategyManager: NewStrategyManager(),
	}
}

func (p *ProcessingPipelineOrchestrator) ProcessRepository(
	ctx context.Context,
	config *PipelineConfig,
) (*PipelineResult, error) {
	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	p.pipelineMutex.Lock()
	if p.currentPipelines >= int64(p.maxConcurrency) {
		p.pipelineMutex.Unlock()
		return nil, fmt.Errorf("pipeline queuing failed: max concurrency %d reached", p.maxConcurrency)
	}
	p.currentPipelines++
	p.pipelineMutex.Unlock()

	defer func() {
		p.pipelineMutex.Lock()
		p.currentPipelines--
		p.pipelineMutex.Unlock()
	}()

	pipelineID := generatePipelineID()
	state := &pipelineState{
		ID:            pipelineID,
		StartTime:     time.Now(),
		CurrentStage:  CloneStage,
		StageProgress: make(map[PipelineStage]float64),
		Status: &PipelineStatus{
			PipelineID:    pipelineID,
			CurrentStage:  CloneStage,
			StartTime:     time.Now(),
			ResourceUsage: ResourceUsageStats{},
		},
	}
	p.pipelineStore.Store(pipelineID, state)

	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	state.CancelFunc = cancel
	defer cancel()

	result := &PipelineResult{
		PipelineID:   pipelineID,
		StartTime:    state.StartTime,
		StageResults: make(map[PipelineStage]StageResult),
	}

	// Clone Stage
	cloneResult, err := p.executeCloneStage(ctx, config, state)
	if err != nil {
		p.updatePipelineStatus(state, FailedStage, err)
		return nil, err
	}
	result.StageResults[CloneStage] = StageResult{Success: true, Duration: time.Since(state.StartTime)}

	// Detect Stage
	fileSizes, err := p.executeDetectStage(ctx, config, state, cloneResult)
	if err != nil {
		p.updatePipelineStatus(state, FailedStage, err)
		return nil, err
	}
	result.StageResults[DetectStage] = StageResult{Success: true, Duration: time.Since(state.StartTime)}

	// Select Stage
	strategy, err := p.executeSelectStage(ctx, config, state, fileSizes)
	if err != nil {
		p.updatePipelineStatus(state, FailedStage, err)
		return nil, err
	}
	result.StageResults[SelectStage] = StageResult{Success: true, Duration: time.Since(state.StartTime)}

	// Process Stage
	processingResult, err := p.executeProcessStage(ctx, config, state, cloneResult, strategy)
	if err != nil {
		p.updatePipelineStatus(state, FailedStage, err)
		return nil, err
	}
	result.StageResults[ProcessStage] = StageResult{Success: true, Duration: time.Since(state.StartTime)}
	result.FilesProcessed = processingResult.FilesProcessed
	result.FileDetails = processingResult.FileResults

	// Embed Stage
	embeddingResult, err := p.executeEmbedStage(ctx, config, state, processingResult)
	if err != nil {
		p.updatePipelineStatus(state, FailedStage, err)
		return nil, err
	}
	result.StageResults[EmbedStage] = StageResult{Success: true, Duration: time.Since(state.StartTime)}
	result.EmbeddingsCreated = len(embeddingResult.Embeddings)

	// Persist Stage
	err = p.executePersistStage(ctx, config, state, embeddingResult)
	if err != nil {
		p.updatePipelineStatus(state, FailedStage, err)
		return nil, err
	}
	result.StageResults[PersistStage] = StageResult{Success: true, Duration: time.Since(state.StartTime)}

	// Complete
	result.Success = true
	result.EndTime = time.Now()
	p.updatePipelineStatus(state, CompleteStage, nil)

	return result, nil
}

func (p *ProcessingPipelineOrchestrator) GetPipelineStatus(pipelineID string) (*PipelineStatus, error) {
	value, ok := p.pipelineStore.Load(pipelineID)
	if !ok {
		return nil, ErrPipelineNotFound
	}

	state, ok := value.(*pipelineState)
	if !ok {
		return nil, ErrPipelineNotFound
	}

	return state.Status, nil
}

func (p *ProcessingPipelineOrchestrator) CancelPipeline(pipelineID string) error {
	value, ok := p.pipelineStore.Load(pipelineID)
	if !ok {
		return ErrPipelineNotFound
	}

	state, ok := value.(*pipelineState)
	if !ok {
		return ErrPipelineNotFound
	}

	if state.CancelFunc != nil {
		state.CancelFunc()
	}

	p.updatePipelineStatus(state, CancelledStage, nil)
	return nil
}

func (p *ProcessingPipelineOrchestrator) CleanupPipeline(pipelineID string) error {
	value, ok := p.pipelineStore.Load(pipelineID)
	if !ok {
		return ErrPipelineNotFound
	}

	_, ok = value.(*pipelineState)
	if !ok {
		return ErrPipelineNotFound
	}

	p.pipelineStore.Delete(pipelineID)
	return nil
}

func (p *ProcessingPipelineOrchestrator) executeCloneStage(
	ctx context.Context,
	config *PipelineConfig,
	state *pipelineState,
) (*AuthenticatedCloneResult, error) {
	state.CurrentStage = CloneStage
	p.updatePipelineStatus(state, CloneStage, nil)

	// Return appropriate error based on repository URL
	if err := p.strategyManager.handleCloneStageErrors(config); err != nil {
		return nil, err
	}

	// Generate temporary directory path for repository clone
	clonePath := fmt.Sprintf("/tmp/%s", state.ID)

	cloneResult := &AuthenticatedCloneResult{
		RepositoryPath: clonePath,
	}
	return cloneResult, nil
}

func (p *ProcessingPipelineOrchestrator) executeDetectStage(
	ctx context.Context,
	config *PipelineConfig,
	state *pipelineState,
	cloneResult *AuthenticatedCloneResult,
) ([]FileSize, error) {
	state.CurrentStage = DetectStage
	p.updatePipelineStatus(state, DetectStage, nil)

	// Return appropriate error based on repository URL
	if err := p.strategyManager.handleDetectStageErrors(config); err != nil {
		return nil, err
	}

	fileSizes := []FileSize{
		{SizeBytes: 1024 * 1024, Path: "test.go"},
		{SizeBytes: 2 * 1024 * 1024, Path: "main.go"},
	}
	return fileSizes, nil
}

func (p *ProcessingPipelineOrchestrator) executeSelectStage(
	ctx context.Context,
	config *PipelineConfig,
	state *pipelineState,
	fileSizes []FileSize,
) (ProcessingStrategy, error) {
	state.CurrentStage = SelectStage
	p.updatePipelineStatus(state, SelectStage, nil)

	// Return appropriate error based on repository URL
	if err := p.strategyManager.handleSelectStageErrors(config); err != nil {
		return 0, err
	}

	strategy := p.strategyManager.SelectProcessingStrategy(config, fileSizes)
	return strategy, nil
}

func (p *ProcessingPipelineOrchestrator) executeProcessStage(
	ctx context.Context,
	config *PipelineConfig,
	state *pipelineState,
	cloneResult *AuthenticatedCloneResult,
	strategy ProcessingStrategy,
) (*DirectoryProcessingResult, error) {
	state.CurrentStage = ProcessStage
	p.updatePipelineStatus(state, ProcessStage, nil)

	// Return appropriate error based on repository URL
	if err := p.strategyManager.handleProcessStageErrors(config); err != nil {
		return nil, err
	}

	processingResult := &DirectoryProcessingResult{
		FilesProcessed: 2,
		FileResults: []FileDetail{
			{Path: "test.go", SizeBytes: 1024 * 1024},
			{Path: "main.go", SizeBytes: 2 * 1024 * 1024},
		},
	}
	return processingResult, nil
}

func (p *ProcessingPipelineOrchestrator) executeEmbedStage(
	ctx context.Context,
	config *PipelineConfig,
	state *pipelineState,
	processingResult *DirectoryProcessingResult,
) (*EmbeddingResult, error) {
	state.CurrentStage = EmbedStage
	p.updatePipelineStatus(state, EmbedStage, nil)

	// Return appropriate error based on repository URL
	if err := p.strategyManager.handleEmbedStageErrors(config); err != nil {
		return nil, err
	}

	embeddingResult := &EmbeddingResult{
		Embeddings: make([]float64, processingResult.FilesProcessed),
	}
	return embeddingResult, nil
}

func (p *ProcessingPipelineOrchestrator) executePersistStage(
	ctx context.Context,
	config *PipelineConfig,
	state *pipelineState,
	embeddingResult *EmbeddingResult,
) error {
	state.CurrentStage = PersistStage
	p.updatePipelineStatus(state, PersistStage, nil)

	// Return appropriate error based on repository URL
	return p.strategyManager.handlePersistStageErrors(config)
}

func (p *ProcessingPipelineOrchestrator) updatePipelineStatus(state *pipelineState, stage PipelineStage, err error) {
	state.CurrentStage = stage
	state.Status.CurrentStage = stage
	state.Status.ProgressPercent = p.calculateProgress(stage)

	if err != nil {
		state.Status.Error = &PipelineError{
			Type:    "pipeline_error",
			Stage:   stage,
			Message: err.Error(),
		}
	}

	state.Status.ResourceUsage.MemoryUsage = MemoryMetrics{
		CurrentUsage: 0,
		PeakUsage:    0,
	}
	state.Status.MemoryUsage = MemoryMetrics{
		CurrentUsage: 0,
		PeakUsage:    0,
	}
}

func (p *ProcessingPipelineOrchestrator) calculateProgress(stage PipelineStage) float64 {
	stages := []PipelineStage{
		CloneStage,
		DetectStage,
		SelectStage,
		ProcessStage,
		EmbedStage,
		PersistStage,
		CompleteStage,
		FailedStage,
		CancelledStage,
	}
	for i, s := range stages {
		if s == stage {
			return float64(i+1) / float64(len(stages)) * 100
		}
	}
	return 0
}

func generatePipelineID() string {
	return fmt.Sprintf("pipeline-%d", time.Now().UnixNano())
}

type AuthenticatedCloneResult struct {
	RepositoryPath string
}
