// Package config handles kanban board configuration.
package config

const (
	// DefaultDir is the default kanban directory name.
	DefaultDir = "kanban"
	// DefaultTasksDir is the default tasks subdirectory name.
	DefaultTasksDir = "tasks"
	// DefaultStatus is the default status for new tasks.
	DefaultStatus = "backlog"
	// DefaultPriority is the default priority for new tasks.
	DefaultPriority = "medium"
	// DefaultClass is the default class of service for new tasks.
	DefaultClass = "standard"
	// DefaultClaimTimeout is the default claim expiration as a duration string.
	DefaultClaimTimeout = "1h"
	// DefaultTitleLines is the default number of title lines in TUI cards.
	DefaultTitleLines = 1

	// ConfigFileName is the name of the config file within the kanban directory.
	ConfigFileName = "config.yml"

	// CurrentVersion is the current config schema version.
	CurrentVersion = 4
)

// Default slice values for a new board (slices cannot be const).
var (
	DefaultStatuses = []string{
		"backlog",
		"todo",
		"in-progress",
		"review",
		"done",
	}

	DefaultPriorities = []string{
		"low",
		"medium",
		"high",
		"critical",
	}

	// DefaultClasses defines the default classes of service.
	DefaultClasses = []ClassConfig{
		{Name: "expedite", WIPLimit: 1, BypassColumnWIP: true},
		{Name: "fixed-date"},
		{Name: "standard"},
		{Name: "intangible"},
	}
)
