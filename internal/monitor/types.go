package monitor

// Snapshot is the complete read model consumed by the desktop UI.
type Snapshot struct {
	GeneratedAt  int64         `json:"generatedAt"`
	Mode         string        `json:"mode"`
	Revision     string        `json:"revision"`
	Summary      Summary       `json:"summary"`
	System       SystemMetrics `json:"system"`
	Thermal      ThermalStatus `json:"thermal"`
	Workloads    []Workload    `json:"workloads"`
	Events       []Event       `json:"events"`
	Capabilities Capabilities  `json:"capabilities"`
}

type Summary struct {
	PressureLabel       string `json:"pressureLabel"`
	PressureDetail      string `json:"pressureDetail"`
	TopWorkloadID       string `json:"topWorkloadId"`
	TopWorkloadName     string `json:"topWorkloadName"`
	ReclaimBytes        uint64 `json:"reclaimBytes"`
	ReclaimProcessCount int    `json:"reclaimProcessCount"`
	TotalRSSBytes       uint64 `json:"totalRssBytes"`
	ActiveWorkloads     int    `json:"activeWorkloads"`
	AttentionCount      int    `json:"attentionCount"`
	ReclaimCount        int    `json:"reclaimCount"`
	Confidence          int    `json:"confidence"`
}

type SystemMetrics struct {
	CPUPercent     float64 `json:"cpuPercent"`
	MemoryUsed     uint64  `json:"memoryUsed"`
	MemoryTotal    uint64  `json:"memoryTotal"`
	SwapUsed       uint64  `json:"swapUsed"`
	SwapTotal      uint64  `json:"swapTotal"`
	ProcessCount   int     `json:"processCount"`
	AgentSessions  int     `json:"agentSessions"`
	ContainerCount int     `json:"containerCount"`
}

type Capabilities struct {
	CanStop       bool   `json:"canStop"`
	Docker        bool   `json:"docker"`
	Elevated      bool   `json:"elevated"`
	Platform      string `json:"platform"`
	CollectionMS  int64  `json:"collectionMs"`
	LiveSupported bool   `json:"liveSupported"`
}

type Workload struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Kind            string        `json:"kind"`
	Source          string        `json:"source"`
	CPUPercent      float64       `json:"cpuPercent"`
	RSSBytes        uint64        `json:"rssBytes"`
	ProcessCount    int           `json:"processCount"`
	AgeSeconds      int64         `json:"ageSeconds"`
	Risk            string        `json:"risk"`
	RiskLabel       string        `json:"riskLabel"`
	RiskDetail      string        `json:"riskDetail"`
	Confidence      int           `json:"confidence"`
	Reclaimable     bool          `json:"reclaimable"`
	Protected       bool          `json:"protected"`
	ProtectedReason string        `json:"protectedReason"`
	StartedAt       int64         `json:"startedAt"`
	LastActivity    string        `json:"lastActivity"`
	Trend           []float64     `json:"trend"`
	Evidence        []Evidence    `json:"evidence"`
	Processes       []ProcessNode `json:"processes"`
	ContainerNames  []string      `json:"containerNames"`
}

type Evidence struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Tone   string `json:"tone"`
}

type ProcessNode struct {
	ID              string  `json:"id"`
	PID             int32   `json:"pid"`
	PPID            int32   `json:"ppid"`
	Name            string  `json:"name"`
	Command         string  `json:"command"`
	RSSBytes        uint64  `json:"rssBytes"`
	CPUPercent      float64 `json:"cpuPercent"`
	AgeSeconds      int64   `json:"ageSeconds"`
	Depth           int     `json:"depth"`
	Kind            string  `json:"kind"`
	Protected       bool    `json:"protected"`
	ProtectedReason string  `json:"protectedReason"`
	StartTimeMs     int64   `json:"startTimeMs"`
}

type Event struct {
	ID           string `json:"id"`
	Time         int64  `json:"time"`
	Kind         string `json:"kind"`
	WorkloadID   string `json:"workloadId"`
	WorkloadName string `json:"workloadName"`
	Title        string `json:"title"`
	Detail       string `json:"detail"`
	Tone         string `json:"tone"`
}

type ActionTarget struct {
	PID         int32  `json:"pid"`
	StartTimeMs int64  `json:"startTimeMs"`
	Name        string `json:"name"`
	RSSBytes    uint64 `json:"rssBytes"`
}

type ProtectedProcess struct {
	PID    int32  `json:"pid"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ActionPlan struct {
	Token          string             `json:"token"`
	WorkloadID     string             `json:"workloadId"`
	WorkloadName   string             `json:"workloadName"`
	Mode           string             `json:"mode"`
	CreatedAtUnix  int64              `json:"createdAtUnix"`
	ExpiresAtUnix  int64              `json:"expiresAtUnix"`
	Targets        []ActionTarget     `json:"targets"`
	Protected      []ProtectedProcess `json:"protected"`
	EstimatedRSS   uint64             `json:"estimatedRssBytes"`
	EstimatedCPU   float64            `json:"estimatedCpuPercent"`
	ContainerNames []string           `json:"containerNames"`
	Warnings       []string           `json:"warnings"`
}

// ActionConfirmation is issued only after the user explicitly confirms a
// read-only preview. A preview token can never be passed to Execute directly.
type ActionConfirmation struct {
	Token           string `json:"token"`
	WorkloadID      string `json:"workloadId"`
	WorkloadName    string `json:"workloadName"`
	TargetCount     int    `json:"targetCount"`
	EstimatedRSS    uint64 `json:"estimatedRssBytes"`
	ConfirmedAtUnix int64  `json:"confirmedAtUnix"`
	ExpiresAtUnix   int64  `json:"expiresAtUnix"`
}

type ActionFailure struct {
	PID    int32  `json:"pid"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ActionResult struct {
	Token          string          `json:"token"`
	WorkloadID     string          `json:"workloadId"`
	WorkloadName   string          `json:"workloadName"`
	Mode           string          `json:"mode"`
	Stopped        []int32         `json:"stopped"`
	Failed         []ActionFailure `json:"failed"`
	Pending        []int32         `json:"pending"`
	Message        string          `json:"message"`
	ReclaimedBytes uint64          `json:"reclaimedBytes"`
	ConfirmedAt    int64           `json:"confirmedAt"`
	FinishedAt     int64           `json:"finishedAt"`
}

// AIStatus advertises whether explanations use local rules or an explicitly
// configured OpenAI-compatible endpoint.
type AIStatus struct {
	Mode            string `json:"mode"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	BaseURL         string `json:"baseUrl"`
	Remote          bool   `json:"remote"`
	Privacy         string `json:"privacy"`
	Configured      bool   `json:"configured"`
	APIKeyAvailable bool   `json:"apiKeyAvailable"`
	Source          string `json:"source"`
	Restoring       bool   `json:"restoring"`
}

// AIProviderInput is the non-secret and secret material accepted by the
// settings dialog. APIKey is never returned by any read method.
type AIProviderInput struct {
	Provider    string `json:"provider"`
	BaseURL     string `json:"baseUrl"`
	Model       string `json:"model"`
	APIKey      string `json:"apiKey"`
	AllowRemote bool   `json:"allowRemote"`
}

// AISettings is the credential-free view rendered by the settings dialog.
type AISettings struct {
	Provider    string `json:"provider"`
	BaseURL     string `json:"baseUrl"`
	Model       string `json:"model"`
	AllowRemote bool   `json:"allowRemote"`
	HasAPIKey   bool   `json:"hasApiKey"`
	Source      string `json:"source"`
	Restoring   bool   `json:"restoring"`
}

// AITestResult reports a minimal connection probe without echoing credentials
// or response bodies.
type AITestResult struct {
	OK        bool   `json:"ok"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latencyMs"`
	Message   string `json:"message"`
	Remote    bool   `json:"remote"`
}

// AIAnswer is a read-only explanation. It intentionally has no action token or
// executable command; the UI can only route a follow-up through Preview.
type AIAnswer struct {
	Title          string   `json:"title"`
	Facts          []string `json:"facts"`
	Inference      string   `json:"inference"`
	Recommendation string   `json:"recommendation"`
	Uncertainty    string   `json:"uncertainty"`
	Source         string   `json:"source"`
	Model          string   `json:"model"`
	Note           string   `json:"note"`
}
