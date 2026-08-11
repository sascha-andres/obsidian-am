package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sascha-andres/reuse/flag"

	obsidianutils "github.com/sascha-andres/obsidian-utils"
	"github.com/sascha-andres/obsidian-utils/internal"
)

// weightKey is the frontmatter key holding the daily weight entry.
const weightKey = "weight"

var (
	folder, dailyFolder, startDateFlag, logLevel string
	printConfig, dryRun                          bool
)

// init initializes the package by setting up flag options, log flags, and prefix.
func init() {
	internal.AddCommonFlagPrefixes()
	flag.SetEnvPrefix("OBS_UTIL_WEIGHT")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.StringVar(&folder, "folder", "", "base path to obsidian vault")
	flag.StringVar(&dailyFolder, "daily-folder", "", "where the daily notes are stored inside the vault")
	flag.StringVar(&startDateFlag, "start-date", "", "lowest date to start scanning from (2006-01-02)")
	flag.BoolVar(&printConfig, "print-config", false, "print configuration")
	flag.BoolVar(&dryRun, "dry-run", false, "do not write files, only log what would change")
}

// main is the entry point of the program.
func main() {
	flag.Parse()
	internal.PrintFlags()

	logger := internal.CreateLogger(logLevel, "OBS_UTIL_WEIGHT")
	if err := run(logger); err != nil {
		logger.Error("error running weight-tool", "err", err)
		os.Exit(1)
	}
}

// run scans daily notes from -start-date up to today and forward-fills zero/missing weight entries.
func run(logger *slog.Logger) error {
	if folder == "" {
		return errors.New("-folder must be non empty")
	}
	resolvedFolder, err := obsidianutils.ApplyDirectoryPlaceHolder(folder)
	if err != nil {
		return err
	}
	if dailyFolder == "" {
		return errors.New("-daily-folder must be non empty")
	}
	if startDateFlag == "" {
		return errors.New("-start-date must be non empty")
	}

	startDate, err := time.Parse(time.DateOnly, startDateFlag)
	if err != nil {
		return err
	}
	today, err := time.Parse(time.DateOnly, time.Now().Format(time.DateOnly))
	if err != nil {
		return err
	}

	if printConfig {
		logger.Info("folder", "folder", resolvedFolder)
		logger.Info("daily-folder", "daily-folder", dailyFolder)
		logger.Info("start-date", "start-date", startDate.Format(time.DateOnly))
		return nil
	}

	if startDate.After(today) {
		return errors.New("-start-date must not be in the future")
	}

	baseFolder := path.Join(resolvedFolder, dailyFolder)

	var (
		memoized     any
		haveMemoized bool
		scanned      int
		updated      int
	)

	for t := startDate; !t.After(today); t = t.AddDate(0, 0, 1) {
		notePath := path.Join(baseFolder, fmt.Sprintf("%s.md", t.Format("2006/01/2006-01-02")))
		exists, err := internal.Exists(notePath)
		if err != nil {
			return err
		}
		if !exists {
			logger.Debug("daily note does not exist, skipping", "file", notePath)
			continue
		}

		scanned++
		processor := obsidianutils.NewSimpleFrontmatterProcessor(notePath)
		value, getErr := processor.GetValue(weightKey)
		present := getErr == nil

		isZero, ok := isZeroWeight(value, present)
		if !ok {
			logger.Warn("could not interpret weight value, skipping", "file", notePath, "value", value)
			continue
		}

		if !isZero {
			memoized = value
			haveMemoized = true
			logger.Debug("memorized weight", "file", notePath, "value", value)
			continue
		}

		if !haveMemoized {
			logger.Debug("zero weight and nothing memorized yet, leaving as is", "file", notePath)
			continue
		}

		if !t.Before(today) {
			logger.Debug("skipping today's or a future daily note", "file", notePath)
			continue
		}

		logger.Debug("backfilling weight", "file", notePath, "old", value, "new", memoized)
		if dryRun {
			updated++
			continue
		}

		if err := processor.SetValue(weightKey, memoized); err != nil {
			return err
		}
		doc, err := processor.GenerateMarkDownDocument()
		if err != nil {
			return err
		}
		if err := os.WriteFile(notePath, doc, 0600); err != nil {
			return err
		}
		updated++
	}

	logger.Info("done scanning daily notes", "scanned", scanned, "updated", updated, "dry-run", dryRun)
	return nil
}

// isZeroWeight determines whether a frontmatter weight value counts as zero.
// A missing key (present == false) is treated as zero. ok is false when the
// value's type or content can't be safely interpreted as a number, in which
// case the caller should skip the file rather than guess.
func isZeroWeight(value any, present bool) (isZero bool, ok bool) {
	if !present {
		return true, true
	}
	switch v := value.(type) {
	case int:
		return v == 0, true
	case int64:
		return v == 0, true
	case float64:
		return v == 0, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return false, false
		}
		return f == 0, true
	default:
		return false, false
	}
}
