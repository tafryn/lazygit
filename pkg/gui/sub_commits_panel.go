package gui

import (
	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// list panel functions

func (gui *Gui) getSelectedSubCommit() *models.Commit {
	selectedLine := gui.State.Panels.SubCommits.SelectedLineIdx
	commits := gui.State.SubCommits
	if selectedLine == -1 || len(commits) == 0 {
		return nil
	}

	return commits[selectedLine]
}

func (gui *Gui) handleSubCommitSelect() error {
	commit := gui.getSelectedSubCommit()
	var task updateTask
	if commit == nil {
		task = NewRenderStringTask("No commits")
	} else {
		cmd := gui.OSCommand.ExecutableFromString(
			gui.GitCommand.ShowCmdStr(commit.Sha, gui.State.Modes.Filtering.GetPath()),
		)

		task = NewRunPtyTask(cmd)
	}

	return gui.refreshMainViews(refreshMainOpts{
		main: &viewUpdateOpts{
			title: "Commit",
			task:  task,
		},
	})
}

func (gui *Gui) handleCheckoutSubCommit() error {
	commit := gui.getSelectedSubCommit()
	if commit == nil {
		return nil
	}

	err := gui.ask(askOpts{
		title:  gui.Tr.LcCheckoutCommit,
		prompt: gui.Tr.SureCheckoutThisCommit,
		handleConfirm: func() error {
			return gui.handleCheckoutRef(commit.Sha, handleCheckoutRefOptions{})
		},
	})
	if err != nil {
		return err
	}

	gui.State.Panels.SubCommits.SelectedLineIdx = 0

	return nil
}

func (gui *Gui) handleCreateSubCommitResetMenu() error {
	commit := gui.getSelectedSubCommit()

	return gui.createResetMenu(commit.Sha)
}

func (gui *Gui) handleViewSubCommitFiles() error {
	commit := gui.getSelectedSubCommit()
	if commit == nil {
		return nil
	}

	return gui.switchToCommitFilesContext(commit.Sha, false, gui.State.Contexts.SubCommits, "branches")
}

func (gui *Gui) switchToSubCommitsContext(refName string) error {
	// need to populate my sub commits
	builder := commands.NewCommitListBuilder(gui.Log, gui.GitCommand, gui.OSCommand, gui.Tr)

	commits, err := builder.GetCommits(
		commands.GetCommitsOptions{
			Limit:                gui.State.Panels.Commits.LimitCommits,
			FilterPath:           gui.State.Modes.Filtering.GetPath(),
			IncludeRebaseCommits: false,
			RefName:              refName,
		},
	)
	if err != nil {
		return err
	}

	gui.State.SubCommits = commits
	gui.State.Panels.SubCommits.refName = refName
	gui.State.Panels.SubCommits.SelectedLineIdx = 0
	gui.State.Contexts.SubCommits.SetParentContext(gui.currentSideListContext())

	return gui.pushContext(gui.State.Contexts.SubCommits)
}

func (gui *Gui) handleSwitchToSubCommits() error {
	currentContext := gui.currentSideListContext()
	if currentContext == nil {
		return nil
	}

	return gui.switchToSubCommitsContext(currentContext.GetSelectedItemId())
}
