package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"

	"golang.org/x/sync/errgroup"
)

// ImplementConfig holds configuration for an implementation run.
type ImplementConfig struct {
	// Name is the decomposition name.
	Name string

	// ProjectRoot is the absolute path to the target project.
	ProjectRoot string

	// OutputDir is the decomposition output directory.
	OutputDir string

	// MaxConcurrent is the maximum number of parallel Claude Code sessions.
	MaxConcurrent int

	// Verbose enables detailed progress output.
	Verbose bool
}

// ImplementerFunc is the function that implements a single milestone.
// This abstraction allows testing without actually spawning Claude Code.
type ImplementerFunc func(ctx context.Context, milestone *MilestoneNode) ([]ImplementationArtifact, error)

// ImplementPipeline coordinates the implementation of a decomposition by
// dispatching milestones to Claude Code sessions in dependency order.
type ImplementPipeline struct {
	cfg         ImplementConfig
	scheduler   *Scheduler
	review      ReviewStrategy
	implementer ImplementerFunc
	progress    *ProgressReporter
}

// NewImplementPipeline creates an implementation pipeline.
func NewImplementPipeline(
	cfg ImplementConfig,
	scheduler *Scheduler,
	review ReviewStrategy,
	implementer ImplementerFunc,
) *ImplementPipeline {
	return &ImplementPipeline{
		cfg:         cfg,
		scheduler:   scheduler,
		review:      review,
		implementer: implementer,
		progress:    NewProgressReporter(),
	}
}

// Progress returns a channel that emits progress events.
func (ip *ImplementPipeline) Progress() <-chan ProgressEvent {
	return ip.progress.Subscribe()
}

// Close shuts down the progress reporter.
func (ip *ImplementPipeline) Close() {
	ip.progress.Close()
}

// Run executes the implementation pipeline, dispatching milestones in
// dependency order with up to MaxConcurrent parallel sessions.
func (ip *ImplementPipeline) Run(ctx context.Context) error {
	maxConcurrent := ip.cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	for {
		ready := ip.scheduler.Ready()
		if len(ready) == 0 {
			if ip.scheduler.AllCompleted() {
				ip.progress.Emit(ProgressEvent{
					Section: "implementation",
					Status:  ProgressComplete,
					Message: "All milestones implemented successfully.",
				})
				return nil
			}
			// No milestones ready and not all completed — means some failed or
			// are blocked by failed dependencies.
			return fmt.Errorf("implementation stalled: no milestones ready, not all completed")
		}

		// Limit concurrency.
		batch := ready
		if len(batch) > maxConcurrent {
			batch = batch[:maxConcurrent]
		}

		g, gctx := errgroup.WithContext(ctx)

		for _, milestone := range batch {
			milestone := milestone // capture
			if err := ip.scheduler.MarkRunning(milestone.ID); err != nil {
				return fmt.Errorf("mark running %s: %w", milestone.ID, err)
			}

			ip.progress.Emit(ProgressEvent{
				Section: milestone.ID,
				Status:  ProgressWorking,
				Message: fmt.Sprintf("Implementing %s: %s", milestone.ID, milestone.Name),
			})

			g.Go(func() error {
				return ip.runMilestone(gctx, milestone)
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}
	}
}

// runMilestone implements a single milestone: run the implementer, then
// go through the review gate.
func (ip *ImplementPipeline) runMilestone(ctx context.Context, milestone *MilestoneNode) error {
	// Run implementation.
	artifacts, err := ip.implementer(ctx, milestone)
	if err != nil {
		_ = ip.scheduler.MarkFailed(milestone.ID)
		ip.progress.Emit(ProgressEvent{
			Section: milestone.ID,
			Status:  ProgressFailed,
			Message: fmt.Sprintf("Implementation failed for %s: %v", milestone.ID, err),
		})
		return fmt.Errorf("implement %s: %w", milestone.ID, err)
	}

	ip.progress.Emit(ProgressEvent{
		Section: milestone.ID,
		Status:  ProgressVerifying,
		Message: fmt.Sprintf("Awaiting review for %s (%d files changed)", milestone.ID, len(artifacts)),
	})

	// Review gate.
	decision, err := ip.review.RequestReview(ctx, milestone, artifacts)
	if err != nil {
		_ = ip.scheduler.MarkFailed(milestone.ID)
		return fmt.Errorf("review %s: %w", milestone.ID, err)
	}

	switch decision {
	case ReviewApproved:
		if _, err := ip.scheduler.MarkCompleted(milestone.ID); err != nil {
			return fmt.Errorf("mark completed %s: %w", milestone.ID, err)
		}
		ip.progress.Emit(ProgressEvent{
			Section: milestone.ID,
			Status:  ProgressComplete,
			Message: fmt.Sprintf("Milestone %s approved and completed.", milestone.ID),
		})
	case ReviewRejected:
		_ = ip.scheduler.MarkFailed(milestone.ID)
		ip.progress.Emit(ProgressEvent{
			Section: milestone.ID,
			Status:  ProgressFailed,
			Message: fmt.Sprintf("Milestone %s rejected.", milestone.ID),
		})
		return fmt.Errorf("milestone %s rejected during review", milestone.ID)
	case ReviewRevise:
		// For now, treat revise as approved with a note.
		if _, err := ip.scheduler.MarkCompleted(milestone.ID); err != nil {
			return fmt.Errorf("mark completed %s: %w", milestone.ID, err)
		}
		ip.progress.Emit(ProgressEvent{
			Section: milestone.ID,
			Status:  ProgressComplete,
			Message: fmt.Sprintf("Milestone %s approved with revisions noted.", milestone.ID),
		})
	}

	return nil
}

// FormatImplementationSummary produces a human-readable summary of the
// implementation run.
func FormatImplementationSummary(milestones []MilestoneNode) string {
	var b strings.Builder
	b.WriteString("## Implementation Summary\n\n")

	completed := 0
	failed := 0
	pending := 0
	for _, m := range milestones {
		var status string
		switch m.Status {
		case MilestoneCompleted:
			status = "completed"
			completed++
		case MilestoneFailed:
			status = "FAILED"
			failed++
		default:
			status = string(m.Status)
			pending++
		}
		fmt.Fprintf(&b, "- **%s** (%s): %s\n", m.ID, m.Name, status)
	}

	fmt.Fprintf(&b, "\n**Total**: %d completed, %d failed, %d pending\n",
		completed, failed, pending)

	if failed > 0 {
		log.Printf("WARNING: %d milestone(s) failed during implementation", failed)
	}

	return b.String()
}
