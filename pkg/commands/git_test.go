package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/go-errors/errors"
	gogit "github.com/jesseduffield/go-git/v5"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/i18n"
	"github.com/jesseduffield/lazygit/pkg/secureexec"
	"github.com/jesseduffield/lazygit/pkg/test"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/stretchr/testify/assert"
)

type fileInfoMock struct {
	name        string
	size        int64
	fileMode    os.FileMode
	fileModTime time.Time
	isDir       bool
	sys         interface{}
}

// Name is a function.
func (f fileInfoMock) Name() string {
	return f.name
}

// Size is a function.
func (f fileInfoMock) Size() int64 {
	return f.size
}

// Mode is a function.
func (f fileInfoMock) Mode() os.FileMode {
	return f.fileMode
}

// ModTime is a function.
func (f fileInfoMock) ModTime() time.Time {
	return f.fileModTime
}

// IsDir is a function.
func (f fileInfoMock) IsDir() bool {
	return f.isDir
}

// Sys is a function.
func (f fileInfoMock) Sys() interface{} {
	return f.sys
}

// TestNavigateToRepoRootDirectory is a function.
func TestNavigateToRepoRootDirectory(t *testing.T) {
	type scenario struct {
		testName string
		stat     func(string) (os.FileInfo, error)
		chdir    func(string) error
		test     func(error)
	}

	scenarios := []scenario{
		{
			"Navigate to git repository",
			func(string) (os.FileInfo, error) {
				return fileInfoMock{isDir: true}, nil
			},
			func(string) error {
				return nil
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"An error occurred when getting path informations",
			func(string) (os.FileInfo, error) {
				return nil, fmt.Errorf("An error occurred")
			},
			func(string) error {
				return nil
			},
			func(err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "An error occurred")
			},
		},
		{
			"An error occurred when trying to move one path backward",
			func(string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			func(string) error {
				return fmt.Errorf("An error occurred")
			},
			func(err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "An error occurred")
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.test(navigateToRepoRootDirectory(s.stat, s.chdir))
		})
	}
}

// TestSetupRepository is a function.
func TestSetupRepository(t *testing.T) {
	type scenario struct {
		testName          string
		openGitRepository func(string) (*gogit.Repository, error)
		errorStr          string
		test              func(*gogit.Repository, error)
	}

	scenarios := []scenario{
		{
			"A gitconfig parsing error occurred",
			func(string) (*gogit.Repository, error) {
				return nil, fmt.Errorf(`unquoted '\' must be followed by new line`)
			},
			"error translated",
			func(r *gogit.Repository, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "error translated")
			},
		},
		{
			"A gogit error occurred",
			func(string) (*gogit.Repository, error) {
				return nil, fmt.Errorf("Error from inside gogit")
			},
			"",
			func(r *gogit.Repository, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "Error from inside gogit")
			},
		},
		{
			"Setup done properly",
			func(string) (*gogit.Repository, error) {
				assert.NoError(t, os.RemoveAll("/tmp/lazygit-test"))
				r, err := gogit.PlainInit("/tmp/lazygit-test", false)
				assert.NoError(t, err)
				return r, nil
			},
			"",
			func(r *gogit.Repository, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, r)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.test(setupRepository(s.openGitRepository, s.errorStr))
		})
	}
}

// TestNewGitCommand is a function.
func TestNewGitCommand(t *testing.T) {
	actual, err := os.Getwd()
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, os.Chdir(actual))
	}()

	type scenario struct {
		testName string
		setup    func()
		test     func(*GitCommand, error)
	}

	scenarios := []scenario{
		{
			"An error occurred, folder doesn't contains a git repository",
			func() {
				assert.NoError(t, os.Chdir("/tmp"))
			},
			func(gitCmd *GitCommand, err error) {
				assert.Error(t, err)
				assert.Regexp(t, `Must open lazygit in a git repository`, err.Error())
			},
		},
		{
			"New GitCommand object created",
			func() {
				assert.NoError(t, os.RemoveAll("/tmp/lazygit-test"))
				_, err := gogit.PlainInit("/tmp/lazygit-test", false)
				assert.NoError(t, err)
				assert.NoError(t, os.Chdir("/tmp/lazygit-test"))
			},
			func(gitCmd *GitCommand, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.setup()
			s.test(NewGitCommand(utils.NewDummyLog(), oscommands.NewDummyOSCommand(), i18n.NewTranslationSet(utils.NewDummyLog()), config.NewDummyAppConfig()))
		})
	}
}

// TestGitCommandGetStashEntries is a function.
func TestGitCommandGetStashEntries(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func([]*models.StashEntry)
	}

	scenarios := []scenario{
		{
			"No stash entries found",
			func(string, ...string) *exec.Cmd {
				return secureexec.Command("echo")
			},
			func(entries []*models.StashEntry) {
				assert.Len(t, entries, 0)
			},
		},
		{
			"Several stash entries found",
			func(string, ...string) *exec.Cmd {
				return secureexec.Command("echo", "WIP on add-pkg-commands-test: 55c6af2 increase parallel build\nWIP on master: bb86a3f update github template")
			},
			func(entries []*models.StashEntry) {
				expected := []*models.StashEntry{
					{
						Index: 0,
						Name:  "WIP on add-pkg-commands-test: 55c6af2 increase parallel build",
					},
					{
						Index: 1,
						Name:  "WIP on master: bb86a3f update github template",
					},
				}

				assert.Len(t, entries, 2)
				assert.EqualValues(t, expected, entries)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command

			s.test(gitCmd.GetStashEntries(""))
		})
	}
}

// TestGitCommandGetStatusFiles is a function.
func TestGitCommandGetStatusFiles(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func([]*models.File)
	}

	scenarios := []scenario{
		{
			"No files found",
			func(cmd string, args ...string) *exec.Cmd {
				return secureexec.Command("echo")
			},
			func(files []*models.File) {
				assert.Len(t, files, 0)
			},
		},
		{
			"Several files found",
			func(cmd string, args ...string) *exec.Cmd {
				return secureexec.Command(
					"echo",
					"MM file1.txt\nA  file3.txt\nAM file2.txt\n?? file4.txt\nUU file5.txt",
				)
			},
			func(files []*models.File) {
				assert.Len(t, files, 5)

				expected := []*models.File{
					{
						Name:                    "file1.txt",
						HasStagedChanges:        true,
						HasUnstagedChanges:      true,
						Tracked:                 true,
						Added:                   false,
						Deleted:                 false,
						HasMergeConflicts:       false,
						HasInlineMergeConflicts: false,
						DisplayString:           "MM file1.txt",
						Type:                    "other",
						ShortStatus:             "MM",
					},
					{
						Name:                    "file3.txt",
						HasStagedChanges:        true,
						HasUnstagedChanges:      false,
						Tracked:                 false,
						Added:                   true,
						Deleted:                 false,
						HasMergeConflicts:       false,
						HasInlineMergeConflicts: false,
						DisplayString:           "A  file3.txt",
						Type:                    "other",
						ShortStatus:             "A ",
					},
					{
						Name:                    "file2.txt",
						HasStagedChanges:        true,
						HasUnstagedChanges:      true,
						Tracked:                 false,
						Added:                   true,
						Deleted:                 false,
						HasMergeConflicts:       false,
						HasInlineMergeConflicts: false,
						DisplayString:           "AM file2.txt",
						Type:                    "other",
						ShortStatus:             "AM",
					},
					{
						Name:                    "file4.txt",
						HasStagedChanges:        false,
						HasUnstagedChanges:      true,
						Tracked:                 false,
						Added:                   true,
						Deleted:                 false,
						HasMergeConflicts:       false,
						HasInlineMergeConflicts: false,
						DisplayString:           "?? file4.txt",
						Type:                    "other",
						ShortStatus:             "??",
					},
					{
						Name:                    "file5.txt",
						HasStagedChanges:        false,
						HasUnstagedChanges:      true,
						Tracked:                 true,
						Added:                   false,
						Deleted:                 false,
						HasMergeConflicts:       true,
						HasInlineMergeConflicts: true,
						DisplayString:           "UU file5.txt",
						Type:                    "other",
						ShortStatus:             "UU",
					},
				}

				assert.EqualValues(t, expected, files)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command

			s.test(gitCmd.GetStatusFiles(GetStatusFileOptions{}))
		})
	}
}

// TestGitCommandStashDo is a function.
func TestGitCommandStashDo(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"stash", "drop", "stash@{1}"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.StashDo(1, "drop"))
}

// TestGitCommandStashSave is a function.
func TestGitCommandStashSave(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"stash", "save", "A stash message"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.StashSave("A stash message"))
}

// TestGitCommandCommitAmend is a function.
func TestGitCommandCommitAmend(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"commit", "--amend", "--allow-empty"}, args)

		return secureexec.Command("echo")
	}

	_, err := gitCmd.PrepareCommitAmendSubProcess().CombinedOutput()
	assert.NoError(t, err)
}

// TestGitCommandGetCommitDifferences is a function.
func TestGitCommandGetCommitDifferences(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string, string)
	}

	scenarios := []scenario{
		{
			"Can't retrieve pushable count",
			func(string, ...string) *exec.Cmd {
				return secureexec.Command("test")
			},
			func(pushableCount string, pullableCount string) {
				assert.EqualValues(t, "?", pushableCount)
				assert.EqualValues(t, "?", pullableCount)
			},
		},
		{
			"Can't retrieve pullable count",
			func(cmd string, args ...string) *exec.Cmd {
				if args[1] == "HEAD..@{u}" {
					return secureexec.Command("test")
				}

				return secureexec.Command("echo")
			},
			func(pushableCount string, pullableCount string) {
				assert.EqualValues(t, "?", pushableCount)
				assert.EqualValues(t, "?", pullableCount)
			},
		},
		{
			"Retrieve pullable and pushable count",
			func(cmd string, args ...string) *exec.Cmd {
				if args[1] == "HEAD..@{u}" {
					return secureexec.Command("echo", "10")
				}

				return secureexec.Command("echo", "11")
			},
			func(pushableCount string, pullableCount string) {
				assert.EqualValues(t, "11", pushableCount)
				assert.EqualValues(t, "10", pullableCount)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.GetCommitDifferences("HEAD", "@{u}"))
		})
	}
}

// TestGitCommandRenameCommit is a function.
func TestGitCommandRenameCommit(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"commit", "--allow-empty", "--amend", "--only", "-m", "test"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.RenameCommit("test"))
}

// TestGitCommandResetToCommit is a function.
func TestGitCommandResetToCommit(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"reset", "--hard", "78976bc"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.ResetToCommit("78976bc", "hard", oscommands.RunCommandOptions{}))
}

// TestGitCommandNewBranch is a function.
func TestGitCommandNewBranch(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"checkout", "-b", "test", "master"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.NewBranch("test", "master"))
}

// TestGitCommandDeleteBranch is a function.
func TestGitCommandDeleteBranch(t *testing.T) {
	type scenario struct {
		testName string
		branch   string
		force    bool
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"Delete a branch",
			"test",
			false,
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"branch", "-d", "test"}, args)

				return secureexec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"Force delete a branch",
			"test",
			true,
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"branch", "-D", "test"}, args)

				return secureexec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.DeleteBranch(s.branch, s.force))
		})
	}
}

// TestGitCommandMerge is a function.
func TestGitCommandMerge(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"merge", "--no-edit", "test"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.Merge("test", MergeOpts{}))
}

// TestGitCommandUsingGpg is a function.
func TestGitCommandUsingGpg(t *testing.T) {
	type scenario struct {
		testName          string
		getGitConfigValue func(string) (string, error)
		test              func(bool)
	}

	scenarios := []scenario{
		{
			"Option global and local config commit.gpgsign is not set",
			func(string) (string, error) { return "", nil },
			func(gpgEnabled bool) {
				assert.False(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is true",
			func(string) (string, error) {
				return "True", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is on",
			func(string) (string, error) {
				return "ON", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is yes",
			func(string) (string, error) {
				return "YeS", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
		{
			"Option commit.gpgsign is 1",
			func(string) (string, error) {
				return "1", nil
			},
			func(gpgEnabled bool) {
				assert.True(t, gpgEnabled)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.getGitConfigValue = s.getGitConfigValue
			s.test(gitCmd.usingGpg())
		})
	}
}

// TestGitCommandCommit is a function.
func TestGitCommandCommit(t *testing.T) {
	type scenario struct {
		testName          string
		command           func(string, ...string) *exec.Cmd
		getGitConfigValue func(string) (string, error)
		test              func(*exec.Cmd, error)
		flags             string
	}

	scenarios := []scenario{
		{
			"Commit using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "bash", cmd)
				assert.EqualValues(t, []string{"-c", "git commit  -m \"test\""}, args)

				return secureexec.Command("echo")
			},
			func(string) (string, error) {
				return "true", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NotNil(t, cmd)
				assert.Nil(t, err)
			},
			"",
		},
		{
			"Commit without using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "-m", "test"}, args)

				return secureexec.Command("echo")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Nil(t, err)
			},
			"",
		},
		{
			"Commit with --no-verify flag",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--no-verify", "-m", "test"}, args)

				return secureexec.Command("echo")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Nil(t, err)
			},
			"--no-verify",
		},
		{
			"Commit without using gpg with an error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "-m", "test"}, args)

				return secureexec.Command("test")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Error(t, err)
			},
			"",
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.getGitConfigValue = s.getGitConfigValue
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.Commit("test", s.flags))
		})
	}
}

// TestGitCommandAmendHead is a function.
func TestGitCommandAmendHead(t *testing.T) {
	type scenario struct {
		testName          string
		command           func(string, ...string) *exec.Cmd
		getGitConfigValue func(string) (string, error)
		test              func(*exec.Cmd, error)
	}

	scenarios := []scenario{
		{
			"Amend commit using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "bash", cmd)
				assert.EqualValues(t, []string{"-c", "git commit --amend --no-edit --allow-empty"}, args)

				return secureexec.Command("echo")
			},
			func(string) (string, error) {
				return "true", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NotNil(t, cmd)
				assert.Nil(t, err)
			},
		},
		{
			"Amend commit without using gpg",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--amend", "--no-edit", "--allow-empty"}, args)

				return secureexec.Command("echo")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Nil(t, err)
			},
		},
		{
			"Amend commit without using gpg with an error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"commit", "--amend", "--no-edit", "--allow-empty"}, args)

				return secureexec.Command("test")
			},
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.Nil(t, cmd)
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.getGitConfigValue = s.getGitConfigValue
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.AmendHead())
		})
	}
}

// TestGitCommandPush is a function.
func TestGitCommandPush(t *testing.T) {
	type scenario struct {
		testName          string
		getGitConfigValue func(string) (string, error)
		command           func(string, ...string) *exec.Cmd
		forcePush         bool
		test              func(error)
	}

	scenarios := []scenario{
		{
			"Push with force disabled, follow-tags on",
			func(string) (string, error) {
				return "", nil
			},
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push", "--follow-tags"}, args)

				return secureexec.Command("echo")
			},
			false,
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"Push with force enabled, follow-tags on",
			func(string) (string, error) {
				return "", nil
			},
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push", "--follow-tags", "--force-with-lease"}, args)

				return secureexec.Command("echo")
			},
			true,
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"Push with force disabled, follow-tags off",
			func(string) (string, error) {
				return "false", nil
			},
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push"}, args)

				return secureexec.Command("echo")
			},
			false,
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"Push with an error occurring, follow-tags on",
			func(string) (string, error) {
				return "", nil
			},
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"push", "--follow-tags"}, args)
				return secureexec.Command("test")
			},
			false,
			func(err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			gitCmd.getGitConfigValue = s.getGitConfigValue
			err := gitCmd.Push("test", s.forcePush, "", "", func(passOrUname string) string {
				return "\n"
			})
			s.test(err)
		})
	}
}

// TestGitCommandCatFile tests emitting a file using commands, where commands vary by OS.
func TestGitCommandCatFile(t *testing.T) {
	var osCmd string
	switch os := runtime.GOOS; os {
	case "windows":
		osCmd = "type"
	default:
		osCmd = "cat"
	}
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, osCmd, cmd)
		assert.EqualValues(t, []string{"test.txt"}, args)

		return secureexec.Command("echo", "-n", "test")
	}

	o, err := gitCmd.CatFile("test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "test", o)
}

// TestGitCommandStageFile is a function.
func TestGitCommandStageFile(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"add", "--", "test.txt"}, args)

		return secureexec.Command("echo")
	}

	assert.NoError(t, gitCmd.StageFile("test.txt"))
}

// TestGitCommandUnstageFile is a function.
func TestGitCommandUnstageFile(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
		reset    bool
	}

	scenarios := []scenario{
		{
			"Remove an untracked file from staging",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"rm", "--cached", "--force", "--", "test.txt"}, args)

				return secureexec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			false,
		},
		{
			"Remove a tracked file from staging",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"reset", "HEAD", "--", "test.txt"}, args)

				return secureexec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.UnStageFile([]string{"test.txt"}, s.reset))
		})
	}
}

// TestGitCommandDiscardAllFileChanges is a function.
// these tests don't cover everything, in part because we already have an integration
// test which does cover everything. I don't want to unnecessarily assert on the 'how'
// when the 'what' is what matters
func TestGitCommandDiscardAllFileChanges(t *testing.T) {
	type scenario struct {
		testName   string
		command    func() (func(string, ...string) *exec.Cmd, *[][]string)
		test       func(*[][]string, error)
		file       *models.File
		removeFile func(string) error
	}

	scenarios := []scenario{
		{
			"An error occurred when resetting",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("test")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Error(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
				})
			},
			&models.File{
				Name:             "test",
				HasStagedChanges: true,
			},
			func(string) error {
				return nil
			},
		},
		{
			"An error occurred when removing file",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("test")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Error(t, err)
				assert.EqualError(t, err, "an error occurred when removing file")
				assert.Len(t, *cmdsCalled, 0)
			},
			&models.File{
				Name:    "test",
				Tracked: false,
				Added:   true,
			},
			func(string) error {
				return fmt.Errorf("an error occurred when removing file")
			},
		},
		{
			"An error occurred with checkout",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("test")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.Error(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"checkout", "--", "test"},
				})
			},
			&models.File{
				Name:             "test",
				Tracked:          true,
				HasStagedChanges: false,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Checkout only",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"checkout", "--", "test"},
				})
			},
			&models.File{
				Name:             "test",
				Tracked:          true,
				HasStagedChanges: false,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Reset and checkout staged changes",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 2)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
					{"checkout", "--", "test"},
				})
			},
			&models.File{
				Name:             "test",
				Tracked:          true,
				HasStagedChanges: true,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Reset and checkout merge conflicts",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 2)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
					{"checkout", "--", "test"},
				})
			},
			&models.File{
				Name:              "test",
				Tracked:           true,
				HasMergeConflicts: true,
			},
			func(string) error {
				return nil
			},
		},
		{
			"Reset and remove",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 1)
				assert.EqualValues(t, *cmdsCalled, [][]string{
					{"reset", "--", "test"},
				})
			},
			&models.File{
				Name:             "test",
				Tracked:          false,
				Added:            true,
				HasStagedChanges: true,
			},
			func(filename string) error {
				assert.Equal(t, "test", filename)
				return nil
			},
		},
		{
			"Remove only",
			func() (func(string, ...string) *exec.Cmd, *[][]string) {
				cmdsCalled := [][]string{}
				return func(cmd string, args ...string) *exec.Cmd {
					cmdsCalled = append(cmdsCalled, args)

					return secureexec.Command("echo")
				}, &cmdsCalled
			},
			func(cmdsCalled *[][]string, err error) {
				assert.NoError(t, err)
				assert.Len(t, *cmdsCalled, 0)
			},
			&models.File{
				Name:             "test",
				Tracked:          false,
				Added:            true,
				HasStagedChanges: false,
			},
			func(filename string) error {
				assert.Equal(t, "test", filename)
				return nil
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			var cmdsCalled *[][]string
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command, cmdsCalled = s.command()
			gitCmd.removeFile = s.removeFile
			s.test(cmdsCalled, gitCmd.DiscardAllFileChanges(s.file))
		})
	}
}

// TestGitCommandCheckout is a function.
func TestGitCommandCheckout(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
		force    bool
	}

	scenarios := []scenario{
		{
			"Checkout",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"checkout", "test"}, args)

				return secureexec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			false,
		},
		{
			"Checkout forced",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"checkout", "--force", "test"}, args)

				return secureexec.Command("echo")
			},
			func(err error) {
				assert.NoError(t, err)
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.Checkout("test", CheckoutOptions{Force: s.force}))
		})
	}
}

// TestGitCommandGetBranchGraph is a function.
func TestGitCommandGetBranchGraph(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"log", "--graph", "--color=always", "--abbrev-commit", "--decorate", "--date=relative", "--pretty=medium", "test", "--"}, args)
		return secureexec.Command("echo")
	}
	_, err := gitCmd.GetBranchGraph("test")
	assert.NoError(t, err)
}

func TestGitCommandGetAllBranchGraph(t *testing.T) {
	gitCmd := NewDummyGitCommand()
	gitCmd.OSCommand.Command = func(cmd string, args ...string) *exec.Cmd {
		assert.EqualValues(t, "git", cmd)
		assert.EqualValues(t, []string{"log", "--graph", "--all", "--color=always", "--abbrev-commit", "--decorate", "--date=relative", "--pretty=medium"}, args)
		return secureexec.Command("echo")
	}
	cmdStr := gitCmd.Config.GetUserConfig().Git.AllBranchesLogCmd
	_, err := gitCmd.OSCommand.RunCommandWithOutput(cmdStr)
	assert.NoError(t, err)
}

// TestGitCommandDiff is a function.
func TestGitCommandDiff(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		file     *models.File
		plain    bool
		cached   bool
	}

	scenarios := []scenario{
		{
			"Default case",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--submodule", "--no-ext-diff", "--color=always", "--", "test.txt"}, args)

				return secureexec.Command("echo")
			},
			&models.File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          true,
			},
			false,
			false,
		},
		{
			"cached",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--submodule", "--no-ext-diff", "--color=always", "--cached", "--", "test.txt"}, args)

				return secureexec.Command("echo")
			},
			&models.File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          true,
			},
			false,
			true,
		},
		{
			"plain",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--submodule", "--no-ext-diff", "--color=never", "--", "test.txt"}, args)

				return secureexec.Command("echo")
			},
			&models.File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          true,
			},
			true,
			false,
		},
		{
			"File not tracked and file has no staged changes",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)
				assert.EqualValues(t, []string{"diff", "--submodule", "--no-ext-diff", "--color=always", "--no-index", "--", "/dev/null", "test.txt"}, args)

				return secureexec.Command("echo")
			},
			&models.File{
				Name:             "test.txt",
				HasStagedChanges: false,
				Tracked:          false,
			},
			false,
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			gitCmd.WorktreeFileDiff(s.file, s.plain, s.cached)
		})
	}
}

// TestGitCommandCurrentBranchName is a function.
func TestGitCommandCurrentBranchName(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(string, string, error)
	}

	scenarios := []scenario{
		{
			"says we are on the master branch if we are",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				return secureexec.Command("echo", "master")
			},
			func(name string, displayname string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "master", name)
				assert.EqualValues(t, "master", displayname)
			},
		},
		{
			"falls back to git `git branch --contains` if symbolic-ref fails",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return secureexec.Command("test")
				case "branch":
					assert.EqualValues(t, []string{"branch", "--contains"}, args)
					return secureexec.Command("echo", "* master")
				}

				return nil
			},
			func(name string, displayname string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "master", name)
				assert.EqualValues(t, "master", displayname)
			},
		},
		{
			"handles a detached head",
			func(cmd string, args ...string) *exec.Cmd {
				assert.EqualValues(t, "git", cmd)

				switch args[0] {
				case "symbolic-ref":
					assert.EqualValues(t, []string{"symbolic-ref", "--short", "HEAD"}, args)
					return secureexec.Command("test")
				case "branch":
					assert.EqualValues(t, []string{"branch", "--contains"}, args)
					return secureexec.Command("echo", "* (HEAD detached at 123abcd)")
				}

				return nil
			},
			func(name string, displayname string, err error) {
				assert.NoError(t, err)
				assert.EqualValues(t, "123abcd", name)
				assert.EqualValues(t, "(HEAD detached at 123abcd)", displayname)
			},
		},
		{
			"bubbles up error if there is one",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				return secureexec.Command("test")
			},
			func(name string, displayname string, err error) {
				assert.Error(t, err)
				assert.EqualValues(t, "", name)
				assert.EqualValues(t, "", displayname)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.CurrentBranchName())
		})
	}
}

func TestGitCommandApplyPatch(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"valid case",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				assert.EqualValues(t, []string{"apply", "--cached"}, args[0:2])
				filename := args[2]
				content, err := ioutil.ReadFile(filename)
				assert.NoError(t, err)

				assert.Equal(t, "test", string(content))

				return secureexec.Command("echo", "done")
			},
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"command returns error",
			func(cmd string, args ...string) *exec.Cmd {
				assert.Equal(t, "git", cmd)
				assert.EqualValues(t, []string{"apply", "--cached"}, args[0:2])
				filename := args[2]
				// TODO: Ideally we want to mock out OSCommand here so that we're not
				// double handling testing it's CreateTempFile functionality,
				// but it is going to take a bit of work to make a proper mock for it
				// so I'm leaving it for another PR
				content, err := ioutil.ReadFile(filename)
				assert.NoError(t, err)

				assert.Equal(t, "test", string(content))

				return secureexec.Command("test")
			},
			func(err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd := NewDummyGitCommand()
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.ApplyPatch("test", "cached"))
		})
	}
}

// TestGitCommandRebaseBranch is a function.
func TestGitCommandRebaseBranch(t *testing.T) {
	type scenario struct {
		testName string
		arg      string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"successful rebase",
			"master",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  "git rebase --interactive --autostash --keep-empty master",
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"unsuccessful rebase",
			"master",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  "git rebase --interactive --autostash --keep-empty master",
					Replace: "test",
				},
			}),
			func(err error) {
				assert.Error(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.RebaseBranch(s.arg))
		})
	}
}

// TestGitCommandCheckoutFile is a function.
func TestGitCommandCheckoutFile(t *testing.T) {
	type scenario struct {
		testName  string
		commitSha string
		fileName  string
		command   func(string, ...string) *exec.Cmd
		test      func(error)
	}

	scenarios := []scenario{
		{
			"typical case",
			"11af912",
			"test999.txt",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  "git checkout 11af912 test999.txt",
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
		{
			"returns error if there is one",
			"11af912",
			"test999.txt",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  "git checkout 11af912 test999.txt",
					Replace: "test",
				},
			}),
			func(err error) {
				assert.Error(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.CheckoutFile(s.commitSha, s.fileName))
		})
	}
}

// TestGitCommandDiscardOldFileChanges is a function.
func TestGitCommandDiscardOldFileChanges(t *testing.T) {
	type scenario struct {
		testName          string
		getGitConfigValue func(string) (string, error)
		commits           []*models.Commit
		commitIndex       int
		fileName          string
		command           func(string, ...string) *exec.Cmd
		test              func(error)
	}

	scenarios := []scenario{
		{
			"returns error when index outside of range of commits",
			func(string) (string, error) {
				return "", nil
			},
			[]*models.Commit{},
			0,
			"test999.txt",
			nil,
			func(err error) {
				assert.Error(t, err)
			},
		},
		{
			"returns error when using gpg",
			func(string) (string, error) {
				return "true", nil
			},
			[]*models.Commit{{Name: "commit", Sha: "123456"}},
			0,
			"test999.txt",
			nil,
			func(err error) {
				assert.Error(t, err)
			},
		},
		{
			"checks out file if it already existed",
			func(string) (string, error) {
				return "", nil
			},
			[]*models.Commit{
				{Name: "commit", Sha: "123456"},
				{Name: "commit2", Sha: "abcdef"},
			},
			0,
			"test999.txt",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  "git rebase --interactive --autostash --keep-empty abcdef",
					Replace: "echo",
				},
				{
					Expect:  "git cat-file -e HEAD^:test999.txt",
					Replace: "echo",
				},
				{
					Expect:  "git checkout HEAD^ test999.txt",
					Replace: "echo",
				},
				{
					Expect:  "git commit --amend --no-edit --allow-empty",
					Replace: "echo",
				},
				{
					Expect:  "git rebase --continue",
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
		// test for when the file was created within the commit requires a refactor to support proper mocks
		// currently we'd need to mock out the os.Remove function and that's gonna introduce tech debt
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			gitCmd.getGitConfigValue = s.getGitConfigValue
			s.test(gitCmd.DiscardOldFileChanges(s.commits, s.commitIndex, s.fileName))
		})
	}
}

// TestGitCommandDiscardUnstagedFileChanges is a function.
func TestGitCommandDiscardUnstagedFileChanges(t *testing.T) {
	type scenario struct {
		testName string
		file     *models.File
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"valid case",
			&models.File{Name: "test.txt"},
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  `git checkout -- "test.txt"`,
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.DiscardUnstagedFileChanges(s.file))
		})
	}
}

// TestGitCommandDiscardAnyUnstagedFileChanges is a function.
func TestGitCommandDiscardAnyUnstagedFileChanges(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"valid case",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  `git checkout -- .`,
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.DiscardAnyUnstagedFileChanges())
		})
	}
}

// TestGitCommandRemoveUntrackedFiles is a function.
func TestGitCommandRemoveUntrackedFiles(t *testing.T) {
	type scenario struct {
		testName string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"valid case",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  `git clean -fd`,
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.RemoveUntrackedFiles())
		})
	}
}

// TestGitCommandResetHard is a function.
func TestGitCommandResetHard(t *testing.T) {
	type scenario struct {
		testName string
		ref      string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"valid case",
			"HEAD",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  `git reset --hard HEAD`,
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.ResetHard(s.ref))
		})
	}
}

// TestGitCommandCreateFixupCommit is a function.
func TestGitCommandCreateFixupCommit(t *testing.T) {
	type scenario struct {
		testName string
		sha      string
		command  func(string, ...string) *exec.Cmd
		test     func(error)
	}

	scenarios := []scenario{
		{
			"valid case",
			"12345",
			test.CreateMockCommand(t, []*test.CommandSwapper{
				{
					Expect:  `git commit --fixup=12345`,
					Replace: "echo",
				},
			}),
			func(err error) {
				assert.NoError(t, err)
			},
		},
	}

	gitCmd := NewDummyGitCommand()

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			gitCmd.OSCommand.Command = s.command
			s.test(gitCmd.CreateFixupCommit(s.sha))
		})
	}
}

// TestGitCommandSkipEditorCommand confirms that SkipEditorCommand injects
// environment variables that suppress an interactive editor
func TestGitCommandSkipEditorCommand(t *testing.T) {
	cmd := NewDummyGitCommand()

	cmd.OSCommand.SetBeforeExecuteCmd(func(cmd *exec.Cmd) {
		test.AssertContainsMatch(
			t,
			cmd.Env,
			regexp.MustCompile("^VISUAL="),
			"expected VISUAL to be set for a non-interactive external command",
		)

		test.AssertContainsMatch(
			t,
			cmd.Env,
			regexp.MustCompile("^EDITOR="),
			"expected EDITOR to be set for a non-interactive external command",
		)

		test.AssertContainsMatch(
			t,
			cmd.Env,
			regexp.MustCompile("^GIT_EDITOR="),
			"expected GIT_EDITOR to be set for a non-interactive external command",
		)

		test.AssertContainsMatch(
			t,
			cmd.Env,
			regexp.MustCompile("^LAZYGIT_CLIENT_COMMAND=EXIT_IMMEDIATELY$"),
			"expected LAZYGIT_CLIENT_COMMAND to be set for a non-interactive external command",
		)
	})

	_ = cmd.runSkipEditorCommand("true")
}

func TestFindDotGitDir(t *testing.T) {
	type scenario struct {
		testName string
		stat     func(string) (os.FileInfo, error)
		readFile func(filename string) ([]byte, error)
		test     func(string, error)
	}

	scenarios := []scenario{
		{
			".git is a directory",
			func(dotGit string) (os.FileInfo, error) {
				assert.Equal(t, ".git", dotGit)
				return os.Stat("testdata/a_dir")
			},
			func(dotGit string) ([]byte, error) {
				assert.Fail(t, "readFile should not be called if .git is a directory")
				return nil, nil
			},
			func(gitDir string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, ".git", gitDir)
			},
		},
		{
			".git is a file",
			func(dotGit string) (os.FileInfo, error) {
				assert.Equal(t, ".git", dotGit)
				return os.Stat("testdata/a_file")
			},
			func(dotGit string) ([]byte, error) {
				assert.Equal(t, ".git", dotGit)
				return []byte("gitdir: blah\n"), nil
			},
			func(gitDir string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "blah", gitDir)
			},
		},
		{
			"os.Stat returns an error",
			func(dotGit string) (os.FileInfo, error) {
				assert.Equal(t, ".git", dotGit)
				return nil, errors.New("error")
			},
			func(dotGit string) ([]byte, error) {
				assert.Fail(t, "readFile should not be called os.Stat returns an error")
				return nil, nil
			},
			func(gitDir string, err error) {
				assert.Error(t, err)
			},
		},
		{
			"readFile returns an error",
			func(dotGit string) (os.FileInfo, error) {
				assert.Equal(t, ".git", dotGit)
				return os.Stat("testdata/a_file")
			},
			func(dotGit string) ([]byte, error) {
				return nil, errors.New("error")
			},
			func(gitDir string, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.testName, func(t *testing.T) {
			s.test(findDotGitDir(s.stat, s.readFile))
		})
	}
}

// TestEditFile is a function.
func TestEditFile(t *testing.T) {
	type scenario struct {
		filename          string
		command           func(string, ...string) *exec.Cmd
		getenv            func(string) string
		getGitConfigValue func(string) (string, error)
		test              func(*exec.Cmd, error)
	}

	scenarios := []scenario{
		{
			"test",
			func(name string, arg ...string) *exec.Cmd {
				return secureexec.Command("exit", "1")
			},
			func(env string) string {
				return ""
			},
			func(cf string) (string, error) {
				return "", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.EqualError(t, err, "No editor defined in $GIT_EDITOR, $VISUAL, $EDITOR, or git config")
			},
		},
		{
			"test",
			func(name string, arg ...string) *exec.Cmd {
				if name == "which" {
					return secureexec.Command("exit", "1")
				}

				assert.EqualValues(t, "nano", name)

				return nil
			},
			func(env string) string {
				return ""
			},
			func(cf string) (string, error) {
				return "nano", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NoError(t, err)
			},
		},
		{
			"test",
			func(name string, arg ...string) *exec.Cmd {
				if name == "which" {
					return secureexec.Command("exit", "1")
				}

				assert.EqualValues(t, "nano", name)

				return nil
			},
			func(env string) string {
				if env == "VISUAL" {
					return "nano"
				}

				return ""
			},
			func(cf string) (string, error) {
				return "", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NoError(t, err)
			},
		},
		{
			"test",
			func(name string, arg ...string) *exec.Cmd {
				if name == "which" {
					return secureexec.Command("exit", "1")
				}

				assert.EqualValues(t, "emacs", name)

				return nil
			},
			func(env string) string {
				if env == "EDITOR" {
					return "emacs"
				}

				return ""
			},
			func(cf string) (string, error) {
				return "", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NoError(t, err)
			},
		},
		{
			"test",
			func(name string, arg ...string) *exec.Cmd {
				if name == "which" {
					return secureexec.Command("echo")
				}

				assert.EqualValues(t, "vi", name)

				return nil
			},
			func(env string) string {
				return ""
			},
			func(cf string) (string, error) {
				return "", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NoError(t, err)
			},
		},
		{
			"file/with space",
			func(name string, args ...string) *exec.Cmd {
				if name == "which" {
					return secureexec.Command("echo")
				}

				assert.EqualValues(t, "vi", name)
				assert.EqualValues(t, "file/with space", args[0])

				return nil
			},
			func(env string) string {
				return ""
			},
			func(cf string) (string, error) {
				return "", nil
			},
			func(cmd *exec.Cmd, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, s := range scenarios {
		gitCmd := NewDummyGitCommand()
		gitCmd.OSCommand.Command = s.command
		gitCmd.OSCommand.Getenv = s.getenv
		gitCmd.getGitConfigValue = s.getGitConfigValue
		s.test(gitCmd.EditFile(s.filename))
	}
}
