# Daily Note Creator (daily)

This utility creates daily notes in Obsidian with a specific template.

## Description

The Daily Note Creator utility allows you to create daily notes in your Obsidian vault. It creates a markdown file with
a predefined template that includes links to previous and next day notes, as well as various sections for tracking
activities, health metrics, tasks, and more. The notes are organized in a year/month directory structure.

## Flags

| Flag             | Description                                                      | Default             |
|------------------|------------------------------------------------------------------|---------------------|
| `-folder`        | Base path to Obsidian vault                                      | (required)          |
| `-daily-folder`  | Where to store the daily note inside the vault                   | (required)          |
| `-template-file` | Path to template file                                            | (embedded template) |
| `-print-config`  | Print configuration                                              | `false`             |
| `-overwrite`     | Overwrite existing file                                          | `false`             |
| `-for-date`      | Date for which to create the daily note (yyyy-MM-dd or +-offset) | Current date        |
| `-until`         | Create daily notes for every day from today until this date (yyyy-MM-dd or +N offset), inclusive. Mutually exclusive with `-for-date` | (none) |

## Usage

```bash
daily -folder /path/to/vault -daily-folder "Daily Notes"
```

This will create a daily note for the current date.

For a specific date:

```bash
daily -folder /path/to/vault -daily-folder "Daily Notes" -for-date 2023-09-15
```

For yesterday:

```bash
daily -folder /path/to/vault -daily-folder "Daily Notes" -for-date -1
```

To create daily notes for the rest of the week, starting today:

```bash
daily -folder /path/to/vault -daily-folder "Daily Notes" -until +6
```

To create daily notes up to and including a specific future date:

```bash
daily -folder /path/to/vault -daily-folder "Daily Notes" -until 2023-09-30
```

To use a custom template:

```bash
daily -folder /path/to/vault -daily-folder "Daily Notes" -template-file /path/to/template.md
```

## Template

The default template includes:

- Frontmatter with various health and activity tracking fields
- Links to previous and next day notes
- A table of contents
- Sections for meetings, birthdays, work, health, food & beverages
- Task tracking sections
- Dataview queries to display meetings, birthdays, tasks, and new/changed items

You can customize the template by providing your own template file with the `-template-file` flag.