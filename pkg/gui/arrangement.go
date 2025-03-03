package gui

import (
	"github.com/jesseduffield/lazygit/pkg/gui/boxlayout"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

func (gui *Gui) mainSectionChildren() []*boxlayout.Box {
	currentWindow := gui.currentWindow()

	// if we're not in split mode we can just show the one main panel. Likewise if
	// the main panel is focused and we're in full-screen mode
	if !gui.isMainPanelSplit() || (gui.State.ScreenMode == SCREEN_FULL && currentWindow == "main") {
		return []*boxlayout.Box{
			{
				Window: "main",
				Weight: 1,
			},
		}
	}

	main := "main"
	secondary := "secondary"
	if gui.secondaryViewFocused() {
		// when you think you've focused the secondary view, we've actually just swapped them around in the layout
		main, secondary = secondary, main
	}

	return []*boxlayout.Box{
		{
			Window: main,
			Weight: 1,
		},
		{
			Window: secondary,
			Weight: 1,
		},
	}
}

func (gui *Gui) getMidSectionWeights() (int, int) {
	currentWindow := gui.currentWindow()

	// we originally specified this as a ratio i.e. .20 would correspond to a weight of 1 against 4
	sidePanelWidthRatio := gui.Config.GetUserConfig().Gui.SidePanelWidth
	// we could make this better by creating ratios like 2:3 rather than always 1:something
	mainSectionWeight := int(1/sidePanelWidthRatio) - 1
	sideSectionWeight := 1

	if gui.splitMainPanelSideBySide() {
		mainSectionWeight = 5 // need to shrink side panel to make way for main panels if side-by-side
	}

	if currentWindow == "main" {
		if gui.State.ScreenMode == SCREEN_HALF || gui.State.ScreenMode == SCREEN_FULL {
			sideSectionWeight = 0
		}
	} else {
		if gui.State.ScreenMode == SCREEN_HALF {
			mainSectionWeight = 1
		} else if gui.State.ScreenMode == SCREEN_FULL {
			mainSectionWeight = 0
		}
	}

	return sideSectionWeight, mainSectionWeight
}

func (gui *Gui) infoSectionChildren(informationStr string, appStatus string) []*boxlayout.Box {
	if gui.State.Searching.isSearching {
		return []*boxlayout.Box{
			{
				Window: "searchPrefix",
				Size:   len(SEARCH_PREFIX),
			},
			{
				Window: "search",
				Weight: 1,
			},
		}
	}

	result := []*boxlayout.Box{}

	if len(appStatus) > 0 {
		result = append(result,
			&boxlayout.Box{
				Window: "appStatus",
				Size:   len(appStatus) + len(INFO_SECTION_PADDING),
			},
		)
	}

	result = append(result,
		[]*boxlayout.Box{
			{
				Window: "options",
				Weight: 1,
			},
			{
				Window: "information",
				// unlike appStatus, informationStr has various colors so we need to decolorise before taking the length
				Size: len(INFO_SECTION_PADDING) + len(utils.Decolorise(informationStr)),
			},
		}...,
	)

	return result
}

func (gui *Gui) splitMainPanelSideBySide() bool {
	if !gui.isMainPanelSplit() {
		return false
	}

	mainPanelSplitMode := gui.Config.GetUserConfig().Gui.MainPanelSplitMode
	width, height := gui.g.Size()

	switch mainPanelSplitMode {
	case "vertical":
		return false
	case "horizontal":
		return true
	default:
		if width < 200 && height > 30 { // 2 80 character width panels + 40 width for side panel
			return false
		} else {
			return true
		}
	}
}

func (gui *Gui) getWindowDimensions(informationStr string, appStatus string) map[string]boxlayout.Dimensions {
	width, height := gui.g.Size()

	sideSectionWeight, mainSectionWeight := gui.getMidSectionWeights()

	sidePanelsDirection := boxlayout.COLUMN
	portraitMode := width <= 84 && height > 45
	if portraitMode {
		sidePanelsDirection = boxlayout.ROW
	}

	mainPanelsDirection := boxlayout.ROW
	if gui.splitMainPanelSideBySide() {
		mainPanelsDirection = boxlayout.COLUMN
	}

	root := &boxlayout.Box{
		Direction: boxlayout.ROW,
		Children: []*boxlayout.Box{
			{
				Direction: sidePanelsDirection,
				Weight:    1,
				Children: []*boxlayout.Box{
					{
						Direction:           boxlayout.ROW,
						Weight:              sideSectionWeight,
						ConditionalChildren: gui.sidePanelChildren,
					},
					{
						Direction: mainPanelsDirection,
						Weight:    mainSectionWeight,
						Children:  gui.mainSectionChildren(),
					},
				},
			},
			{
				Direction: boxlayout.COLUMN,
				Size:      1,
				Children:  gui.infoSectionChildren(informationStr, appStatus),
			},
		},
	}

	return boxlayout.ArrangeWindows(root, 0, 0, width, height)
}

// The stash window by default only contains one line so that it's not hogging
// too much space, but if you access it it should take up some space. This is
// the default behaviour when accordian mode is NOT in effect. If it is in effect
// then when it's accessed it will have weight 2, not 1.
func (gui *Gui) getDefaultStashWindowBox() *boxlayout.Box {
	gui.State.ContextManager.RLock()
	defer gui.State.ContextManager.RUnlock()

	box := &boxlayout.Box{Window: "stash"}
	stashWindowAccessed := false
	for _, context := range gui.State.ContextManager.ContextStack {
		if context.GetWindowName() == "stash" {
			stashWindowAccessed = true
		}
	}
	// if the stash window is anywhere in our stack we should enlargen it
	if stashWindowAccessed {
		box.Weight = 1
	} else {
		box.Size = 3
	}

	return box
}

func (gui *Gui) sidePanelChildren(width int, height int) []*boxlayout.Box {
	currentWindow := gui.currentSideWindowName()

	if gui.State.ScreenMode == SCREEN_FULL || gui.State.ScreenMode == SCREEN_HALF {
		fullHeightBox := func(window string) *boxlayout.Box {
			if window == currentWindow {
				return &boxlayout.Box{
					Window: window,
					Weight: 1,
				}
			} else {
				return &boxlayout.Box{
					Window: window,
					Size:   0,
				}
			}
		}

		return []*boxlayout.Box{
			fullHeightBox("status"),
			fullHeightBox("files"),
			fullHeightBox("branches"),
			fullHeightBox("commits"),
			fullHeightBox("stash"),
		}
	} else if height >= 28 {
		accordianMode := gui.Config.GetUserConfig().Gui.ExpandFocusedSidePanel
		accordianBox := func(defaultBox *boxlayout.Box) *boxlayout.Box {
			if accordianMode && defaultBox.Window == currentWindow {
				return &boxlayout.Box{
					Window: defaultBox.Window,
					Weight: 2,
				}
			}

			return defaultBox
		}

		return []*boxlayout.Box{
			{
				Window: "status",
				Size:   3,
			},
			accordianBox(&boxlayout.Box{Window: "files", Weight: 1}),
			accordianBox(&boxlayout.Box{Window: "branches", Weight: 1}),
			accordianBox(&boxlayout.Box{Window: "commits", Weight: 1}),
			accordianBox(gui.getDefaultStashWindowBox()),
		}
	} else {
		squashedHeight := 1
		if height >= 21 {
			squashedHeight = 3
		}

		squashedSidePanelBox := func(window string) *boxlayout.Box {
			if window == currentWindow {
				return &boxlayout.Box{
					Window: window,
					Weight: 1,
				}
			} else {
				return &boxlayout.Box{
					Window: window,
					Size:   squashedHeight,
				}
			}
		}

		return []*boxlayout.Box{
			squashedSidePanelBox("status"),
			squashedSidePanelBox("files"),
			squashedSidePanelBox("branches"),
			squashedSidePanelBox("commits"),
			squashedSidePanelBox("stash"),
		}
	}
}

func (gui *Gui) currentSideWindowName() string {
	// there is always one and only one cyclable context in the context stack. We'll look from top to bottom
	gui.State.ContextManager.RLock()
	defer gui.State.ContextManager.RUnlock()

	for idx := range gui.State.ContextManager.ContextStack {
		reversedIdx := len(gui.State.ContextManager.ContextStack) - 1 - idx
		context := gui.State.ContextManager.ContextStack[reversedIdx]

		if context.GetKind() == SIDE_CONTEXT {
			return context.GetWindowName()
		}
	}

	return "files" // default
}
