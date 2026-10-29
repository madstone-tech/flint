package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestPickerSelectWithArrowKeys(t *testing.T) {
	m := newPickerModel([]string{"default", "dev", "staging", "production"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Move down one (to "dev") and select it.
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	fm := finalModel(t, tm)
	if fm.aborted {
		t.Fatal("expected a selection, got aborted")
	}
	if fm.choice != "dev" {
		t.Errorf("choice = %q, want %q", fm.choice, "dev")
	}
}

func TestPickerFilterThenSelect(t *testing.T) {
	m := newPickerModel([]string{"default", "dev", "staging", "production"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Enter filter mode ("/"), type "prod".
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "prod" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Wait for the filtered view to show "production" before applying.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytesContains(b, "production")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // apply filter
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // select the single match

	fm := finalModel(t, tm)
	if fm.choice != "production" {
		t.Errorf("choice = %q, want %q", fm.choice, "production")
	}
}

func bytesContains(b []byte, sub string) bool {
	return strings.Contains(string(b), sub)
}

func TestPickerAbort(t *testing.T) {
	m := newPickerModel([]string{"default", "dev"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	fm := finalModel(t, tm)
	if !fm.aborted {
		t.Errorf("expected aborted, got choice=%q", fm.choice)
	}
}

func finalModel(t *testing.T, tm *teatest.TestModel) pickerModel {
	t.Helper()
	fm, ok := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(pickerModel)
	if !ok {
		t.Fatal("final model is not a pickerModel")
	}
	return fm
}
