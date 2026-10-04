package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/host"
	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/install"
	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/modules"
	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/modules/printmod"
	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/print"
	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/update"
)

// version is overwritten by -ldflags at release time.
var version = "0.1.0"

func main() {
	installFlag := flag.Bool("install", false, "установить одну копию и службу, которая будит помощника при входе")
	uninstallFlag := flag.Bool("uninstall", false, "убрать службу и копию программы")
	showVersion := flag.Bool("version", false, "показать версию")
	enableUpdate := flag.Bool("enable-update", false, "включить самообновление с релизов, когда программу уже отдали другим")
	disableUpdate := flag.Bool("disable-update", false, "выключить самообновление")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *installFlag {
		if err := install.Install(exe); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Агаева ID установлена. Служба будит помощника в сессии вошедшего пользователя.")
		if !*enableUpdate && !*disableUpdate {
			return
		}
	}
	if *enableUpdate || *disableUpdate {
		if err := update.SetEnabled(update.SettingsPath(), *enableUpdate); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if *enableUpdate {
			fmt.Println("Самообновление включено. Помощник подхватит новый релиз в течение нескольких часов.")
		} else {
			fmt.Println("Самообновление выключено.")
		}
		return
	}
	if *uninstallFlag {
		if err := install.Uninstall(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Агаева ID удалена.")
		return
	}

	logger := newLogger()
	if install.RunningAsService() {
		if err := install.RunService(logger); err != nil {
			logger.Println(err)
			os.Exit(1)
		}
		return
	}
	if install.AlreadyRunning() {
		logger.Println("уже запущено в этой сессии")
		return
	}
	addr := host.ListenAddr(17321)
	if err := host.ValidateAddr(addr); err != nil {
		logger.Println(err)
		os.Exit(1)
	}
	if host.Probe(addr) {
		logger.Println("уже слушает 127.0.0.1:17321")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	catalog := []modules.Module{
		printmod.Module{Styles: &print.LiveStyles{}},
	}
	if update.Enabled(update.SettingsPath()) {
		logger.Println("самообновление включено")
		go update.Watch(ctx, update.Config{
			Repository: update.ReleasesRepository,
			Current:    version,
			Interval:   0,
			InstallDir: filepath.Dir(runningPath(exe)),
			Executable: runningPath(exe),
			Logf:       logger.Printf,
		})
	}

	srv := &host.Server{Addr: addr, Version: version, Modules: catalog, Log: logger}
	logger.Printf("Агаева ID %s слушает http://%s", version, addr)
	if err := srv.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Println(err)
		os.Exit(1)
	}
}

func runningPath(exe string) string {
	if installed := install.InstalledPath(); installed != "" {
		if _, err := os.Stat(installed); err == nil {
			return installed
		}
	}
	return exe
}

func newLogger() *log.Logger {
	dir := logDir()
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "agaeva-id.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return log.New(os.Stderr, "", log.LstdFlags)
	}
	return log.New(f, "", log.LstdFlags)
}

func logDir() string {
	if os.Getenv("ProgramData") != "" {
		return filepath.Join(os.Getenv("ProgramData"), "AgaevaId", "logs")
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "AgaevaId", "logs")
	}
	return os.TempDir()
}
