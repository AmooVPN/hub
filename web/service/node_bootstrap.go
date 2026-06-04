package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/v3/database/model"
	"github.com/drunkleen/l-ui/v3/util/common"
	"github.com/drunkleen/l-ui/v3/util/random"
	"golang.org/x/crypto/ssh"
)

type BootstrapStep struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
}

type NodeBootstrapRequest struct {
	Name          string `json:"name" form:"name" validate:"required"`
	Remark        string `json:"remark" form:"remark"`
	Address       string `json:"address" form:"address" validate:"required"`
	SSHUser       string `json:"sshUser" form:"sshUser" validate:"required"`
	SSHPassword   string `json:"sshPassword" form:"sshPassword" validate:"required"`
	SSHPort       int    `json:"sshPort" form:"sshPort" validate:"omitempty,gte=1,lte=65535"`
	AgentPort     int    `json:"agentPort" form:"agentPort" validate:"omitempty,gte=1,lte=65535"`
	BootstrapBase string `json:"bootstrapBase,omitempty" form:"bootstrapBase"`
}

type NodeBootstrapResult struct {
	Node  *model.Node     `json:"node"`
	Steps []BootstrapStep `json:"steps"`
}

func sshQuote(s string) string { return shellQuote(s) }

func sshAddress(host string, port int) string {
	if port <= 0 {
		port = 22
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func runSSHCommand(client *ssh.Client, password, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var stdin io.WriteCloser
	if password != "" {
		stdin, err = sess.StdinPipe()
		if err != nil {
			return "", err
		}
	}
	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out

	remoteCmd := cmd
	if password != "" {
		remoteCmd = "sudo -S -p '' sh -lc " + sshQuote(cmd)
	}
	if err := sess.Start(remoteCmd); err != nil {
		return out.String(), err
	}
	if stdin != nil {
		_, _ = io.WriteString(stdin, password+"\n")
		_ = stdin.Close()
	}
	if err := sess.Wait(); err != nil {
		return out.String(), err
	}
	return strings.TrimSpace(out.String()), nil
}

func sshBootstrapArch(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	case "armv7l", "armv7":
		return "armv7", nil
	case "armv6l", "armv6":
		return "armv6", nil
	case "armv5tel", "armv5":
		return "armv5", nil
	case "i386", "i686", "386":
		return "386", nil
	case "s390x":
		return "s390x", nil
	default:
		return "", common.NewError("unsupported remote architecture: " + raw)
	}
}

func (s *NodeService) Bootstrap(ctx context.Context, req NodeBootstrapRequest) (*NodeBootstrapResult, error) {
	if req.Address == "" {
		return nil, common.NewError("node address is required")
	}
	if req.AgentPort <= 0 {
		req.AgentPort = 2053
	}
	if req.SSHPort <= 0 {
		req.SSHPort = 22
	}
	node := model.Node{
		Name:                req.Name,
		Remark:              req.Remark,
		Address:             req.Address,
		Scheme:              "http",
		Port:                req.AgentPort,
		BasePath:            "/",
		Enable:              true,
		AllowPrivateAddress: true,
		TlsVerifyMode:       "verify",
		ApiToken:            random.Seq(48),
	}
	if req.BootstrapBase != "" {
		node.BasePath = normalizeBasePath(req.BootstrapBase)
	}
	if err := s.normalize(&node); err != nil {
		return nil, err
	}
	node.Scheme = "http"
	node.Port = req.AgentPort
	node.BasePath = "/"
	node.ApiToken = strings.TrimSpace(node.ApiToken)

	addr := sshAddress(req.Address, req.SSHPort)
	sshCfg := &ssh.ClientConfig{
		User:            req.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.Password(req.SSHPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}
	conn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh connect to %s: %w", addr, err)
	}
	defer conn.Close()

	steps := make([]BootstrapStep, 0, 8)
	addStep := func(name string, ok bool, output string) {
		steps = append(steps, BootstrapStep{Name: name, OK: ok, Output: strings.TrimSpace(output)})
	}

	stepCmds := []struct {
		name string
		cmd  string
	}{
		{"detect-arch", "uname -m"},
		{"detect-release", `tag=$(curl -fsSL https://api.github.com/repos/drunkleen/l-ui/releases/latest | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | cut -d'"' -f4); [ -n "$tag" ] && printf '%s' "$tag"`},
		{"prepare-dirs", "mkdir -p /usr/local/l-ui /etc/l-ui /var/log/l-ui"},
	}

	var arch string
	var tag string
	for _, step := range stepCmds {
		out, err := runSSHCommand(conn, req.SSHPassword, step.cmd)
		if err != nil {
			addStep(step.name, false, out+"\n"+err.Error())
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
		addStep(step.name, true, out)
		switch step.name {
		case "detect-arch":
			arch, err = sshBootstrapArch(out)
			if err != nil {
				addStep("map-arch", false, err.Error())
				return &NodeBootstrapResult{Node: &node, Steps: steps}, err
			}
			addStep("map-arch", true, arch)
		case "detect-release":
			tag = strings.TrimSpace(out)
			if tag == "" {
				err = errors.New("latest release tag not found")
				addStep("release-tag", false, err.Error())
				return &NodeBootstrapResult{Node: &node, Steps: steps}, err
			}
			addStep("release-tag", true, tag)
		}
	}

	downloadCmd := fmt.Sprintf(`set -e
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
curl -fL -o "$tmpdir/l-ui.tar.gz" https://github.com/drunkleen/l-ui/releases/download/%s/l-ui-linux-%s.tar.gz
	rm -rf /usr/local/l-ui
	mkdir -p /usr/local
	tar -xzf "$tmpdir/l-ui.tar.gz" -C /usr/local
	chmod +x /usr/local/l-ui/l-ui
`, tag, arch)
	out, err := runSSHCommand(conn, req.SSHPassword, downloadCmd)
	if err != nil {
		addStep("download-install", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("download-install", true, out)

	envCmd := fmt.Sprintf(`cat >/etc/default/l-ui <<'EOF'
LUI_DB_FOLDER=/etc/l-ui
LUI_MAIN_FOLDER=/usr/local/l-ui
LUI_SERVICE=/etc/systemd/system
LUI_BOOTSTRAP_API_TOKEN=%s
	EOF`, node.ApiToken)
	out, err = runSSHCommand(conn, req.SSHPassword, envCmd)
	if err != nil {
		addStep("write-env", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("write-env", true, out)

	serviceCmd := `cat >/etc/systemd/system/l-ui.service <<'EOF'
[Unit]
Description=l-ui node agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/usr/local/l-ui
EnvironmentFile=-/etc/default/l-ui
ExecStart=/usr/local/l-ui/l-ui agent
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF`
	out, err = runSSHCommand(conn, req.SSHPassword, serviceCmd)
	if err != nil {
		addStep("write-service", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("write-service", true, out)

	startCmd := `systemctl daemon-reload && systemctl enable --now l-ui`
	out, err = runSSHCommand(conn, req.SSHPassword, startCmd)
	if err != nil {
		addStep("start-service", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("start-service", true, out)

	verifyCmd := fmt.Sprintf(`for i in $(seq 1 20); do
  if curl -fsS -H 'Authorization: Bearer %s' http://127.0.0.1:%d/panel/api/server/status >/tmp/lui-bootstrap-status.json; then
    cat /tmp/lui-bootstrap-status.json
    exit 0
  fi
  sleep 2
done
exit 1`, node.ApiToken, req.AgentPort)
	out, err = runSSHCommand(conn, req.SSHPassword, verifyCmd)
	if err != nil {
		addStep("verify-agent", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("verify-agent", true, out)

	if err := s.Create(&node); err != nil {
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	return &NodeBootstrapResult{Node: &node, Steps: steps}, nil
}
