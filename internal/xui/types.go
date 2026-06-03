package xui

type XUIInbound struct {
	ID       int64                  `json:"id"`
	Remark   string                 `json:"remark,omitempty"`
	Protocol string                 `json:"protocol,omitempty"`
	Port     int                    `json:"port,omitempty"`
	Network  string                 `json:"network,omitempty"`
	Security string                 `json:"security,omitempty"`
	Enabled  bool                   `json:"enabled"`
	RawJSON  map[string]any         `json:"raw_json,omitempty"`
	Extra    map[string]interface{} `json:"extra,omitempty"`
}

type XUIClientCreateRequest struct {
	Email       string                 `json:"email,omitempty"`
	ID          string                 `json:"id,omitempty"`
	Flow        string                 `json:"flow,omitempty"`
	LimitIP     int                    `json:"limit_ip,omitempty"`
	TotalGB     int64                  `json:"total_gb,omitempty"`
	ExpiryTime  int64                  `json:"expiry_time,omitempty"`
	Enable      bool                   `json:"enable"`
	RawSettings map[string]interface{} `json:"raw_settings,omitempty"`
}

type XUIClientUpdateRequest struct {
	Email      string `json:"email,omitempty"`
	Flow       string `json:"flow,omitempty"`
	LimitIP    int    `json:"limit_ip,omitempty"`
	TotalGB    int64  `json:"total_gb,omitempty"`
	ExpiryTime int64  `json:"expiry_time,omitempty"`
	Enable     bool   `json:"enable"`
}

type XUIClientResult struct {
	ID     string `json:"id,omitempty"`
	Email  string `json:"email,omitempty"`
	Enable bool   `json:"enable"`
}

type XUITraffic struct {
	Upload   int64 `json:"upload,omitempty"`
	Download int64 `json:"download,omitempty"`
	Total    int64 `json:"total,omitempty"`
}

type XUICompatibility struct {
	Version                     string
	SupportsClientEnableDisable bool
	SupportsTrafficReset        bool
	SupportsOnlineUsers         bool
	SupportsSubscriptionAPI     bool
}
