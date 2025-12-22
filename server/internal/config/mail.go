package config

type Mail struct {
	Addresser string `mapstructure:"addresser" json:"addresser" yaml:"addresser"` // 发件人名称
	Host      string `mapstructure:"host" json:"host" yaml:"host"`                // SMTP服务器地址
	Port      int    `mapstructure:"port" json:"port" yaml:"port"`                // SMTP端口
	User      string `mapstructure:"user" json:"user" yaml:"user"`                // SMTP登录用户名
	From      string `mapstructure:"from" json:"from" yaml:"from"`                // 发件邮箱地址
	Pass      string `mapstructure:"pass" json:"pass" yaml:"pass"`                // SMTP密码/授权码
	Debug     bool   `mapstructure:"debug" json:"debug" yaml:"debug"`             // 调试模式
}
