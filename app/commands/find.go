package commands

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/ghdb/app/clients"
	"github.com/hay-kot/ghdb/app/commands/gdb"
	"github.com/urfave/cli/v2"
)

func openWithDefault(htmlURL string) error {
	opSys := runtime.GOOS

	switch opSys {
	case "darwin":
		return exec.Command("open", htmlURL).Start()
	case "linux":
		// TODO: I have no idea if this works
		return exec.Command("xdg-open", htmlURL).Start()
	case "windows":
		// TODO: I have no idea if this works
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", htmlURL).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func (c *Controller) Find(ctx *cli.Context) error {
	cache, err := c.GitDB.LoadCache()
	if err != nil {
		return err
	}

	// Create a new Model, which is the data for the program.
	model := newFind(cache)

	// Start the Bubble Tea loop.
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// -----------------------------------------------------------------------------
// Bubble Tea Model

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	boldText = lipgloss.NewStyle().
			Bold(true)

	titleColor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#3772FF"))

	titleStyle = titleColor.Copy().
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3772FF")).
				Render
)

const (
	IconGitTree     = "\ufb2b"
	IconIssue       = "\u26a0"
	IconPullRequest = "\uf113"
)

type findItem struct {
	pr   clients.PullRequest
	repo clients.Repository
}

func or(s1, s2 string) string {
	if s1 == "" {
		return s2
	}
	return s1
}

func (i findItem) Title() string {
	switch {
	case i.repo.Name != "":
		return IconGitTree + " " + i.repo.Owner.Login + "/" + i.repo.Name
	case i.pr.Title != "":
		return IconPullRequest + "  " + i.pr.User.Login + ": " + i.pr.Title
	default:
		return ""
	}
}

func (i findItem) Description() string {
	if i.repo.Name != "" {
		return i.repo.Description
	}
	return fmt.Sprintf("#%d %v", i.pr.Number, i.pr.RepositoryName())
}

func (i findItem) FilterValue() string { return or(i.repo.Name, i.pr.Title) }

type listKeyMap struct {
	toggleHelpMenu   key.Binding
	toggleRepoSearch key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		toggleHelpMenu: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "toggle help"),
		),
		toggleRepoSearch: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "toggle repo/issue search"),
		),
	}
}

type find struct {
	list         list.Model
	keys         *listKeyMap
	delegateKeys *delegateKeyMap

	searchRepos bool
	prs         []list.Item
	repos       []list.Item
}

func newFind(cache gdb.Cache) *find {
	var (
		delegateKeys = newDelegateKeyMap()
		listKeys     = newListKeyMap()
	)

	// Setup repos
	repos := make([]list.Item, len(cache.Repositories))
	for i, repo := range cache.Repositories {
		repos[i] = findItem{repo: repo}
	}

	prs := make([]list.Item, len(cache.PullRequests))
	for i, pr := range cache.PullRequests {
		prs[i] = findItem{pr: pr}
	}

	// Setup list
	delegate := newItemDelegate(delegateKeys)
	groceryList := list.New(repos, delegate, 0, 0)
	groceryList.Title = "Repositories"
	groceryList.Styles.Title = titleStyle
	groceryList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.toggleRepoSearch,
			listKeys.toggleHelpMenu,
		}
	}

	return &find{
		list:         groceryList,
		keys:         listKeys,
		delegateKeys: delegateKeys,
		searchRepos:  true,
		prs:          prs,
		repos:        repos,
	}
}

func (m *find) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m *find) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.toggleRepoSearch):
			m.searchRepos = !m.searchRepos

			if m.searchRepos {
				m.list.Title = "Repositories"
				m.list.SetItems(m.repos)
			} else {
				m.list.Title = "Pull Requests"
				m.list.SetItems(m.prs)
			}

			return m, nil

		case key.Matches(msg, m.keys.toggleHelpMenu):
			m.list.SetShowHelp(!m.list.ShowHelp())
			return m, nil
		}
	}

	// This will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *find) View() string {
	return appStyle.Render(m.list.View())
}

func newItemDelegate(keys *delegateKeyMap) list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		var item findItem
		var title string

		if i, ok := m.SelectedItem().(findItem); ok {
			title = i.Title()
			item = i
		} else {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, keys.choose):
				return m.NewStatusMessage(statusMessageStyle("You chose " + title))

			case key.Matches(msg, keys.open):

				if item.pr.URL != "" {
					_ = openWithDefault(item.pr.URL)
				} else {
					_ = openWithDefault(item.repo.WebURL)
				}
				return m.NewStatusMessage(statusMessageStyle("You opened" + title))
			}
		}

		return nil
	}

	help := []key.Binding{keys.choose, keys.open}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

type delegateKeyMap struct {
	choose key.Binding
	open   key.Binding
}

// Additional short help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		d.choose,
		d.open,
	}
}

// Additional full help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			d.choose,
			d.open,
		},
	}
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
		open: key.NewBinding(
			key.WithKeys("o", "o"),
			key.WithHelp("o", "open"),
		),
	}
}
