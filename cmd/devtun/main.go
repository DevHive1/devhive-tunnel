// Command devtun is the devhive-tunnel device client: setup wizard, network
// creation (AI-assisted), joining, live chat, encrypted file sharing.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"devhive-tunnel/pkg/aigen"
	"devhive-tunnel/pkg/client"
	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/crypto"
	"devhive-tunnel/pkg/hub"
	"devhive-tunnel/pkg/i18n"
	"devhive-tunnel/pkg/mesh"
	"devhive-tunnel/pkg/tui"
	"devhive-tunnel/pkg/tun"
)

var stdin = bufio.NewReader(os.Stdin)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	switch os.Args[1] {
	case "setup":
		cmdSetup()
	case "ai":
		cmdCreate(os.Args[2:], true)
	case "create":
		cmdCreate(os.Args[2:], false)
	case "join":
		cmdJoin(os.Args[2:])
	case "list":
		cmdList(os.Args[2:])
	case "status":
		cmdStatus()
	case "chat":
		cmdChat(os.Args[2:])
	case "listen":
		cmdListen(os.Args[2:])
	case "send":
		cmdSend(os.Args[2:])
	case "invite":
		cmdInvite(os.Args[2:])
	case "lan":
		cmdLan(os.Args[2:])
	case "tui":
		cmdTUI()
	case "version", "-v", "--version":
		fmt.Println("devhive-tunnel", conf.Version)
	default:
		usage()
	}
}

func usage() {
	fmt.Print(`devhive-tunnel — secure p2p hive networks

usage: devtun <command> [flags]

  setup              interactive first-run wizard (create / join)
  ai                 AI-guided network creation (recommended)
  create             create a network with flags
  join <name|id>     join an existing network
  list               show public networks on the hub
  status             show your identity and saved networks
  tui                full-screen wizard + chat interface
  chat [network]     live encrypted group chat + DMs + files
  listen [network]   receive chats/files in the background
  send <peer> <file> send a file to a peer
  invite             create a join invite (owner only)
  lan [network]      real LAN mode (TUN) for LAN games
  version            print version

common flags: -hub <addr> -nick <name>
`)
}

func errf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "devtun: "+format+"\n", a...)
	os.Exit(1)
}

func platform() string {
	switch runtime.GOOS {
	case "android":
		return "android"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

func loadConf() *conf.Client {
	c, err := conf.Load("")
	if err != nil {
		errf("config: %v", err)
	}
	return c
}

func ensureIdentity(cfg *conf.Client) *crypto.Identity {
	seedHex, err := cfg.LoadSeedHex()
	if err != nil {
		errf("identity: %v", err)
	}
	if seedHex == "" {
		id, err := crypto.NewIdentity()
		if err != nil {
			errf("identity: %v", err)
		}
		if err := cfg.StoreSeedHex(id.SaveSeedHexText()); err != nil {
			errf("identity save: %v", err)
		}
		fmt.Println("✦ new device identity created:", id.Fingerprint())
		return id
	}
	id, err := crypto.LoadSeedHexText(seedHex)
	if err != nil {
		errf("identity corrupt: %v", err)
	}
	return id
}

func ask(prompt, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", prompt, def)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func askSecret(prompt string) string {
	fmt.Printf("%s: ", prompt)
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}

func tr(lang *i18n.Set, key string) string { return lang.T(key) }

// ---------------- setup wizard ----------------

func cmdSetup() {
	cfg := loadConf()
	fmt.Println("🐝  devhive-tunnel setup")
	fmt.Println("────────────────────────")

	if cfg.Language == "" {
		fmt.Println("1) English   2) العربية")
		switch ask("language / اللغة", "1") {
		case "2":
			cfg.Language = "ar"
		default:
			cfg.Language = "en"
		}
	}
	lang := i18n.Load(cfg.Language)

	cfg.Hub = ask(tr(lang, "setup.welcome")+" — hub address", cfg.Hub)
	cfg.Nickname = ask("nickname / الاسم", cfg.Nickname)
	if cfg.Nickname == "" {
		cfg.Nickname = "peer-" + time.Now().Format("1504")
	}
	_ = ensureIdentity(cfg)

	fmt.Println("\n" + tr(lang, "setup.choose-action"))
	fmt.Println("  1) " + tr(lang, "setup.create-network"))
	fmt.Println("  2) " + tr(lang, "setup.join-network"))
	switch ask("→", "1") {
	case "2":
		cmdJoinWith(cfg, nil)
	default:
		cmdCreateWith(cfg, nil, true)
	}
	if err := cfg.Save(); err != nil {
		errf("save: %v", err)
	}
	fmt.Println("\n✓", tr(lang, "state.saved"), "—", conf.DefaultDir())
}

// ---------------- AI / create ----------------

func cmdCreate(args []string, aiDefault bool) {
	cfg := loadConf()
	cmdCreateWith(cfg, args, aiDefault)
	_ = cfg.Save()
}

func cmdCreateWith(cfg *conf.Client, args []string, aiDefault bool) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "network name")
	password := fs.String("password", "", "network password (auto-generated if empty)")
	profile := fs.String("profile", "team", "security profile: private|team|community|paranoid")
	members := fs.Int("members", 8, "expected members")
	lan := fs.Bool("lan", false, "LAN games mode")
	files := fs.Bool("files", true, "file sharing")
	invite := fs.Bool("invite", false, "force invite key requirement")
	anonymous := fs.Bool("anonymous", false, "list network publicly")
	hubAddr := fs.String("hub", cfg.Hub, "hub address")
	nick := fs.String("nick", cfg.Nickname, "your nickname")
	assumeYes := fs.Bool("yes", false, "skip the final confirmation")
	if args != nil {
		_ = fs.Parse(args)
	}

	nameVal := *name
	profileVal := aigen.Profile(*profile)
	membersVal := *members
	lanVal := *lan
	inviteVal := *invite
	anonymousVal := *anonymous

	if args == nil || nameVal == "" {
		if cfg.Language == "ar" {
			lang := i18n.Load("ar")
			nameVal = ask(lang.T("create.name"), "")
			fmt.Println("الملفات الشخصية: private / team / community / paranoid")
			if v := ask("الملف الأمني", "team"); v != "" {
				profileVal = aigen.Profile(v)
			}
			lanVal = strings.HasPrefix(strings.ToLower(ask("هل تحتاج ألعاب شبكة محلية؟ (y/n)", "n")), "y")
			inviteVal = strings.HasPrefix(strings.ToLower(ask("طلب مفتاح دعوة للانضمام؟ (y/n)", "y")), "y")
		} else {
			lang := i18n.Load("en")
			nameVal = ask(lang.T("create.name"), "")
			fmt.Println("profiles: private / team / community / paranoid")
			if v := ask("security profile", "team"); v != "" {
				profileVal = aigen.Profile(v)
			}
			lanVal = strings.HasPrefix(strings.ToLower(ask("LAN games mode? (y/n)", "n")), "y")
			inviteVal = strings.HasPrefix(strings.ToLower(ask("require invite key? (y/n)", "y")), "y")
		}
	}
	if nameVal == "" {
		errf("network name is required")
	}

	in := aigen.Input{
		Name: nameVal, Profile: profileVal, Members: membersVal,
		LANGames: lanVal, FileSharing: *files, Language: cfg.Language,
	}
	plan := aigen.Generate(in)
	if *password != "" {
		plan.Password = *password
	}
	if inviteVal {
		plan.Settings.RequireInviteKey = true
	}
	if anonymousVal {
		plan.Settings.AllowAnonymous = true
	}
	if !aiDefault && *profile == "team" && !*invite {
		// plain create defaults: still safe but no forced invites only if user asked
	}

	fmt.Println("\n🐝 AI network plan")
	fmt.Printf("  name      : %s\n", plan.Name)
	fmt.Printf("  password  : %s\n", plan.Password)
	fmt.Printf("  subnet    : %s\n", plan.SubnetCIDR)
	fmt.Printf("  invite req: %v   public: %v   max peers: %d\n",
		plan.Settings.RequireInviteKey, plan.Settings.AllowAnonymous, plan.Settings.MaxPeers)
	fmt.Printf("  security  : %d/100\n", plan.SecurityScore)
	fmt.Println("  why:")
	for _, r := range plan.Rationale {
		fmt.Println("   •", r)
	}
	if !*assumeYes && !strings.HasPrefix(strings.ToLower(ask("\ncreate this network now? (y/n)", "y")), "y") {
		fmt.Println("cancelled")
		return
	}

	id := ensureIdentity(cfg)
	cc := clientConfig(cfg, id, *hubAddr, *nick)
	resp, c, adminKey, err := createNetwork(cc, plan)
	_ = c
	if err != nil {
		errf("create: %v", err)
	}
	net := &conf.Network{
		ID: resp.NetworkID, Name: plan.Name, Password: plan.Password,
		AdminKey: adminKey, Created: true,
	}
	cfg.AddNetwork(net)
	cfg.Hub = *hubAddr
	if cfg.Nickname == "" {
		cfg.Nickname = *nick
	}
	_ = cfg.Save()

	fmt.Println("\n✓ network created")
	fmt.Println("  network ID :", resp.NetworkID)
	fmt.Println("  admin key  :", adminKey)
	fmt.Println("  password   :", plan.Password)
	fmt.Println("  dashboard  : http://" + dashboardAddr(*hubAddr) + "  (owner login)")
	fmt.Println("\n⚠ save the admin key now — it is never shown again.")
	if len(plan.Checklist) > 0 {
		fmt.Println("\nchecklist:")
		for _, s := range plan.Checklist {
			fmt.Println(" •", s)
		}
	}
}

func createNetwork(cc client.Config, plan aigen.Plan) (hub.CreateResp, *client.Client, string, error) {
	cc.Network = &conf.Network{Name: plan.Name, Password: plan.Password}
	c, resp, err := client.Create(cc, hub.CreateReq{
		Name: plan.Name, Password: plan.Password,
		Nickname: cc.Nickname, Settings: &plan.Settings,
	})
	if err != nil {
		return hub.CreateResp{}, nil, "", err
	}
	return resp, c, resp.AdminKey, nil
}

func clientConfig(cfg *conf.Client, id *crypto.Identity, hubAddr, nick string) client.Config {
	if nick == "" {
		nick = cfg.Nickname
	}
	if nick == "" {
		nick = "peer-" + id.Fingerprint()[:6]
	}
	return client.Config{
		Hub: hubAddr, Identity: id, Nickname: nick, Platform: platform(),
		DownloadDir: cfg.DownloadsDir(),
	}
}

func dashboardAddr(hubAddr string) string {
	host, _, err := splitHost(hubAddr)
	if err != nil {
		return hubAddr
	}
	return host + ":7600"
}

func splitHost(addr string) (string, string, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return addr, "", nil
	}
	return addr[:i], addr[i+1:], nil
}

// ---------------- join ----------------

func cmdJoin(args []string) {
	cfg := loadConf()
	cmdJoinWith(cfg, args)
	_ = cfg.Save()
}

func cmdJoinWith(cfg *conf.Client, args []string) {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	password := fs.String("password", "", "network password")
	adminKey := fs.String("admin-key", "", "admin/invite key (if required)")
	nick := fs.String("nick", cfg.Nickname, "your nickname")
	hubAddr := fs.String("hub", cfg.Hub, "hub address")
	stay := fs.Bool("stay", false, "stay connected and show events")
	var target string
	if args != nil {
		_ = fs.Parse(args)
		if fs.NArg() > 0 {
			target = fs.Arg(0)
		}
	}
	if target == "" {
		lang := i18n.Load(cfg.Language)
		target = ask(lang.T("join.name-or-id"), "")
	}
	if target == "" {
		errf("network name or id required")
	}
	pass := *password
	if pass == "" {
		lang := i18n.Load(cfg.Language)
		pass = askSecret(lang.T("join.password"))
	}
	if pass == "" {
		errf("password required")
	}
	id := ensureIdentity(cfg)
	cc := clientConfig(cfg, id, *hubAddr, *nick)
	cc.Network = &conf.Network{ID: target, Name: target, Password: pass, AdminKey: *adminKey}
	c, err := client.Join(cc)
	if err != nil {
		lang := i18n.Load(cfg.Language)
		errf("%s: %v", lang.T("join.wrong-credentials"), err)
	}
	defer c.Close()
	info := c.Info()
	lang := i18n.Load(cfg.Language)
	fmt.Println("✓", lang.T("join.success"), "→", info.Name, "("+info.NetworkID+")")
	fmt.Println("  your virtual IP:", info.YourVLANIP)
	net := &conf.Network{
		ID: info.NetworkID, Name: info.Name, Password: pass,
		AdminKey: *adminKey, Hub: *hubAddr,
	}
	cfg.AddNetwork(net)
	cfg.Hub = *hubAddr
	_ = cfg.Save()
	printPeers(c)
	if *stay {
		eventLoop(c)
	}
}

func printPeers(c *client.Client) {
	peers := c.Peers()
	if len(peers) == 0 {
		fmt.Println("  (no peers online yet)")
		return
	}
	fmt.Println("  peers:")
	for _, p := range peers {
		state := "offline"
		if p.Online {
			state = "online"
		}
		role := ""
		if p.Admin {
			role = " [admin]"
		}
		fmt.Printf("   • %-16s %-8s %-16s %s%s\n", p.Nickname, state, p.VLANIP, p.Fingerprint, role)
	}
}

// ---------------- list / status ----------------

func cmdList(args []string) {
	cfg := loadConf()
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	query := fs.String("query", "", "search text")
	hubAddr := fs.String("hub", cfg.Hub, "hub address")
	_ = fs.Parse(args)
	id := ensureIdentity(cfg)
	nets, err := client.List(*hubAddr, id, *query)
	if err != nil {
		errf("list: %v", err)
	}
	if len(nets) == 0 {
		fmt.Println("no public networks found")
		return
	}
	for _, n := range nets {
		fmt.Printf("• %-24s %-18s %d online\n", n.Name, n.ID, n.Online)
	}
}

func cmdStatus() {
	cfg := loadConf()
	id := ensureIdentity(cfg)
	fmt.Println("🐝 devhive-tunnel", conf.Version)
	fmt.Println("identity :", id.Fingerprint())
	fmt.Println("hub      :", cfg.Hub)
	fmt.Println("nickname :", cfg.Nickname)
	fmt.Println("config   :", conf.DefaultDir())
	if len(cfg.Networks) == 0 {
		fmt.Println("networks : none — run `devtun setup`")
		return
	}
	fmt.Println("networks :")
	for _, n := range cfg.Networks {
		role := "member"
		if n.Created || n.AdminKey != "" {
			role = "owner/admin"
		}
		state := "saved"
		fmt.Printf("  • %-20s %-16s %s (%s)\n", n.Name, n.ID, role, state)
	}
}

// ---------------- live session helpers ----------------

func connectSaved(cfg *conf.Client, args []string) *client.Client {
	fs := flag.NewFlagSet("session", flag.ExitOnError)
	nick := fs.String("nick", cfg.Nickname, "nickname")
	hubAddr := fs.String("hub", cfg.Hub, "hub address")
	_ = fs.Parse(args)
	var arg string
	if fs.NArg() > 0 {
		arg = fs.Arg(0)
	}
	var net *conf.Network
	if arg != "" {
		net = cfg.FindNetwork(arg)
		if net == nil {
			errf("network %q not found in config — join it first", arg)
		}
	} else if len(cfg.Networks) == 1 {
		net = cfg.Networks[0]
	} else if len(cfg.Networks) > 1 {
		fmt.Println("choose a network:")
		for i, n := range cfg.Networks {
			fmt.Printf("  %d) %s (%s)\n", i+1, n.Name, n.ID)
		}
		idx := 0
		fmt.Scanln(&idx)
		if idx < 1 || idx > len(cfg.Networks) {
			errf("bad choice")
		}
		net = cfg.Networks[idx-1]
	} else {
		errf("no networks — run `devtun setup`")
	}
	id := ensureIdentity(cfg)
	h := *hubAddr
	if net.Hub != "" {
		h = net.Hub
	}
	cc := clientConfig(cfg, id, h, *nick)
	cc.Network = net
	c, err := client.Join(cc)
	if err != nil {
		errf("join: %v", err)
	}
	return c
}

func eventLoop(c *client.Client) {
	words := map[string]string{}
	words[client.EvGroupChat] = "💬"
	words[client.EvDM] = "✉"
	words[client.EvPeerEvent] = "👥"
	words[client.EvFileOffer] = "📥"
	words[client.EvFileDone] = "✅"
	words[client.EvError] = "⚠"
	words[client.EvConnected] = "🔗"

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; c.Close(); os.Exit(0) }()
	for ev := range c.Events {
		icon := words[ev.Type]
		if icon == "" {
			icon = "•"
		}
		switch ev.Type {
		case client.EvGroupChat, client.EvDM:
			fmt.Printf("\n%s %s\n> ", icon, ev.Text)
		case client.EvPeerEvent:
			state := ev.Note
			name := "?"
			if ev.Peer != nil {
				name = ev.Peer.Nickname
			}
			fmt.Printf("\n%s %s: %s\n> ", icon, name, state)
		case client.EvFileDone:
			fmt.Printf("\n%s file saved: %s\n> ", icon, ev.Note)
		case client.EvFileOffer:
			if ev.File != nil {
				fmt.Printf("\n%s incoming file: %s (%d bytes)\n> ", icon, ev.File.Name, ev.File.Size)
			}
		case client.EvError:
			if ev.Err == "disconnected from hub" {
				fmt.Println("\n⚠ disconnected")
				return
			}
			fmt.Printf("\n%s %s %s\n> ", icon, ev.Err, ev.Note)
		}
	}
}

func cmdListen(args []string) {
	cfg := loadConf()
	c := connectSaved(cfg, args)
	defer c.Close()
	fmt.Println("👂 listening — messages and files will appear here (ctrl+c to quit)")
	eventLoop(c)
}

func cmdChat(args []string) {
	cfg := loadConf()
	c := connectSaved(cfg, args)
	defer c.Close()
	fmt.Println("💬 chat — /peers /dm <peer> <text> /send <peer> <file> /invite /quit")
	go eventLoop(c)

	for {
		fmt.Print("> ")
		line, err := stdin.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			fields := strings.Fields(line)
			switch fields[0] {
			case "/quit", "/exit":
				return
			case "/peers":
				printPeers(c)
			case "/dm":
				if len(fields) < 3 {
					fmt.Println("usage: /dm <peer> <message>")
					continue
				}
				if err := c.SendDM(fields[1], strings.Join(fields[2:], " ")); err != nil {
					fmt.Println("⚠", err)
				}
			case "/send":
				if len(fields) < 3 {
					fmt.Println("usage: /send <peer> <file>")
					continue
				}
				go func() {
					if _, err := c.SendFile(fields[1], fields[2]); err != nil {
						fmt.Println("⚠", err)
					}
				}()
			case "/invite":
				resp, err := c.Invite(hub.InviteReq{UsesMax: 1, ExpiresDays: 7})
				if err != nil {
					fmt.Println("⚠", err)
				} else {
					fmt.Println("invite key:", resp.Code, "(single use, 7 days)")
				}
			default:
				fmt.Println("unknown command")
			}
			continue
		}
		if err := c.SendGroupChat(line); err != nil {
			fmt.Println("⚠", err)
		}
	}
}

func cmdSend(args []string) {
	cfg := loadConf()
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() < 2 {
		errf("usage: devtun send <peer> <file> [network]")
	}
	peer, file := fs.Arg(0), fs.Arg(1)
	var rest []string
	if fs.NArg() > 2 {
		rest = []string{fs.Arg(2)}
	}
	c := connectSaved(cfg, rest)
	defer c.Close()
	offer, err := c.SendFile(peer, file)
	if err != nil {
		errf("send: %v", err)
	}
	fmt.Printf("✓ sent %s (%d bytes, %d chunks, sha256 %s…)\n", offer.Name, offer.Size, offer.Chunks, offer.SHA256[:16])
}

func cmdInvite(args []string) {
	cfg := loadConf()
	fs := flag.NewFlagSet("invite", flag.ExitOnError)
	note := fs.String("note", "", "who is it for")
	uses := fs.Int("uses", 1, "max uses (0 unlimited)")
	days := fs.Int("days", 7, "expiry days (0 never)")
	role := fs.String("role", "member", "member|admin")
	var rest []string
	_ = fs.Parse(args)
	if fs.NArg() > 0 {
		rest = []string{fs.Arg(0)}
	}
	c := connectSaved(cfg, rest)
	defer c.Close()
	resp, err := c.Invite(hub.InviteReq{Note: *note, UsesMax: *uses, ExpiresDays: *days, Role: *role})
	if err != nil {
		errf("invite: %v", err)
	}
	fmt.Println("invite key:", resp.Code)
	fmt.Println("share it with the invitee; they join with: devtun join <network>")
}

func cmdTUI() {
	cfg := loadConf()
	id := ensureIdentity(cfg)
	if err := tui.Run(cfg, id); err != nil {
		errf("tui: %v", err)
	}
}

func cmdLan(args []string) {
	cfg := loadConf()
	fs := flag.NewFlagSet("lan", flag.ExitOnError)
	devName := fs.String("dev", "", "interface name")
	noCfg := fs.Bool("no-config", false, "skip address configuration")
	_ = fs.Parse(args)
	var rest []string
	if fs.NArg() > 0 {
		rest = []string{fs.Arg(0)}
	}
	c := connectSaved(cfg, rest)
	defer c.Close()
	info := c.Info()

	dev, err := tun.Open(*devName)
	if err != nil {
		fmt.Println("✗ LAN mode unavailable:", err)
		fmt.Println("  chat/files still work fully encrypted: `devtun chat` / `devtun send`")
		return
	}
	defer dev.Close()
	if !*noCfg {
		if err := tun.Configure(dev, info.YourVLANIP+"/16", info.Settings.SubnetCIDR); err != nil {
			fmt.Println("⚠ address setup failed:", err)
			fmt.Printf("  as root run: ip addr add %s/16 dev %s && ip link set %s up\n",
				info.YourVLANIP, dev.Name(), dev.Name())
		}
	}
	m, err := mesh.Start(c, dev)
	if err != nil {
		errf("mesh: %v", err)
	}
	defer m.Close()
	fmt.Printf("🐝 LAN mode up: %s ip=%s subnet=%s mtu=1400\n",
		dev.Name(), info.YourVLANIP, info.Settings.SubnetCIDR)
	fmt.Println("   peers are reachable at their 100.x addresses; broadcast/multicast LAN discovery is relayed.")
	fmt.Println("   ctrl+c to stop.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	tick := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-sig:
			return
		case <-tick.C:
			in, out, drops := m.Stats()
			fmt.Printf("[lan] in=%d out=%d drops=%d\n", in, out, drops)
		}
	}
}
