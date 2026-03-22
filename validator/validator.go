package validator

import (
	"context"
	"encoding/json"
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

const defaultBatchSize = 50

// batchResult represents the JSON output from a batch validation script for one account.
type batchResult struct {
	ID     uint   `json:"id"`
	Used   bool   `json:"used"`
	Banned bool   `json:"banned"`
	Error  string `json:"error,omitempty"`
}

// batchInputItem is the JSON input format for each account in a batch.
type batchInputItem struct {
	ID   uint   `json:"id"`
	Data string `json:"data"`
}

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

	// Split accounts into batches for efficient Python execution
	batches := splitIntoBatches(accounts, defaultBatchSize)
	appendLog(fmt.Sprintf("[%s] Starting validation for %d accounts in %d batches (batch size: %d)",
		time.Now().Format("15:04:05"), len(accounts), len(batches), defaultBatchSize))

	// Create a single shared script file for the entire validation run
	scriptContent := buildBatchScript(cat.ValidationScript)
	scriptFile, err := os.CreateTemp("", "validate-batch-*.py")
	if err != nil {
		appendLog(fmt.Sprintf("[%s] ERROR creating script file: %v", time.Now().Format("15:04:05"), err))
		return
	}
	scriptFile.WriteString(scriptContent)
	scriptFile.Close()
	defer os.Remove(scriptFile.Name())

	// Determine Python executable
	venvPython := fmt.Sprintf("./data/venvs/%d/bin/python", cat.ID)
	useVenv := false
	if _, err := os.Stat(venvPython); err == nil {
		useVenv = true
	}

	workerSlots := make(chan int, concurrency)
	for i := 1; i <= concurrency; i++ {
		workerSlots <- i
	}

	for batchIdx, batch := range batches {
		select {
		case <-ctx.Done():
			stopped = true
			appendLog(fmt.Sprintf("[%s] Validation stopped by user", time.Now().Format("15:04:05")))
			goto done
		default:
		}
		wg.Add(1)
		worker := <-workerSlots
		go func(batch []database.Account, batchIdx, worker int) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error.Printf("validator worker panic: %v", r)
				}
			}()
			defer func() { workerSlots <- worker }()
			defer wg.Done()

			// Write batch data to a temp JSON file
			items := make([]batchInputItem, len(batch))
			for i, acc := range batch {
				items[i] = batchInputItem{ID: acc.ID, Data: acc.Data}
			}
			dataJSON, err := json.Marshal(items)
			if err != nil {
				appendLog(fmt.Sprintf("[%s] [W%d] Batch %d: ERROR marshaling data: %v",
					time.Now().Format("15:04:05"), worker, batchIdx+1, err))
				return
			}
			dataFile, err := os.CreateTemp("", "validate-data-*.json")
			if err != nil {
				appendLog(fmt.Sprintf("[%s] [W%d] Batch %d: ERROR creating data file: %v",
					time.Now().Format("15:04:05"), worker, batchIdx+1, err))
				return
			}
			dataFile.Write(dataJSON)
			dataFile.Close()
			defer os.Remove(dataFile.Name())

			// Dynamic timeout: base 60s + 2s per account, capped at 300s
			timeout := time.Duration(60+len(batch)*2) * time.Second
			if timeout > 300*time.Second {
				timeout = 300 * time.Second
			}
			execCtx, execCancel := context.WithTimeout(ctx, timeout)
			defer execCancel()

			var cmd *exec.Cmd
			if useVenv {
				cmd = exec.CommandContext(execCtx, venvPython, scriptFile.Name(), dataFile.Name())
			} else {
				cmd = exec.CommandContext(execCtx, "uv", "run", "--isolated", "--no-project", scriptFile.Name(), dataFile.Name())
			}
			output, err := cmd.CombinedOutput()
			outputStr := strings.TrimSpace(string(output))
			if err != nil {
				appendLog(fmt.Sprintf("[%s] [W%d] Batch %d: ERROR - %s",
					time.Now().Format("15:04:05"), worker, batchIdx+1, outputStr))
				// Count the batch as processed even on error
				newCount := atomic.AddInt32(&processedCount, int32(len(batch)))
				database.DB.Model(&run).Update("processed_count", int(newCount))
				return
			}

			// Parse output: everything before "---BATCH_RESULT---" is script output,
			// the line after is JSON results
			var results []batchResult
			sentinel := "---BATCH_RESULT---"
			sentinelIdx := strings.LastIndex(outputStr, sentinel)
			if sentinelIdx < 0 {
				appendLog(fmt.Sprintf("[%s] [W%d] Batch %d: ERROR - no result sentinel found in output: %s",
					time.Now().Format("15:04:05"), worker, batchIdx+1, outputStr))
				newCount := atomic.AddInt32(&processedCount, int32(len(batch)))
				database.DB.Model(&run).Update("processed_count", int(newCount))
				return
			}

			// Log any script output before the sentinel
			scriptOutput := strings.TrimSpace(outputStr[:sentinelIdx])
			if scriptOutput != "" {
				appendLog(fmt.Sprintf("[%s] [W%d] Batch %d output: %s",
					time.Now().Format("15:04:05"), worker, batchIdx+1, scriptOutput))
			}

			resultJSON := strings.TrimSpace(outputStr[sentinelIdx+len(sentinel):])
			if err := json.Unmarshal([]byte(resultJSON), &results); err != nil {
				appendLog(fmt.Sprintf("[%s] [W%d] Batch %d: ERROR parsing results: %v",
					time.Now().Format("15:04:05"), worker, batchIdx+1, err))
				newCount := atomic.AddInt32(&processedCount, int32(len(batch)))
				database.DB.Model(&run).Update("processed_count", int(newCount))
				return
			}

			// Process results: batch DB updates by status group
			var okIDs, usedIDs, bannedIDs []uint
			for _, r := range results {
				if r.Error != "" {
					appendLog(fmt.Sprintf("[%s] [W%d] Account %d: ERROR - %s",
						time.Now().Format("15:04:05"), worker, r.ID, r.Error))
					continue
				}
				if r.Banned {
					bannedIDs = append(bannedIDs, r.ID)
					atomic.AddInt32(&bannedCount, 1)
					appendLog(fmt.Sprintf("[%s] [W%d] Account %d: BANNED",
						time.Now().Format("15:04:05"), worker, r.ID))
				} else if r.Used {
					usedIDs = append(usedIDs, r.ID)
					atomic.AddInt32(&usedCount, 1)
					appendLog(fmt.Sprintf("[%s] [W%d] Account %d: USED",
						time.Now().Format("15:04:05"), worker, r.ID))
				} else {
					okIDs = append(okIDs, r.ID)
					appendLog(fmt.Sprintf("[%s] [W%d] Account %d: OK",
						time.Now().Format("15:04:05"), worker, r.ID))
				}
			}

			// Batch DB updates — one UPDATE per status group instead of per account
			if len(okIDs) > 0 {
				database.DB.Model(&database.Account{}).Where("id IN ?", okIDs).
					Updates(map[string]interface{}{"used": false, "banned": false})
			}
			if len(usedIDs) > 0 {
				database.DB.Model(&database.Account{}).Where("id IN ?", usedIDs).
					Updates(map[string]interface{}{"used": true, "banned": false})
			}
			if len(bannedIDs) > 0 {
				database.DB.Model(&database.Account{}).Where("id IN ?", bannedIDs).
					Updates(map[string]interface{}{"used": false, "banned": true})
			}

			// Update progress once per batch
			newCount := atomic.AddInt32(&processedCount, int32(len(batch)))
			database.DB.Model(&run).Update("processed_count", int(newCount))
		}(batch, batchIdx, worker)
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

// buildBatchScript generates a Python script that calls the user's validate() function
// for each account in a JSON input file and outputs structured JSON results.
// The user's validation script is embedded unchanged — only the harness around it changes.
func buildBatchScript(validationScript string) string {
	return fmt.Sprintf(`# /// script
# requires-python = ">=3.11"
# ///
import json, sys

%s

with open(sys.argv[1]) as _f:
    _accounts = json.load(_f)
_results = []
for _acc in _accounts:
    try:
        _used, _banned = validate(_acc["data"])
        _results.append({"id": _acc["id"], "used": bool(_used), "banned": bool(_banned)})
    except Exception as _e:
        _results.append({"id": _acc["id"], "error": str(_e)})
print("---BATCH_RESULT---")
print(json.dumps(_results))
`, validationScript)
}

// splitIntoBatches divides a slice of accounts into chunks of the given size.
func splitIntoBatches(accounts []database.Account, size int) [][]database.Account {
	if size <= 0 {
		size = defaultBatchSize
	}
	var batches [][]database.Account
	for i := 0; i < len(accounts); i += size {
		end := i + size
		if end > len(accounts) {
			end = len(accounts)
		}
		batches = append(batches, accounts[i:end])
	}
	return batches
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
