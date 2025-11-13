package ui

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

// Anything that can be shown in a selector
type OptionLabeler interface {
	OptionLabel() string
}

// Select one item
func SelectOne[T OptionLabeler](message string, items []T) (T, error) {
	var zero T

	if len(items) == 0 {
		return zero, fmt.Errorf("no items to select from")
	}

	options := make([]string, len(items))
	for i, it := range items {
		options[i] = it.OptionLabel()
	}

	var choice string
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}

	if err := survey.AskOne(prompt, &choice); err != nil {
		return zero, err
	}

	for i, opt := range options {
		if opt == choice {
			return items[i], nil
		}
	}

	return zero, fmt.Errorf("selection not found")
}

// Select multiple items
func SelectMany[T OptionLabeler](message string, items []T) ([]T, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	options := make([]string, len(items))
	for i, it := range items {
		options[i] = it.OptionLabel()
	}

	var choices []string
	prompt := &survey.MultiSelect{
		Message: message,
		Options: options,
	}

	if err := survey.AskOne(prompt, &choices); err != nil {
		return nil, err
	}

	selected := make([]T, 0, len(choices))
	for _, ch := range choices {
		for i, opt := range options {
			if opt == ch {
				selected = append(selected, items[i])
				break
			}
		}
	}

	return selected, nil
}

