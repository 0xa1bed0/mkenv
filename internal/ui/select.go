package ui

import (
	"fmt"
	"os"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
)

// SelectOption is the interface for items that can be used in SelectOne/SelectMany.
type SelectOption interface {
	OptionLabel() string // what user sees
	OptionID() string    // stable identifier for logs/logic
}

// ToSelectOptions converts a slice of any SelectOption implementation
// (including pointer types like []*ContainerInfo) into []SelectOption.
func ToSelectOptions[T SelectOption](items []T) []SelectOption {
	out := make([]SelectOption, len(items))
	for i := range items {
		out[i] = items[i]
	}
	return out
}

func (l *Logger) finalizeTailForPrompt() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tail != nil && !l.tail.closed {
		// This will clear live box and print static box once.
		l.finalizeTailLocked()
	}
}

func formatSelectOptionsForLog(options []SelectOption) string {
	var parts []string
	for _, opt := range options {
		parts = append(parts, fmt.Sprintf("%s(%s)", opt.OptionID(), opt.OptionLabel()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

type confirmOption struct {
	yes bool
}

func (co *confirmOption) OptionLabel() string {
	if co.yes {
		return "yes"
	}
	return "no"
}

func (co *confirmOption) OptionID() string {
	if co.yes {
		return "yes"
	}
	return "no"
}

func (l *Logger) Confirm(text string) (bool, error) {
	yes := &confirmOption{yes: true}
	no := &confirmOption{yes: false}

	answer, err := l.SelectOne(text, []SelectOption{yes, no})
	if err != nil {
		return false, err
	}

	return (answer.OptionID() == "yes"), nil
}

// SelectOne asks the user to choose one option with an arrow-key menu.
// It logs the prompt and the answer (ID + label) to the full log.
func (l *Logger) SelectOne(label string, options []SelectOption) (value SelectOption, err error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("SelectOne: no options provided")
	}

	// Ensure no live tail box is on screen.
	l.finalizeTailForPrompt()

	l.Spacer()
	l.InfoSilent("PROMPT: %s (options: %s)", label, formatSelectOptionsForLog(options))

	display := make([]string, len(options))
	for i, opt := range options {
		display[i] = opt.OptionLabel()
	}

	var chosenLabel string

	prompt := &survey.Select{
		Message: label,
		Options: display,
	}

	err = survey.AskOne(
		prompt,
		&chosenLabel,
		survey.WithStdio(os.Stdin, os.Stdout, os.Stderr),
	)
	if err != nil {
		return nil, err
	}

	// Map chosen label back to index.
	for _, opt := range options {
		if opt.OptionLabel() == chosenLabel {
			value = opt
			l.InfoSilent("ANSWER: id=%s label=%s", value.OptionID(), value.OptionLabel())
			return value, nil
		}
	}

	// Shouldn't happen, but be defensive.
	l.Error("PROMPT ERROR: chosen label %q not found in options", chosenLabel)
	return nil, fmt.Errorf("chosen label not found")
}

// SelectMany asks the user to choose multiple options using an arrow-key multi-select
// (space to toggle, enter to confirm). It logs the prompt and selected IDs/labels.
func (l *Logger) SelectMany(label string, options []SelectOption) (values []SelectOption, err error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("SelectMany: no options provided")
	}

	l.finalizeTailForPrompt()

	l.Spacer()
	l.InfoSilent("PROMPT: %s (multi-select; options: %s)", label, formatSelectOptionsForLog(options))

	display := make([]string, len(options))
	for i, opt := range options {
		display[i] = opt.OptionLabel()
	}

	var chosenLabels []string

	prompt := &survey.MultiSelect{
		Message: label,
		Options: display,
	}

	err = survey.AskOne(
		prompt,
		&chosenLabels,
		survey.WithStdio(os.Stdin, os.Stdout, os.Stderr),
	)
	if err != nil {
		l.Error("PROMPT FAILED: %v", err)
		return nil, err
	}

	if len(chosenLabels) == 0 {
		l.InfoSilent("ANSWER: []")
		return nil, nil
	}

	// Map labels back to indices/options.
	labelSet := make(map[string]struct{}, len(chosenLabels))
	for _, lab := range chosenLabels {
		labelSet[lab] = struct{}{}
	}

	for _, opt := range options {
		if _, ok := labelSet[opt.OptionLabel()]; ok {
			values = append(values, opt)
		}
	}

	// Log answer.
	var answerParts []string
	for _, v := range values {
		answerParts = append(answerParts, fmt.Sprintf("%s(%s)", v.OptionID(), v.OptionLabel()))
	}
	l.InfoSilent("ANSWER: %s", "["+strings.Join(answerParts, ", ")+"]")

	return values, nil
}
