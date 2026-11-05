package monitor

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type dockerContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

func collectDocker(ctx context.Context) ([]Workload, bool) {
	socket := dockerSocket()
	if socket == "" {
		return nil, false
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 350 * time.Millisecond}).DialContext(ctx, "unix", socket)
		},
	}
	client := &http.Client{Transport: transport, Timeout: 700 * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/v1.43/containers/json?all=0", nil)
	if err != nil {
		return nil, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	var containers []dockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, false
	}
	result := make([]Workload, 0, len(containers))
	for _, container := range containers {
		name := strings.TrimPrefix(firstNonEmpty(container.Names...), "/")
		if project := container.Labels["com.docker.compose.project"]; project != "" {
			service := container.Labels["com.docker.compose.service"]
			name = "docker compose · " + project
			if service != "" {
				name += "/" + service
			}
		}
		if name == "" {
			name = "container · " + shortID(container.ID)
		}
		started := time.Now().Add(-time.Minute).UnixMilli()
		node := ProcessNode{
			ID:              "docker-" + container.ID,
			Name:            name,
			Command:         "docker container " + shortID(container.ID) + " · " + container.Image,
			Kind:            "container",
			Depth:           0,
			Protected:       true,
			ProtectedReason: "容器停止需要独立确认，MVP 不包含批量停止",
			StartTimeMs:     started,
		}
		result = append(result, Workload{
			ID:              "docker-" + container.ID,
			Name:            name,
			Kind:            "container",
			Source:          "Docker",
			ProcessCount:    1,
			Risk:            "healthy",
			RiskLabel:       "运行中",
			RiskDetail:      firstNonEmpty(container.Status, "容器运行正常"),
			Confidence:      96,
			Protected:       true,
			ProtectedReason: "容器停止需要独立确认，MVP 不包含批量停止",
			StartedAt:       started,
			LastActivity:    "正在运行",
			Trend:           make([]float64, 24),
			Evidence: []Evidence{
				{Label: "容器状态", Detail: firstNonEmpty(container.Status, container.State), Tone: "healthy"},
				{Label: "镜像", Detail: container.Image, Tone: "neutral"},
			},
			Processes:      []ProcessNode{node},
			ContainerNames: []string{strings.TrimPrefix(name, "/")},
		})
	}
	return result, true
}
func dockerSocket() string {
	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		return strings.TrimPrefix(host, "unix://")
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".docker", "run", "docker.sock"),
		"/var/run/docker.sock",
		"/run/docker.sock",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode()&os.ModeSocket != 0 {
			return candidate
		}
	}
	return ""
}
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	if id == "" {
		return "unknown"
	}
	return id
}
