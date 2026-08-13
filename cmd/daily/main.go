package main

import (
	"bytes"
	_ "embed"
	"errors"
	f "flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/sascha-andres/reuse/flag"

	obsidianutils "github.com/sascha-andres/obsidian-utils"
	"github.com/sascha-andres/obsidian-utils/internal"
)

type (

	// DayData represents the structured data for a specific date including year, month, day, and a composite date-only string.
	DayData struct {
		// Year represents the year portion of a date in a string format.
		Year string

		// Month represents the month portion of a date in a string format.
		Month string

		// Day represents the day portion of a date in a string format.
		Day string

		// DateOnly represents the combination of year, month, and day as a single string formatted as a date.
		DateOnly string
	}

	// NoteData represents the note-related data for a specific date, including links to previous, next, and current day's data.
	// Previous is the DayData for the prior day relative to the current date.
	// Next is the DayData for the following day relative to the current date.
	// Current holds the DayData for the current date.
	// DailyNoteFolder specifies the directory path for storing daily notes.
	NoteData struct {
		// Previous represents the DayData for the prior day relative to the current date.
		Previous DayData

		// Next represents the DayData for the following day relative to the current date.
		Next DayData

		// Current holds the DayData for the current date.
		Current DayData

		// DailyNoteFolder defines the path or location where daily notes are stored as a string.
		// which is basically the -daily-folder parameter
		DailyNoteFolder string

		// WorkLocation specifies the location associated with the current note's context, typically related to the daily note's metadata.
		WorkLocation string
	}
)

var (
	folder, forDate, until, dailyFolder, templateFile string
	defaultWorkLocation                               = "Office"
	logLevel                                          string
	printConfig, overwrite                            bool
)

//go:embed DNote.md
var defaultTemplateFile string

// init initializes the package by setting up flag options, log flags, and prefix.
func init() {
	internal.AddCommonFlagPrefixes()
	flag.SetEnvPrefix("OBS_UTIL_DAILY")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.StringVar(&defaultWorkLocation, "default-work-location", defaultWorkLocation, "default work location")
	flag.StringVar(&folder, "folder", "", "base path to obsidian vault")
	flag.StringVar(&dailyFolder, "daily-folder", "", "where to store the daily note inside the vault")
	flag.StringVar(&templateFile, "template-file", "", "path to template file")
	flag.BoolVar(&printConfig, "print-config", false, "print configuration")
	flag.BoolVar(&overwrite, "overwrite", false, "overwrite existing file")
	flag.StringVar(&forDate, "for-date", time.Now().Format(time.DateOnly), "date for which to create the daily note (2006-01-02)")
	flag.StringVar(&until, "until", "", "date until which to create daily notes, starting today (2006-01-02 or +N offset); mutually exclusive with -for-date")
}

// main is the entry point of the program.
func main() {
	flag.Parse()
	internal.PrintFlags()

	logger := internal.CreateLogger("OBS_UTIL_DAILY", logLevel)
	if err := run(logger); err != nil {
		logger.Error("error running daily", "err", err)
		os.Exit(1)
	}
}

// resolveDateArg resolves a date argument that is either an absolute date (2006-01-02) or an
// offset relative to today expressed as +N or -N days.
func resolveDateArg(raw string) (string, error) {
	if relativeString, ok := strings.CutPrefix(raw, "-"); ok {
		offset, err := strconv.Atoi(relativeString)
		if err != nil {
			return "", err
		}
		return time.Now().AddDate(0, 0, offset*-1).Format(time.DateOnly), nil
	}
	if relativeString, ok := strings.CutPrefix(raw, "+"); ok {
		offset, err := strconv.Atoi(relativeString)
		if err != nil {
			return "", err
		}
		return time.Now().AddDate(0, 0, offset).Format(time.DateOnly), nil
	}
	return raw, nil
}

// explicitlySet reports whether the named flag was actually passed on the command line.
func explicitlySet(name string) bool {
	set := false
	flag.Visit(func(fl *f.Flag) {
		if fl.Name == name {
			set = true
		}
	})
	return set
}

// run initializes and executes the daily note creation process based on specified folder paths, date, and template configuration.
func run(logger *slog.Logger) error {
	logger.Info("start creating a daily note")

	if folder == "" {
		return errors.New("-folder must be non empty")
	}
	folder, err := obsidianutils.ApplyDirectoryPlaceHolder(folder)
	if err != nil {
		return err
	}
	if dailyFolder == "" {
		return errors.New("-daily-folder must be non empty")
	}
	if until != "" && explicitlySet("for-date") {
		return errors.New("-for-date and -until are mutually exclusive")
	}
	if forDate == "" {
		forDate = time.Now().Format(time.DateOnly)
	}
	forDate, err = resolveDateArg(forDate)
	if err != nil {
		return err
	}

	folder = path.Join(folder, dailyFolder)

	if until != "" {
		resolvedUntil, err := resolveDateArg(until)
		if err != nil {
			return err
		}
		untilTime, err := time.Parse("2006-01-02", resolvedUntil)
		if err != nil {
			return err
		}
		today := time.Now().Truncate(24 * time.Hour)
		if untilTime.Before(today) {
			return errors.New("-until must be a date in the future")
		}

		if printConfig {
			logger.Info("daily notes folder", "folder", folder)
			logger.Info("date range", "from", today.Format(time.DateOnly), "until", untilTime.Format(time.DateOnly))
			return nil
		}

		for t := today; !t.After(untilTime); t = t.AddDate(0, 0, 1) {
			if err := createDailyNote(logger, folder, t); err != nil {
				return err
			}
		}
		return nil
	}

	if printConfig {
		logger.Info("daily notes folder", "folder", folder)
		logger.Info("for-date", "for-date", forDate)
		return nil
	}

	t, err := time.Parse("2006-01-02", forDate)
	if err != nil {
		return err
	}

	return createDailyNote(logger, folder, t)
}

// createDailyNote creates the daily note file for the given date inside folder, unless it already exists (or -overwrite is set).
func createDailyNote(logger *slog.Logger, folder string, t time.Time) error {
	resultingDirectory := path.Join(folder, t.Format("2006/01"))
	resultingFile := path.Join(folder, fmt.Sprintf("%s.md", t.Format("2006/01/2006-01-02")))

	_ = os.MkdirAll(resultingDirectory, 0700)

	if overwrite {
		logger.Info("overwriting existing file", "file", resultingFile)
		_ = os.Remove(resultingFile)
	}

	if _, err := os.Stat(resultingFile); err == nil {
		logger.Warn("file already exists", "file", resultingFile)
		return nil
	}

	logger.Info("creating file", "file", resultingFile)

	tpl, err := executeTemplate(logger, t, nil)
	if err != nil {
		return err
	}

	if err := os.WriteFile(resultingFile, tpl.Bytes(), 0600); err != nil {
		return err
	}
	logger.Info("file created", "file", resultingFile)

	return nil
}

// executeTemplate generates a template using the provided date and error, returning the rendered output or an error.
func executeTemplate(logger *slog.Logger, t time.Time, err error) (bytes.Buffer, error) {
	templateData := NoteData{}
	templateData.DailyNoteFolder = dailyFolder
	templateData.Current.Year = t.Format("2006")
	templateData.Current.Month = t.Format("01")
	templateData.Current.Day = t.Format("02")
	templateData.Current.DateOnly = t.Format("2006-01-02")
	templateData.Previous.Year = t.AddDate(0, 0, -1).Format("2006")
	templateData.Previous.Month = t.AddDate(0, 0, -1).Format("01")
	templateData.Previous.Day = t.AddDate(0, 0, -1).Format("02")
	templateData.Previous.DateOnly = t.AddDate(0, 0, -1).Format("2006-01-02")
	templateData.Next.Year = t.AddDate(0, 0, 1).Format("2006")
	templateData.Next.Month = t.AddDate(0, 0, 1).Format("01")
	templateData.Next.Day = t.AddDate(0, 0, 1).Format("02")
	templateData.Next.DateOnly = t.AddDate(0, 0, 1).Format("2006-01-02")
	templateData.WorkLocation = defaultWorkLocation

	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		templateData.WorkLocation = "Weekend"
	}

	templateContent := ""
	if templateFile != "" {
		data, err := os.ReadFile(templateFile)
		if err != nil {
			logger.Error("could not read template file", "file", templateFile, "err", err)
			return bytes.Buffer{}, err
		}
		templateContent = string(data)
	}
	if templateContent == "" {
		templateContent = defaultTemplateFile
	}
	templateEngine, err := template.New("daily").Parse(templateContent)
	if err != nil {
		logger.Error("could not parse template", "err", err)
		return bytes.Buffer{}, err
	}
	var tpl bytes.Buffer
	err = templateEngine.Execute(&tpl, templateData)
	if err != nil {
		logger.Error("could not execute template", "err", err)
		return bytes.Buffer{}, err
	}
	return tpl, nil
}
