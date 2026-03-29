package config

type Security struct {
	AccessJwtSecret          string   `mapstructure:"access_jwt_secret" json:"access_jwt_secret" yaml:"access_jwt_secret"`
	RefreshJwtSecret         string   `mapstructure:"refresh_jwt_secret" json:"refresh_jwt_secret" yaml:"refresh_jwt_secret"`
	PlayJwtSecret            string   `mapstructure:"play_jwt_secret" json:"play_jwt_secret" yaml:"play_jwt_secret"`                         // 播放授权 JWT
	StreamJwtSecret          string   `mapstructure:"stream_jwt_secret" json:"stream_jwt_secret" yaml:"stream_jwt_secret"`                 // 分片流 URL JWT（可与 play 分离轮换）
	PlayAllowedRefererHosts  []string `mapstructure:"play_allowed_referer_hosts" json:"play_allowed_referer_hosts" yaml:"play_allowed_referer_hosts"` // 可选：非空则校验 Referer Host
	PlayAllowedCIDRs         []string `mapstructure:"play_allowed_cidrs" json:"play_allowed_cidrs" yaml:"play_allowed_cidrs"`                       // 可选：客户端 IP 须落入其一（CIDR）
	CloseRecordUserOperation bool     `mapstructure:"close_record_user_operation" json:"close_record_user_operation" yaml:"close_record_user_operation"`
}
