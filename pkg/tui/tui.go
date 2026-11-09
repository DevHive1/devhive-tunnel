// Package tui is the terminal UI: a first-run wizard (create/join, bilingual)
// and a live chat screen with peers, DMs and encrypted file sending.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"devhive-tunnel/pkg/aigen"
	"devhive-tunnel/pkg/client"
	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/crypto"
	"devhive-tunnel/pkg/hub"
)

var (
	accent = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	okc    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	frame  = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214"))
)

type step struct {
	key, prompt, def string
	secret           bool
}

type mode int

const (
	modeWizard mode = iota
	modeChat
)

type app struct {
	cfg *conf.Client
	id  *crypto.Identity

	mode   mode
	steps  []step
	at     int
	answer map[string]string
	err    error
	busy   bool

	plan   *aigen.Plan
	cl     *client.Client
	info   hub.JoinInfo
	notice string

	input textinput.Model
	view  viewport.Model
	peers []*hub.PeerRecord
	lines []string

	width  int
	height int
	ready  bool
}

type connectedMsg struct {
	c     *client.Client
	admin string
	err   error
	net   *conf.Network
	hub   string
	nick  string
}

type eventMsg struct{ ev client.Event }
type closedMsg struct{}
type inviteMsg struct {
	code string
	err  error
}
type tickMsg time.Time

// Run starts the TUI. cfg/id come from the CLI layer.
func Run(cfg *conf.Client, id *crypto.Identity) error {
	ti := textinput.New()
	ti.Placeholder = "…"
	ti.Focus()
	ti.CharLimit = 4096

	a := &app{cfg: cfg, id: id, answer: map[string]string{}, input: ti}
	a.buildWizard()
	p := tea.NewProgram(a, tea.WithAltScreen())
	// cfg.Save errors already reported through the UI when relevant.
	if _, err := p.Run(); err != nil {
		return err
	}
	if a.cl != nil {
		a.cl.Close()
	}
	return cfg.Save()
}

func (a *app) buildWizard() {
	lang := a.cfg.Language
	if lang == "" {
		lang = "en"
	}
	a.steps = []step{
		{key: "lang", prompt: "Language / اللغة (1 English, 2 العربية)", def: "1"},
		{key: "nick", prompt: "Your nickname / اسمك", def: a.cfg.Nickname},
		{key: "hub", prompt: "Hub address", def: a.cfg.Hub},
		{key: "action", prompt: "Create a new network (1) or join one (2)?", def: "1"},
	}
	a.at = 0
	a.input.SetValue(a.steps[0].def)
}

func (a *app) expandSteps() {
	action := a.answer["action"]
	if action == "2" {
		a.steps = append(a.steps,
			step{key: "network", prompt: "Network name or id"},
			step{key: "password", prompt: "Network password", secret: true},
			step{key: "invite", prompt: "Invite key (leave empty if not needed)"},
		)
	} else {
		a.steps = append(a.steps,
			step{key: "name", prompt: "Network name"},
			step{key: "profile", prompt: "Security profile: private / team / community / paranoid", def: "team"},
			step{key: "members", prompt: "Expected members", def: "8"},
			step{key: "lan", prompt: "LAN games mode? (y/n)", def: "n"},
			step{key: "confirm", prompt: "Create it now? (y/n)", def: "y"},
		)
	}
}

func (a *app) nextStep() tea.Cmd {
	a.steps[a.at].def = "" // defs are only defaults when first shown
	a.at++
	if a.at >= len(a.steps) {
		return a.submit()
	}
	st := a.steps[a.at]
	a.input.SetValue(st.def)
	if st.secret {
		a.input.EchoMode = textinput.EchoPassword
	} else {
		a.input.EchoMode = textinput.EchoNormal
	}
	a.input.Focus()
	return nil
}

func (a *app) submit() tea.Cmd {
	a.busy = true
	a.err = nil
	answers := a.answer
	if answers["action"] == "2" {
		return a.joinCmd(answers)
	}
	return a.createCmd(answers)
}

func (a *app) joinCmd(answers map[string]string) tea.Cmd {
	net := &conf.Network{
		ID: answers["network"], Name: answers["network"],
		Password: answers["password"], AdminKey: answers["invite"],
	}
	cc := client.Config{
		Hub: answers["hub"], Identity: a.id, Nickname: answers["nick"],
		Network: net, Platform: "tui", DownloadDir: a.cfg.DownloadsDir(),
	}
	return func() tea.Msg {
		c, err := client.Join(cc)
		if err != nil {
			return connectedMsg{err: err}
		}
		return connectedMsg{c: c, hub: answers["hub"], nick: answers["nick"]}
	}
}

func (a *app) createCmd(answers map[string]string) tea.Cmd {
	members := 8
	fmt.Sscanf(answers["members"], "%d", &members)
	plan := aigen.Generate(aigen.Input{
		Name: answers["name"], Profile: aigen.Profile(answers["profile"]), Members: members,
		LANGames:    strings.HasPrefix(strings.ToLower(answers["lan"]), "y"),
		FileSharing: true, Language: a.cfg.Language,
	})
	a.plan = &plan
	cc := client.Config{
		Hub: answers["hub"], Identity: a.id, Nickname: answers["nick"],
		Platform: "tui", DownloadDir: a.cfg.DownloadsDir(),
		Network: &conf.Network{Name: plan.Name, Password: plan.Password},
	}
	return func() tea.Msg {
		_, resp, err := client.Create(cc, hub.CreateReq{
			Name: plan.Name, Password: plan.Password,
			Nickname: cc.Nickname, Settings: &plan.Settings,
		})
		if err != nil {
			return connectedMsg{err: err}
		}
		net := &conf.Network{
			ID: resp.NetworkID, Name: resp.Name, Password: plan.Password,
			AdminKey: resp.AdminKey, Created: true, Hub: answers["hub"],
		}
		c, err := client.Join(cc)
		if err != nil {
			return connectedMsg{admin: resp.AdminKey, net: net, err: err}
		}
		return connectedMsg{c: c, admin: resp.AdminKey, net: net, hub: answers["hub"], nick: answers["nick"]}
	}
}

func waitEvent(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-c.Events
		if !ok {
			return closedMsg{}
		}
		return eventMsg{ev: ev}
	}
}

func tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Init implements tea.Model.
func (a *app) Init() tea.Cmd { return textinput.Blink }

// Update implements tea.Model.
func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		if !a.ready && m.Width > 10 && m.Height > 10 {
			a.view = viewport.New(m.Width-4, m.Height-8)
			a.ready = true
		} else if a.ready {
			a.view.Width = m.Width - 4
			a.view.Height = m.Height - 8
		}
		a.input.Width = m.Width - 6
		return a, nil

	case connectedMsg:
		a.busy = false
		if m.err != nil {
			a.err = m.err
			return a, nil
		}
		a.cl = m.c
		a.info = m.c.Info()
		if m.net != nil {
			a.cfg.AddNetwork(m.net)
		}
		a.cfg.Hub = m.hub
		a.cfg.Nickname = m.nick
		a.mode = modeChat
		a.appendLine(okc.Render("connected to " + a.info.Name + " (" + a.info.NetworkID + ") as " + a.info.YourVLANIP))
		if m.admin != "" {
			a.appendLine(accent.Render("admin key (save it now!): " + m.admin))
		}
		a.peers = a.cl.Peers()
		a.input.SetValue("")
		a.input.EchoMode = textinput.EchoNormal
		a.input.Focus()
		return a, tea.Batch(waitEvent(a.cl), tick())

	case eventMsg:
		a.handleEvent(m.ev)
		return a, waitEvent(a.cl)

	case closedMsg:
		a.appendLine(dim.Render("connection closed"))
		return a, nil

	case inviteMsg:
		if m.err != nil {
			a.appendLine(dim.Render("invite failed: " + m.err.Error()))
		} else {
			a.appendLine(accent.Render("invite key: " + m.code + " (single use, 7 days)"))
		}
		return a, nil

	case tickMsg:
		if a.cl != nil {
			a.peers = a.cl.Peers()
		}
		return a, tick()

	case tea.KeyMsg:
		switch m.String() {
		case "ctrl+c":
			if a.cl != nil {
				a.cl.Close()
			}
			return a, tea.Quit
		case "enter":
			if a.mode == modeWizard {
				return a, a.wizardEnter()
			}
			return a, a.chatEnter()
		}
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	if a.ready {
		a.view, _ = a.view.Update(msg)
	}
	return a, cmd
}

func (a *app) wizardEnter() tea.Cmd {
	val := strings.TrimSpace(a.input.Value())
	st := a.steps[a.at]
	if st.secret && val == "" {
		a.err = fmt.Errorf("value is required")
		return nil
	}
	a.answer[st.key] = val
	a.err = nil
	switch st.key {
	case "lang":
		if val == "2" {
			a.cfg.Language = "ar"
		} else {
			a.cfg.Language = "en"
		}
	case "nick":
		a.cfg.Nickname = val
	case "hub":
		a.cfg.Hub = val
	case "action":
		if val != "2" {
			a.answer["action"] = "1"
		}
		a.expandSteps()
	case "confirm":
		if !strings.HasPrefix(strings.ToLower(val), "y") {
			a.steps = a.steps[:a.at]
			a.at = 0
			a.input.SetValue(a.steps[0].def)
			return nil
		}
	}
	return a.nextStep()
}

func (a *app) chatEnter() tea.Cmd {
	text := strings.TrimSpace(a.input.Value())
	if text == "" {
		return nil
	}
	a.input.SetValue("")
	switch {
	case text == "/quit":
		a.cl.Close()
		return tea.Quit
	case text == "/peers":
		a.appendLine(accent.Render(peerList(a.peers)))
	case text == "/invite":
		c := a.cl
		a.appendLine(dim.Render("creating invite…"))
		return func() tea.Msg {
			resp, err := c.Invite(hub.InviteReq{UsesMax: 1, ExpiresDays: 7})
			return inviteMsg{code: resp.Code, err: err}
		}
	case strings.HasPrefix(text, "/dm "):
		parts := strings.SplitN(text[4:], " ", 2)
		if len(parts) < 2 {
			a.appendLine(dim.Render("usage: /dm <peer> <text>"))
			break
		}
		if err := a.cl.SendDM(parts[0], parts[1]); err != nil {
			a.appendLine(dim.Render("dm failed: " + err.Error()))
		} else {
			a.appendLine(accent.Render("→ " + parts[0] + " (dm): " + parts[1]))
		}
	case strings.HasPrefix(text, "/send "):
		parts := strings.SplitN(text[6:], " ", 2)
		if len(parts) < 2 {
			a.appendLine(dim.Render("usage: /send <peer> <file>"))
			break
		}
		c, peer, path := a.cl, parts[0], parts[1]
		a.appendLine(dim.Render("sending " + path + "…"))
		go func() {
			if _, err := c.SendFile(peer, path); err != nil {
				_ = err
			}
		}()
	default:
		if err := a.cl.SendGroupChat(text); err != nil {
			a.appendLine(dim.Render("send failed: " + err.Error()))
		} else {
			nick := a.cfg.Nickname
			if nick == "" {
				nick = "you"
			}
			a.appendLine(nick + ": " + text)
		}
	}
	return nil
}

func (a *app) handleEvent(ev client.Event) {
	switch ev.Type {
	case client.EvGroupChat:
		a.appendLine(ev.Text)
	case client.EvDM:
		a.appendLine(accent.Render("✉ " + ev.Text))
	case client.EvPeerEvent:
		if ev.Peer != nil {
			state := ev.Note
			a.appendLine(dim.Render("• " + ev.Peer.Nickname + " " + state))
			a.peers = a.cl.Peers()
		}
	case client.EvFileOffer:
		if ev.File != nil {
			a.appendLine(okc.Render("📥 incoming file " + ev.File.Name))
		}
	case client.EvFileDone:
		a.appendLine(okc.Render("✅ file saved: " + ev.Note))
	case client.EvError:
		a.appendLine(dim.Render("⚠ " + ev.Err))
	}
}

func (a *app) appendLine(s string) {
	a.lines = append(a.lines, s)
	if len(a.lines) > 2000 {
		a.lines = a.lines[len(a.lines)-1500:]
	}
	if a.ready {
		a.view.SetContent(strings.Join(a.lines, "\n"))
		a.view.GotoBottom()
	}
}

func peerList(peers []*hub.PeerRecord) string {
	if len(peers) == 0 {
		return "no peers online"
	}
	var sb strings.Builder
	sb.WriteString("peers:\n")
	for _, p := range peers {
		st := "offline"
		if p.Online {
			st = "online"
		}
		role := ""
		if p.Admin {
			role = " [admin]"
		}
		fmt.Fprintf(&sb, "  • %-14s %-7s %-15s%s\n", p.Nickname, st, p.VLANIP, role)
	}
	return sb.String()
}

// View implements tea.Model.
func (a *app) View() string {
	if !a.ready || a.width == 0 {
		return "loading…"
	}
	if a.mode == modeWizard {
		return a.wizardView()
	}
	return a.chatView()
}

func (a *app) wizardView() string {
	if a.busy || a.at >= len(a.steps) {
		var b strings.Builder
		b.WriteString(accent.Render("🐝 devhive-tunnel") + "\n\n")
		b.WriteString(accent.Render("connecting…") + "\n\n")
		if a.err != nil {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("✗ "+a.err.Error()) + "\n\n")
		}
		b.WriteString(dim.Render("ctrl+c = quit"))
		return frame.Width(min(a.width-4, 84)).Render(b.String())
	}
	st := a.steps[a.at]
	var sb strings.Builder
	sb.WriteString(accent.Render("🐝 devhive-tunnel") + "  " + dim.Render(fmt.Sprintf("step %d/%d", a.at+1, len(a.steps))) + "\n\n")
	sb.WriteString(st.prompt + "\n\n")
	sb.WriteString(a.input.View() + "\n\n")
	if a.err != nil {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("✗ "+a.err.Error()) + "\n\n")
	}
	if a.busy {
		sb.WriteString(accent.Render("working…") + "\n\n")
	}
	sb.WriteString(dim.Render("enter = next • ctrl+c = quit"))
	return frame.Width(min(a.width-4, 84)).Render(sb.String())
}

func (a *app) chatView() string {
	header := accent.Render("🐝 "+a.info.Name) +
		dim.Render(fmt.Sprintf("  you=%s  hub=%s  peers=%d", a.info.YourVLANIP, a.cfg.Hub, len(a.peers)))
	body := a.view.View()
	input := frame.Width(a.width - 4).Render(a.input.View())
	help := dim.Render("/dm <peer> <text>  /send <peer> <file>  /invite  /peers  /quit   enter=send  ctrl+c=quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, body, input, help)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
