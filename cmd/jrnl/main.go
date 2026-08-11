package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sascha-andres/reuse/flag"

	obsidianutils "github.com/sascha-andres/obsidian-utils"
	"github.com/sascha-andres/obsidian-utils/internal"
)

var (
	folder, forDate, dailyFolder string
	headline                     = "## Other stuff"
	logLevel                     string
	dryRun                       bool
)

func init() {
	internal.AddCommonFlagPrefixes()
	flag.SetEnvPrefix("OBS_UTIL_JRNL")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.StringVar(&folder, "folder", "", "base path to obsidian vault")
	flag.StringVar(&dailyFolder, "daily-folder", "", "where to store the daily note inside the vault")
	flag.StringVar(&forDate, "for-date", time.Now().Format(time.DateOnly), "date for which to create the daily note (2006-01-02)")
	flag.StringVar(&headline, "headline", headline, fmt.Sprintf("headline under which to place the journal note (default: %s)", headline))
	flag.BoolVar(&dryRun, "dry-run", false, "pass to not edit file but to print added line with some context")
}

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		panic(err)
	}
}

func run(_ context.Context) error {
	flag.Parse()
	internal.PrintFlags()
	fmt.Printf("--- %#v", os.Args)

	logger := internal.CreateLogger("OBS_UTIL_DAILY", logLevel)

	logger.Debug("start adding a journal note")
	dailyNoteFolder, err := constructFolder()
	if err != nil {
		return err
	}

	t, err := time.Parse("2006-01-02", forDate)
	if err != nil {
		return err
	}

	resultingFile := path.Join(dailyNoteFolder, fmt.Sprintf("%s.md", t.Format("2006/01/2006-01-02")))
	if e, _ := internal.Exists(resultingFile); !e {
		return fmt.Errorf("file %s does not exist, consider to create with daily", resultingFile)
	}

	fileData, err := os.ReadFile(resultingFile)
	if err != nil {
		return err
	}

	defaultEntry := strings.Join(flag.GetVerbs(), " ")
	bulletPoint, err := internal.PromptText("Journal entry", defaultEntry, func(s string) error {
		if len(s) == 0 {
			return errors.New("journal entry cannot be empty")
		}
		return nil
	})

	if err != nil {
		return err
	}
	line := ""
	if strings.HasPrefix(bulletPoint, "[ ]") {
		line = fmt.Sprintf("%s", bulletPoint)
	} else {
		line = fmt.Sprintf("(%s) %s", time.Now().Format("15:04"), bulletPoint)
	}

	newFileData, err := addBulletpoint(fileData, line, headline)
	if err != nil {
		logger.Error("could not add bullet point", "err", err, "file", resultingFile, "headline", headline)
		return err
	}

	if dryRun {
		d := cmp.Diff(string(fileData), string(newFileData))
		if d == "" {
			logger.Error("no changes detected")
			return fmt.Errorf("no changes detected")
		}
		fmt.Println(d)
		return nil
	}

	return os.WriteFile(resultingFile, newFileData, 0600)
}

func addBulletpoint(data []byte, bulletPoint, after string) ([]byte, error) {
	// Convert to lines for easier manipulation
	content := string(data)
	lines := strings.Split(content, "\n")

	trimEq := func(a, b string) bool { return strings.TrimSpace(a) == strings.TrimSpace(b) }
	isHeadline := func(s string) bool {
		// A markdown headline starts with one or more '#'
		s = strings.TrimSpace(s)
		return strings.HasPrefix(s, "#")
	}
	isULItem := func(s string) (bool, string, rune) {
		// Detect unordered list item of the form: optional spaces + ('-', '*', '+') + space
		// Returns (ok, indent, marker)
		if s == "" {
			return false, "", 0
		}
		// Count leading spaces
		i := 0
		for i < len(s) && s[i] == ' ' {
			i++
		}
		indent := s[:i]
		rest := s[i:]
		if len(rest) < 2 {
			return false, "", 0
		}
		switch rest[0] {
		case '-', '*', '+':
			if len(rest) >= 2 && rest[1] == ' ' {
				return true, indent, rune(rest[0])
			}
		}
		return false, "", 0
	}

	// 1) Find the starting point line matching 'after'
	start := -1
	for i := range lines {
		if trimEq(lines[i], after) {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("anchor line not found: %q", after)
	}

	// 2) Determine the scan window: from line after 'after' to before the next headline
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isHeadline(lines[i]) {
			end = i
			break
		}
	}

	// 3) Search for the first unordered list in [start+1, end)
	listStart := -1
	listIndent := ""
	listMarker := '-'
	for i := start + 1; i < end; i++ {
		if ok, indent, marker := isULItem(lines[i]); ok {
			listStart = i
			listIndent = indent
			listMarker = marker
			break
		}
	}

	if listStart != -1 {
		// 3a) Found a list. Find the last consecutive list item line to append after.
		listEnd := listStart
		for i := listStart; i < end; i++ {
			if ok, _, _ := isULItem(lines[i]); ok {
				listEnd = i
				continue
			}
			break
		}
		// Insert a new list item after listEnd
		insertion := listEnd + 1
		newLine := fmt.Sprintf("%s%c %s", listIndent, listMarker, bulletPoint)
		// Insert while preserving order
		lines = append(lines[:insertion], append([]string{newLine}, lines[insertion:]...)...)
	} else {
		// 3b) No list found before next headline. Create a new list with the bullet point as first item.
		// Requirement: ensure exactly one empty line between the starting line and the newly created list.
		blankStart := start + 1
		blankEnd := blankStart
		// Consume existing blank lines right after the anchor (but stop at next headline boundary)
		for blankEnd < len(lines) && blankEnd < end && strings.TrimSpace(lines[blankEnd]) == "" {
			blankEnd++
		}
		// Ensure there is exactly one blank line between the anchor and the list:
		// - If there were no blank lines, insert one at blankStart.
		// - If there were multiple, collapse them to a single one by removing extras.
		if blankStart >= len(lines) {
			// Anchor was the last line; just append the one blank line and the new list item.
			lines = append(lines, "", fmt.Sprintf("- %s", bulletPoint))
		} else {
			// We will make sure lines[blankStart] is a blank line and remove any additional blank lines up to blankEnd.
			if strings.TrimSpace(lines[blankStart]) != "" {
				// Insert a blank line at blankStart
				lines = append(lines[:blankStart], append([]string{""}, lines[blankStart:]...)...)
				// After insertion, the first non-blank shifts by +1
				blankEnd = blankStart + 1
				if blankEnd < len(lines) {
					for blankEnd < len(lines) && blankEnd < end+1 && strings.TrimSpace(lines[blankEnd]) == "" {
						blankEnd++
					}
				}
			} else {
				// Collapse multiple blank lines to exactly one
				if blankEnd > blankStart+1 {
					lines = append(lines[:blankStart+1], lines[blankEnd:]...)
					// Adjust end boundary after deletion
					end -= (blankEnd - (blankStart + 1))
				}
			}
			// Insert the new list item right after the single blank line
			insertion := blankStart + 1
			lines = append(lines[:insertion], append([]string{fmt.Sprintf("- %s\n", bulletPoint)}, lines[insertion:]...)...)
		}
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// constructFolder validates and processes folder paths, applies placeholders, and adjusts dates based on input parameters.
func constructFolder() (string, error) {
	if folder == "" {
		return "", errors.New("-folder must be non empty")
	}
	folder, err := obsidianutils.ApplyDirectoryPlaceHolder(folder)
	if err != nil {
		return "", err
	}
	if dailyFolder == "" {
		return "", errors.New("-daily-folder must be non empty")
	}
	if forDate == "" {
		forDate = time.Now().Format(time.DateOnly)
	}
	if strings.HasPrefix(forDate, "-") {
		relativeString := strings.TrimPrefix(forDate, "-")
		offset, err := strconv.Atoi(relativeString)
		if err != nil {
			return "", err
		}
		forDate = time.Now().AddDate(0, 0, offset*-1).Format(time.DateOnly)
	}
	if strings.HasPrefix(forDate, "+") {
		relativeString := strings.TrimPrefix(forDate, "+")
		offset, err := strconv.Atoi(relativeString)
		if err != nil {
			return "", err
		}
		forDate = time.Now().AddDate(0, 0, offset).Format(time.DateOnly)
	}

	folder = path.Join(folder, dailyFolder)

	return folder, nil
}
