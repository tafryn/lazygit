package commands

import (
	"os"
	"strconv"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/utils"
)

func (c *GitCommand) ConfiguredPager() string {
	if os.Getenv("GIT_PAGER") != "" {
		return os.Getenv("GIT_PAGER")
	}
	if os.Getenv("PAGER") != "" {
		return os.Getenv("PAGER")
	}
	output, err := c.RunCommandWithOutput("git config --get-all core.pager")
	if err != nil {
		return ""
	}
	trimmedOutput := strings.TrimSpace(output)
	return strings.Split(trimmedOutput, "\n")[0]
}

func (c *GitCommand) GetPager(width int) string {
	useConfig := c.Config.GetUserConfig().Git.Paging.UseConfig
	if useConfig {
		pager := c.ConfiguredPager()
		return strings.Split(pager, "| less")[0]
	}

	templateValues := map[string]string{
		"columnWidth": strconv.Itoa(width/2 - 6),
	}

	pagerTemplate := c.Config.GetUserConfig().Git.Paging.Pager
	return utils.ResolvePlaceholderString(pagerTemplate, templateValues)
}

func (c *GitCommand) colorArg() string {
	return c.Config.GetUserConfig().Git.Paging.ColorArg
}

func (c *GitCommand) GetConfigValue(key string) string {
	output, _ := c.getGitConfigValue(key)
	return output
}
