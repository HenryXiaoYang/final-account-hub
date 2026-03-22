package validator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"final-account-hub/database"
	"final-account-hub/logger"

	"github.com/robfig/cron/v3"
)

var cronScheduler *cron.Cron
var categoryJobs = make(map[uint]cron.EntryID)
var jobsMutex sync.Mutex
var runningValidations = make(map[uint]context.CancelFunc)
var runningMutex sync.Mutex

func StartScheduler() {
	cronScheduler = cron.New()
	cronScheduler.Start()

	// Snapshot cron jobs
	cronScheduler.AddFunc("@every 1h", func() { database.TakeSnapshots("1h") })
	cronScheduler.AddFunc("0 0 * * *", func() { database.TakeSnapshots("1d") })
	cronScheduler.AddFunc("0 0 * * 1", func() { database.TakeSnapshots("1w") })
	cronScheduler.AddFunc("0 1 * * *", func() { database.CleanupOldSnapshots() })

	// Take initial snapshots on startup so charts are not empty
	go func() {
		database.TakeSnapshots("1h")
		database.TakeSnapshots("1d")
		database.TakeSnapshots("1w")
	}()

	ReloadAllJobs()
}

func ReloadAllJobs() {
	jobsMutex.Lock()
	defer jobsMutex.Unlock()

	// Remove all existing jobs
	for _, entryID := range categoryJobs {
		cronScheduler.Remove(entryID)
	}
	categoryJobs = make(map[uint]cron.EntryID)

	// Load categories with validation scripts (only enabled ones)
	var categories []database.Category
	database.DB.Where("validation_script != '' AND validation_cron != '' AND validation_enabled = ?", true).Find(&categories)

	for _, cat := range categories {
		addJobForCategory(cat)
	}
}

func ReloadJobForCategory(categoryID uint) {
	jobsMutex.Lock()
	defer jobsMutex.Unlock()

	// Remove existing job
	if entryID, exists := categoryJobs[categoryID]; exists {
		cronScheduler.Remove(entryID)
		delete(categoryJobs, categoryID)
	}

	// Load category
	var cat database.Category
	if err := database.DB.First(&cat, categoryID).Error; err != nil {
		return
	}

	if cat.ValidationEnabled && cat.ValidationScript != "" && cat.ValidationCron != "" {
		addJobForCategory(cat)
	}
}

func addJobForCategory(cat database.Category) {
	catID := cat.ID
	entryID, err := cronScheduler.AddFunc(cat.ValidationCron, func() {
		var c database.Category
		if err := database.DB.First(&c, catID).Error; err != nil {
			logger.Error.Printf("Failed to load category %d for validation: %v", catID, err)
			return
		}
		validateCategory(c)
	})
	if err != nil {
		logger.Error.Printf("Failed to add cron job for category %s: %v", cat.Name, err)
		return
	}
	categoryJobs[cat.ID] = entryID
}

func validateCategory(cat database.Category) {
	// Skip if already running for this category
	runningMutex.Lock()
	if _, running := runningValidations[cat.ID]; running {
		runningMutex.Unlock()
		logger.Info.Printf("Skipping validation for category %s (ID: %d): already running", cat.Name, cat.ID)
		return
	}
	runningMutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	runningMutex.Lock()
	runningValidations[cat.ID] = cancel
	runningMutex.Unlock()
	defer func() {
		runningMutex.Lock()
		delete(runningValidations, cat.ID)
		runningMutex.Unlock()
	}()

	logger.Info.Printf("Starting validation for category %s (ID: %d)", cat.Name, cat.ID)

	// Build scope-based WHERE clause
	scopeConditions := buildScopeConditions(cat.ValidationScope)
	var accounts []database.Account
	database.DB.Where("category_id = ? AND ("+scopeConditions+")", cat.ID).Limit(100000).Find(&accounts)
	logger.Info.Printf("Found %d accounts to validate (scope: %s)", len(accounts), cat.ValidationScope)

	// Create run record
	run := database.ValidationRun{
		CategoryID: cat.ID,
		Status:     "running",
		TotalCount: len(accounts),
		StartedAt:  time.Now(),
	}
	if err := database.DB.Create(&run).Error; err != nil {
		logger.Error.Printf("Failed to create run record: %v", err)
		return
	}
	logger.Info.Printf("Created run record ID: %d", run.ID)
	if err := database.CleanupValidationRuns(cat.ID, cat.ValidationHistoryLimit); err != nil {
		logger.Error.Printf("Failed to cleanup old validation runs: %v", err)
	}
	var stopped bool

	concurrency := cat.ValidationConcurrency
	if concurrency < 1 {
		concurrency = 1
	} else if concurrency > 100 {
		concurrency = 100
	}

	var wg sync.WaitGroup
	var bannedCount int32
	var usedCount int32
	var processedCount int32
	var logMutex sync.Mutex
	var logBuilder strings.Builder
	const maxLogSize = 1 << 20 // 1MB
	logDirty := false

	appendLog := func(msg string) {
		logMutex.Lock()
		defer logMutex.Unlock()
		if logBuilder.Len() >= maxLogSize {
			return
		}
		logBuilder.WriteString(msg + "\n")
		logDirty = true
	}

	// Periodic log flush: write to DB every 5 seconds instead of per-line
	flushDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logMutex.Lock()
				if logDirty {
					database.DB.Model(&run).Update("log", logBuilder.String())
					logDirty = false
				}
				logMutex.Unlock()
			case <-flushDone:
				return
			}
		}
	}()

	appendLog(fmt.Sprintf("[%s] Starting validation for %d accounts", time.Now().Format("15:04:05"), len(accounts)))

	workerSlots := make(chan int, concurrency)
	for i := 1; i <= concurrency; i++ {
		workerSlots <- i
	}
	for _, acc := range accounts {
		select {
		case <-ctx.Done():
			stopped = true
			appendLog(fmt.Sprintf("[%s] Validation stopped by user", time.Now().Format("15:04:05")))
			goto done
		default:
		}
		wg.Add(1)
		worker := <-workerSlots
		go func(acc database.Account, worker int) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error.Printf("validator worker panic: %v", r)
				}
			}()
			defer func() { workerSlots <- worker }()
			defer wg.Done()
			defer func() {
				atomic.AddInt32(&processedCount, 1)
				database.DB.Model(&run).Update("processed_count", atomic.LoadInt32(&processedCount))
			}()

			script := fmt.Sprintf(`# /// script
# requires-python = ">=3.11"
# ///
%s
used, banned = validate(%q)
print(used)
print(banned)
`, cat.ValidationScript, acc.Data)

			tmpFile, err := os.CreateTemp("", "validate-*.py")
			if err != nil {
				appendLog(fmt.Sprintf("[%s] ERROR creating temp file for account %d: %v", time.Now().Format("15:04:05"), acc.ID, err))
				return
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)
			tmpFile.WriteString(script)
			tmpFile.Close()

			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			venvPython := fmt.Sprintf("./data/venvs/%d/bin/python", cat.ID)
			var cmd *exec.Cmd
			if _, err := os.Stat(venvPython); err == nil {
				cmd = exec.CommandContext(ctx, venvPython, tmpPath)
			} else {
				cmd = exec.CommandContext(ctx, "uv", "run", "--isolated", "--no-project", tmpPath)
			}
			output, err := cmd.CombinedOutput()
			outputStr := strings.TrimSpace(string(output))
			if err != nil {
				appendLog(fmt.Sprintf("[%s] [W%d] Account %d: ERROR - %s", time.Now().Format("15:04:05"), worker, acc.ID, outputStr))
				return
			}

			lines := strings.Split(outputStr, "\n")
			if len(lines) >= 2 {
				// Last two lines are used/banned, everything before is script output
				scriptOutput := strings.Join(lines[:len(lines)-2], "\n")
				if scriptOutput != "" {
					appendLog(fmt.Sprintf("[%s] [W%d] Account %d: %s", time.Now().Format("15:04:05"), worker, acc.ID, scriptOutput))
				}
				used := lines[len(lines)-2] == "True"
				banned := lines[len(lines)-1] == "True"
				database.DB.Model(&acc).Updates(map[string]interface{}{"used": used, "banned": banned})
				status := "OK"
				if banned {
					atomic.AddInt32(&bannedCount, 1)
					status = "BANNED"
				} else if used {
					atomic.AddInt32(&usedCount, 1)
					status = "USED"
				}
				appendLog(fmt.Sprintf("[%s] [W%d] Account %d: %s", time.Now().Format("15:04:05"), worker, acc.ID, status))
			}
		}(acc, worker)
	}

done:
	wg.Wait()
	close(flushDone) // Stop the periodic flush goroutine

	// Check if ctx was cancelled (either during dispatch or during wg.Wait)
	if !stopped {
		select {
		case <-ctx.Done():
			stopped = true
		default:
		}
	}

	// Also check DB — StopValidation may have set status to "stopping"
	if !stopped {
		var currentRun database.ValidationRun
		if database.DB.Select("status").First(&currentRun, run.ID).Error == nil && currentRun.Status == "stopping" {
			stopped = true
		}
	}

	now := time.Now()
	finalStatus := "success"
	if stopped {
		finalStatus = "stopped"
		appendLog(fmt.Sprintf("[%s] Stopped: %d processed, %d used, %d banned", time.Now().Format("15:04:05"), processedCount, usedCount, bannedCount))
	} else {
		appendLog(fmt.Sprintf("[%s] Completed: %d total, %d used, %d banned", time.Now().Format("15:04:05"), len(accounts), usedCount, bannedCount))
	}

	// Final log flush + status update
	logMutex.Lock()
	finalLog := logBuilder.String()
	logMutex.Unlock()

	database.DB.Model(&run).Updates(map[string]interface{}{
		"status":       finalStatus,
		"used_count":   int(usedCount),
		"banned_count": int(bannedCount),
		"finished_at":  now,
		"log":          finalLog,
	})
	database.DB.Model(&cat).Update("last_validated_at", now)
	logger.Info.Printf("Validated category %s: %d accounts, %d banned", cat.Name, len(accounts), bannedCount)
}

// buildScopeConditions converts a comma-separated scope string into SQL OR conditions.
// Valid values: "available", "used", "banned".
func buildScopeConditions(scope string) string {
	if scope == "" {
		scope = "available,used"
	}
	parts := strings.Split(scope, ",")
	var conditions []string
	for _, p := range parts {
		switch strings.TrimSpace(p) {
		case "available":
			conditions = append(conditions, "(used = false AND banned = false)")
		case "used":
			conditions = append(conditions, "(used = true AND banned = false)")
		case "banned":
			conditions = append(conditions, "(banned = true)")
		}
	}
	if len(conditions) == 0 {
		return "1=0" // No valid scope — match nothing
	}
	return strings.Join(conditions, " OR ")
}

func RunValidationNow(categoryID uint) error {
	var cat database.Category
	if err := database.DB.First(&cat, categoryID).Error; err != nil {
		return err
	}
	if !cat.ValidationEnabled {
		return fmt.Errorf("validation is disabled for this category")
	}
	if cat.ValidationScript == "" {
		return fmt.Errorf("no validation script")
	}
	// Prevent duplicate runs for the same category
	runningMutex.Lock()
	if _, running := runningValidations[categoryID]; running {
		runningMutex.Unlock()
		return fmt.Errorf("validation already running")
	}
	runningMutex.Unlock()
	go validateCategory(cat)
	return nil
}

func StopValidation(categoryID uint) bool {
	runningMutex.Lock()
	defer runningMutex.Unlock()
	if cancel, ok := runningValidations[categoryID]; ok {
		cancel()
		// Immediately mark the DB record as "stopping" so the UI reflects it
		database.DB.Model(&database.ValidationRun{}).
			Where("category_id = ? AND status = ?", categoryID, "running").
			Update("status", "stopping")
		return true
	}
	return false
}

func StopScheduler() {
	if cronScheduler != nil {
		cronScheduler.Stop()
	}
}

// InitSchedulerForTest initializes the cron scheduler without starting
// background snapshot jobs. Use in tests where handlers call
// ReloadJobForCategory and the scheduler must be non-nil.
func InitSchedulerForTest() {
	cronScheduler = cron.New()
	cronScheduler.Start()
}
