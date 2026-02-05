package cli

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/pjtatlow/bore/internal/config"
	"github.com/pjtatlow/bore/internal/ipc"
)

func runInteractive() error {
	// Load config to get available tunnels and groups
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Tunnels) == 0 && len(cfg.Groups) == 0 {
		fmt.Println("No tunnels or groups configured.")
		fmt.Println("Edit your config with: bore config edit")
		return nil
	}

	// Check if daemon is running
	daemonRunning := ipc.IsDaemonRunning()

	var client *ipc.Client
	var status *ipc.StatusResponse
	if daemonRunning {
		client, err = ipc.NewClient()
		if err != nil {
			return err
		}
		status, err = client.Status()
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}
	}

	// Build set of running tunnels
	runningTunnels := make(map[string]bool)
	if status != nil {
		for _, t := range status.Tunnels {
			runningTunnels[t.Name] = true
		}
	}

	// Build options for tunnels
	var tunnelOptions []huh.Option[string]
	tunnelNames := make([]string, 0, len(cfg.Tunnels))
	for name := range cfg.Tunnels {
		tunnelNames = append(tunnelNames, name)
	}
	sort.Strings(tunnelNames)

	for _, name := range tunnelNames {
		t := cfg.Tunnels[name]
		label := fmt.Sprintf("%s (%s:%d -> %s:%d)",
			name, t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)
		if runningTunnels[name] {
			label = "[*] " + label
		} else {
			label = "[ ] " + label
		}
		tunnelOptions = append(tunnelOptions, huh.NewOption(label, name))
	}

	// Build options for groups
	var groupOptions []huh.Option[string]
	groupNames := make([]string, 0, len(cfg.Groups))
	for name := range cfg.Groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	for _, name := range groupNames {
		g := cfg.Groups[name]
		label := fmt.Sprintf("%s - %s (%d tunnels)", name, g.Description, len(g.Tunnels))
		groupOptions = append(groupOptions, huh.NewOption(label, name))
	}

	// Ask what action to take
	var action string
	actionOptions := []huh.Option[string]{
		huh.NewOption("Manage tunnels", "tunnels"),
		huh.NewOption("Manage groups", "groups"),
	}

	if !daemonRunning {
		actionOptions = append(actionOptions, huh.NewOption("Start daemon", "start"))
	} else {
		actionOptions = append(actionOptions, huh.NewOption("Stop daemon", "stop"))
		actionOptions = append(actionOptions, huh.NewOption("View status", "status"))
	}

	err = huh.NewSelect[string]().
		Title("What would you like to do?").
		Options(actionOptions...).
		Value(&action).
		Run()

	if err != nil {
		return err
	}

	switch action {
	case "start":
		return runStart(nil, nil)

	case "stop":
		return runStop(nil, nil)

	case "status":
		return runStatus(nil, nil)

	case "tunnels":
		return manageTunnels(client, tunnelOptions, runningTunnels, daemonRunning)

	case "groups":
		return manageGroups(client, groupOptions, daemonRunning)
	}

	return nil
}

func manageTunnels(client *ipc.Client, options []huh.Option[string], running map[string]bool, daemonRunning bool) error {
	if len(options) == 0 {
		fmt.Println("No tunnels configured.")
		return nil
	}

	var selectedTunnels []string
	err := huh.NewMultiSelect[string]().
		Title("Select tunnels to toggle ([*] = running)").
		Options(options...).
		Value(&selectedTunnels).
		Run()

	if err != nil {
		return err
	}

	if len(selectedTunnels) == 0 {
		fmt.Println("No tunnels selected")
		return nil
	}

	if !daemonRunning {
		fmt.Println("Starting daemon first...")
		if err := runStart(nil, nil); err != nil {
			return err
		}
		client, err = ipc.NewClient()
		if err != nil {
			return err
		}
	}

	// Toggle selected tunnels
	for _, name := range selectedTunnels {
		if running[name] {
			fmt.Printf("Stopping tunnel '%s'... ", name)
			if err := client.TunnelDown(name); err != nil {
				fmt.Printf("error: %v\n", err)
			} else {
				fmt.Println("done")
			}
		} else {
			fmt.Printf("Starting tunnel '%s'... ", name)
			if err := client.TunnelUp(name); err != nil {
				fmt.Printf("error: %v\n", err)
			} else {
				fmt.Println("done")
			}
		}
	}

	return nil
}

func manageGroups(client *ipc.Client, options []huh.Option[string], daemonRunning bool) error {
	if len(options) == 0 {
		fmt.Println("No groups configured.")
		return nil
	}

	var selectedGroup string
	err := huh.NewSelect[string]().
		Title("Select a group").
		Options(options...).
		Value(&selectedGroup).
		Run()

	if err != nil {
		return err
	}

	var action string
	err = huh.NewSelect[string]().
		Title(fmt.Sprintf("What to do with group '%s'?", selectedGroup)).
		Options(
			huh.NewOption("Enable (start all tunnels)", "enable"),
			huh.NewOption("Disable (stop all tunnels)", "disable"),
		).
		Value(&action).
		Run()

	if err != nil {
		return err
	}

	if !daemonRunning {
		fmt.Println("Starting daemon first...")
		if err := runStart(nil, nil); err != nil {
			return err
		}
		client, err = ipc.NewClient()
		if err != nil {
			return err
		}
	}

	switch action {
	case "enable":
		fmt.Printf("Enabling group '%s'... ", selectedGroup)
		if err := client.GroupEnable(selectedGroup); err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Println("done")
		}
	case "disable":
		fmt.Printf("Disabling group '%s'... ", selectedGroup)
		if err := client.GroupDisable(selectedGroup); err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Println("done")
		}
	}

	return nil
}
