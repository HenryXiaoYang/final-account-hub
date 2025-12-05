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

func StartScheduler() {
	cronScheduler = cron.New()
	cronScheduler.Start()
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

	// Load categories with validation scripts
	var categories []database.Category
	database.DB.Where("validation_script != '' AND validation_cron != ''").Find(&categories)

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

	if cat.ValidationScript != "" && cat.ValidationCron != "" {
		addJobForCategory(cat)
	}
}

func addJobForCategory(cat database.Category) {
	catID := cat.ID
	entryID, err := cronScheduler.AddFunc(cat.ValidationCron, func() {
		var c database.Category
		database.DB.First(&c, catID)
		validateCategory(c)
	})
	if err != nil {
		logger.Error.Printf("Failed to add cron job for category %s: %v", cat.Name, err)
		return
	}
	categoryJobs[cat.ID] = entryID
}

func validateCategory(cat database.Category) {
	logger.Info.Printf("Starting validation for category %s (ID: %d)", cat.Name, cat.ID)

	var accounts []database.Account
	database.DB.Where("category_id = ? AND used = false AND banned = false", cat.ID).Find(&accounts)
	logger.Info.Printf("Found %d accounts to validate", len(accounts))

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

	concurrency := cat.ValidationConcurrency
	if concurrency < 1 {
		concurrency = 1
	}

	var wg sync.WaitGroup
	var bannedCount int32
	var processedCount int32
	var logMutex sync.Mutex
	var logBuilder strings.Builder

	appendLog := func(msg string) {
		logMutex.Lock()
		logBuilder.WriteString(msg + "\n")
		database.DB.Model(&run).Update("log", logBuilder.String())
		logMutex.Unlock()
	}

	appendLog(fmt.Sprintf("[%s] Starting validation for %d accounts", time.Now().Format("15:04:05"), len(accounts)))

	workerSlots := make(chan int, concurrency)
	for i := 1; i <= concurrency; i++ {
		workerSlots <- i
	}
	for _, acc := range accounts {
		wg.Add(1)
		worker := <-workerSlots
		go func(acc database.Account, worker int) {
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

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
					status = "USED"
				}
				appendLog(fmt.Sprintf("[%s] [W%d] Account %d: %s", time.Now().Format("15:04:05"), worker, acc.ID, status))
			}
		}(acc, worker)
	}

	wg.Wait()
	appendLog(fmt.Sprintf("[%s] Completed: %d total, %d banned", time.Now().Format("15:04:05"), len(accounts), bannedCount))

	// Update run record
	now := time.Now()
	database.DB.Model(&run).Updates(map[string]interface{}{
		"status":       "success",
		"banned_count": int(bannedCount),
		"finished_at":  now,
	})
	database.DB.Model(&cat).Update("last_validated_at", now)
	logger.Info.Printf("Validated category %s: %d accounts, %d banned", cat.Name, len(accounts), bannedCount)
}

func RunValidationNow(categoryID uint) error {
	var cat database.Category
	if err := database.DB.First(&cat, categoryID).Error; err != nil {
		return err
	}
	if cat.ValidationScript == "" {
		return fmt.Errorf("no validation script")
	}
	go validateCategory(cat)
	return nil
}

func StopScheduler() {
	if cronScheduler != nil {
		cronScheduler.Stop()
	}
}
