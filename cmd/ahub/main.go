package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "embed"
)

const (
	defaultPort = 8080
	serviceName  = "hub"
)

var (
	//go:embed templates/docker-compose.yml
	dockerComposeTemplate string

	//go:embed templates/Dockerfile
	dockerfileTemplate string

	//go:embed templates/.env.example
	envExampleTemplate string
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ahub:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}
	if isHelp(args[0]) {
		printHelp(stdout)
		return nil
	}

	rest := args[1:]
	switch args[0] {
	case "help":
		printHelp(stdout)
		return nil
	case "create":
		if len(args) < 2 {
			printHelp(stdout)
			return nil
		}
		switch args[1] {
		case "env":
			return runCreateEnv(args[2:], stdout)
		case "docker":
			return runCreateDocker(args[2:], stdout)
		default:
			return fmt.Errorf("unknown create command %q", args[1])
		}
	case "start":
		return runCompose("up", "-d")
	case "stop":
		return runCompose("down")
	case "rebuild":
		return runCompose("up", "-d", "--build")
	case "restart":
		return runCompose("restart")
	case "update":
		if err := tryGitPull(); err != nil {
			return err
		}
		if err := runCompose("pull"); err != nil {
			return err
		}
		if err := runCompose("run", "--rm", "--no-deps", "--build", serviceName, "/app/hub", "migrate"); err != nil {
			return err
		}
		if err := runCompose("up", "-d", "--build", "--remove-orphans"); err != nil {
			return err
		}
		return rebuildAhubBinary()
	case "status":
		if len(rest) != 0 {
			switch rest[0] {
			case "logs":
				fmt.Fprintln(stdout, "use: ahub logs")
				return nil
			case "backup":
				fmt.Fprintln(stdout, "use: ahub backup")
				return nil
			case "shell":
				fmt.Fprintln(stdout, "use: ahub shell")
				return nil
			default:
				return fmt.Errorf("status does not take arguments")
			}
		}
		return runCompose("ps")
	case "logs":
		return runLogs(rest)
	case "backup":
		if len(rest) != 0 {
			return fmt.Errorf("backup does not take arguments")
		}
		return runCompose("run", "--rm", "--no-deps", serviceName, "backup", "export")
	case "restore":
		return runRestore(rest)
	case "shell":
		return runCompose("exec", serviceName, "sh")
	case "destroy":
		return runCompose("down", "-v", "--remove-orphans")
	case "uninstall":
		return runUninstall(stdout, stderr)
	case "set":
		if len(args) < 2 {
			return errors.New("missing set command")
		}
		switch args[1] {
		case "port":
			return runSetPort(args[2:], stdout)
		default:
			return fmt.Errorf("unknown set command %q", args[1])
		}
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printHelp(w io.Writer) {
	color := helpColorEnabled(w)
	title := helpStyle(color, "bold", "ahub") + " - " + helpStyle(color, "cyan", "Hub server manager")
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, helpStyle(color, "dim", "Docker-first operations for the installed hub stack."))
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, helpStyle(color, "bold", "Usage"))
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub")+" <command> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, helpStyle(color, "bold", "Quick Start"))
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub create env"))
	fmt.Fprintln(w, "    Create `.env` with a generated secret key.")
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub create docker"))
	fmt.Fprintln(w, "    Generate Docker Compose and Dockerfile templates.")
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub start"))
	fmt.Fprintln(w, "    Start the stack in the install directory.")
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub logs"))
	fmt.Fprintln(w, "    Live-follow container logs (tail 100 by default).")
	fmt.Fprintln(w, "    Flags: --tail N, --since DURATION, --no-follow")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, helpStyle(color, "bold", "Daily Ops"))
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "start")+"     Start containers")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "stop")+"      Stop containers")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "restart")+"   Restart containers")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "rebuild")+"   Rebuild and start")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "update")+"     Pull, migrate, restart, and rebuild ahub")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "status")+"     Show container status")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "logs")+"       Follow live logs")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "shell")+"      Open a shell in the app container")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "backup")+"     Export a backup archive")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "restore <file>")+"  Import a backup archive")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "destroy")+"    Stop and remove volumes")
	fmt.Fprintln(w, "  "+helpStyle(color, "yellow", "uninstall")+"  Remove containers, image, volumes, and files")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, helpStyle(color, "bold", "Setup"))
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub setup --username admin --password '...' [--email ...] [--role owner]"))
	fmt.Fprintln(w, "    Bootstrap the initial admin account.")
	fmt.Fprintln(w, "  "+helpStyle(color, "green", "ahub set port 8080"))
	fmt.Fprintln(w, "    Update `APP_PORT` and `APP_BASE_URL` in `.env`.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, helpStyle(color, "bold", "Tips"))
	fmt.Fprintln(w, "  "+helpStyle(color, "dim", "Run commands from the install directory, or set AHUB_DIR."))
	fmt.Fprintln(w, "  "+helpStyle(color, "dim", "Use `ahub help` anytime to redisplay this guide."))
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func runCreateEnv(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("create env", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "overwrite existing .env")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := ".env"
	if !*force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force)", path)
		}
	}
	content := strings.ReplaceAll(envExampleTemplate, "REDIS_ADDR=127.0.0.1:6379", "REDIS_ADDR=127.0.0.1:6379")
	content = strings.ReplaceAll(content, "HUB_SECRET_KEY=replace_me", "HUB_SECRET_KEY="+randomSecret())
	content = strings.ReplaceAll(content, "APP_PORT=8080", "APP_PORT=8080")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "created .env")
	return nil
}

func runCreateDocker(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("create docker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "overwrite existing files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := writeFileIfNeeded("docker-compose.yml", dockerComposeTemplate, *force); err != nil {
		return err
	}
	if err := writeFileIfNeeded("Dockerfile", dockerfileTemplate, *force); err != nil {
		return err
	}
	if err := writeFileIfNeeded(".env.example", envExampleTemplate, *force); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "created docker files")
	return nil
}

func runSetPort(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return errors.New("port is required")
	}
	port, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid port")
	}
	if err := updateEnvFile(".env", map[string]string{
		"APP_PORT":    strconv.Itoa(port),
		"APP_BASE_URL": fmt.Sprintf("http://localhost:%d", port),
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "set port to %d\n", port)
	return nil
}

func runSetup(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "admin", "initial admin username")
	password := fs.String("password", "", "initial admin password")
	email := fs.String("email", "", "initial admin email")
	role := fs.String("role", "owner", "initial admin role")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*password) == "" {
		return errors.New("--password is required")
	}
	composeArgs := []string{"run", "--rm", "--no-deps", serviceName, "setup", "--username", *username, "--password", *password, "--email", *email, "--role", *role}
	return runCompose(composeArgs...)
}

func runRestore(args []string) error {
	if len(args) == 0 {
		return errors.New("restore file is required")
	}
	path := args[0]
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	_ = runCompose("stop", serviceName)
	defer func() { _ = runCompose("start", serviceName) }()
	return runCompose("run", "--rm", "--no-deps", "-v", filepath.Dir(abs)+":/restore", serviceName, "backup", "import", "--file", "/restore/"+filepath.Base(abs))
}

func runLogs(args []string) error {
	composeArgs, err := buildLogsComposeArgs(args)
	if err != nil {
		return err
	}
	return runCompose(composeArgs...)
}

func buildLogsComposeArgs(args []string) ([]string, error) {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tail := fs.Int("tail", 100, "number of lines to show")
	since := fs.String("since", "", "show logs since time or duration")
	follow := fs.Bool("follow", true, "follow output")
	noFollow := fs.Bool("no-follow", false, "do not follow output")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() != 0 {
		return nil, fmt.Errorf("logs does not take arguments")
	}
	composeArgs := []string{"logs", "--tail", strconv.Itoa(*tail)}
	if strings.TrimSpace(*since) != "" {
		composeArgs = append(composeArgs, "--since", strings.TrimSpace(*since))
	}
	if *follow && !*noFollow {
		composeArgs = append(composeArgs, "-f")
	}
	composeArgs = append(composeArgs, serviceName)
	return composeArgs, nil
}

func rebuildAhubBinary() error {
	installDir := defaultInstallDir()
	binDir := defaultBinDir()
	if _, err := os.Stat(filepath.Join(installDir, ".git")); err != nil {
		return nil
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	tmpBin := filepath.Join(binDir, "ahub.update.tmp")
	if err := buildAhubBinary(installDir, tmpBin); err != nil {
		return err
	}
	return os.Rename(tmpBin, filepath.Join(binDir, "ahub"))
}

func buildAhubBinary(installDir, outputPath string) error {
	if _, err := os.Stat(installDir); err != nil {
		return err
	}
	cacheDir := filepath.Join(os.TempDir(), "ahub-go-build")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command("docker", "run", "--rm",
			"-u", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			"-v", installDir+":/src",
			"-v", filepath.Dir(outputPath)+":/out",
			"-v", cacheDir+":/cache",
			"-e", "GOCACHE=/cache",
			"-w", "/src",
			"golang:1.22",
			"go", "build", "-buildvcs=false", "-o", "/out/"+filepath.Base(outputPath), "./cmd/ahub")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go or docker is required to rebuild ahub")
	}
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", outputPath, "./cmd/ahub")
	cmd.Dir = installDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runUninstall(stdout, stderr io.Writer) error {
	if err := runCompose("down", "-v", "--remove-orphans"); err != nil {
		return err
	}
	if ids, err := composeImageIDs(serviceName); err == nil {
		for _, id := range ids {
			if id == "" {
				continue
			}
			_ = runDocker("image", "rm", "-f", id)
		}
	}
	_ = runDocker("volume", "rm", "-f", "hub-data", "hub-backups")
	if err := os.RemoveAll(defaultInstallDir()); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(defaultBinDir(), "ahub"))
	fmt.Fprintln(stdout, "uninstalled ahub")
	return nil
}

func composeImageIDs(service string) ([]string, error) {
	cmd := exec.Command("docker", "compose", "images", "-q", service)
	cmd.Dir = composeWorkDir()
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	ids := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids, nil
}

func runDocker(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = "."
	return cmd.Run()
}

func defaultInstallDir() string {
	if base := os.Getenv("AHUB_DIR"); base != "" {
		return base
	}
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, "ahub")
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".local", "share", "ahub")
}

func defaultBinDir() string {
	if base := os.Getenv("AHUB_BIN_DIR"); base != "" {
		return base
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".local", "bin")
}

func runCompose(args ...string) error {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = composeWorkDir()
	return cmd.Run()
}

func tryGitPull() error {
	dir := composeWorkDir()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return nil
	}
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = dir
	return cmd.Run()
}

func composeWorkDir() string {
	dir := defaultInstallDir()
	if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
		return dir
	}
	return "."
}

func writeFileIfNeeded(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func updateEnvFile(path string, values map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	seen := make(map[string]bool, len(values))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		if value, ok := values[key]; ok {
			lines[i] = key + "=" + value
			seen[key] = true
		}
	}
	for key, value := range values {
		if seen[key] {
			continue
		}
		lines = append(lines, key+"="+value)
	}
	output := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	return os.WriteFile(path, []byte(output), 0o600)
}

func randomSecret() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "change-me"
	}
	return hex.EncodeToString(buf[:])
}

func helpColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := w.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	info, err := os.NewFile(file.Fd(), "").Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func helpStyle(enabled bool, style, text string) string {
	if !enabled {
		return text
	}
	switch style {
	case "bold":
		return "\033[1m" + text + "\033[0m"
	case "dim":
		return "\033[2m" + text + "\033[0m"
	case "green":
		return "\033[32m" + text + "\033[0m"
	case "yellow":
		return "\033[33m" + text + "\033[0m"
	case "cyan":
		return "\033[36m" + text + "\033[0m"
	default:
		return text
	}
}
