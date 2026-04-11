package config

type Server struct {
	Port     string `mapstructure:"port" json:"port" yaml:"port"`                // HTTP端口
	Ssl      Ssl    `mapstructure:"ssl" json:"ssl" yaml:"ssl"`                   // SSL配置
}

type Ssl struct {
	Enabled  bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`       // 是否启用HTTPS
	Port     string `mapstructure:"port" json:"port" yaml:"port"`                // HTTPS端口
	CertFile string `mapstructure:"cert_file" json:"cert_file" yaml:"cert_file"` // 证书文件路径
	KeyFile  string `mapstructure:"key_file" json:"key_file" yaml:"key_file"`    // 私钥文件路径
}
