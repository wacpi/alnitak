package mail

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"

	"interastral-peace.com/alnitak/internal/global"
)

type Message struct {
	Host     string
	Port     int
	Username string
	Password string
	header   map[string]string
	body     string
}

func NewMessage() *Message {
	m := &Message{
		header: make(map[string]string),
	}

	return m
}

func (m *Message) SetHeader(field string, value string) {
	m.header[field] = value
}

func (m *Message) SetBody(value string) {
	m.body = value
}

// 生成邮件内容
func (m *Message) GenerateMessage() (message string) {
	for k, v := range m.header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + m.body
	return
}

func (m *Message) SetDialer(host string, port int, username, password string) {
	m.Host = host
	m.Port = port
	m.Username = username
	m.Password = password
}

func (m *Message) DialAndSend(fromEmail, toEmail string) error {
	msg := []byte(m.GenerateMessage())
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)

	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)

	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		return err
	}

	//分解主机端口字符串
	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()

	if ok, _ := c.Extension("AUTH"); ok {
		if err = c.Auth(auth); err != nil {
			return err
		}
	}

	// 使用发件邮箱地址而不是登录用户名
	if err = c.Mail(fromEmail); err != nil {
		return err
	}

	if err = c.Rcpt(toEmail); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(msg); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return c.Quit()
}

/**
 * 发送邮箱验证码
 * param: email 目标邮箱
 * param: code 邮箱验证码
 * return: 发送失败时的错误信息
 */
func SendCaptcha(email string, code string) error {
	// 定义收件人
	mailTo := email
	// 邮件主题
	subject := "验证码"
	// 邮件正文
	body := `
<div style="max-width: 500px; margin: 0 auto; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;">
  <div style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px 20px; text-align: center; border-radius: 12px 12px 0 0;">
    <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 600;">邮箱验证码</h1>
  </div>
  <div style="background: #ffffff; padding: 40px 30px; border: 1px solid #e8e8e8; border-top: none;">
    <p style="color: #333333; font-size: 16px; line-height: 1.6; margin: 0 0 25px 0;">尊敬的用户，您好！</p>
    <p style="color: #666666; font-size: 14px; line-height: 1.6; margin: 0 0 25px 0;">您正在进行邮箱验证操作，验证码为：</p>
    <div style="background: #f8f9fa; border-radius: 8px; padding: 20px; text-align: center; margin: 0 0 25px 0;">
      <span style="font-size: 32px; font-weight: bold; color: #667eea; letter-spacing: 8px;">` + code + `</span>
    </div>
    <p style="color: #999999; font-size: 13px; line-height: 1.6; margin: 0;">此验证码 <strong style="color: #e74c3c;">5 分钟</strong> 内有效，请勿泄露给他人。</p>
  </div>
  <div style="background: #f8f9fa; padding: 20px; text-align: center; border-radius: 0 0 12px 12px; border: 1px solid #e8e8e8; border-top: none;">
    <p style="color: #999999; font-size: 12px; margin: 0;">如非本人操作，请忽略此邮件</p>
  </div>
</div>`
	return Send(mailTo, subject, body)
}

/**
 * 发送电子邮件
 * param: emailList 目标邮箱数组
 * param: subject 邮件主题
 * param: body 邮件内容
 * return: 发送失败时的错误信息
 */
func Send(email string, subject string, body string) error {
	m := NewMessage()
	// 使用独立的发件邮箱地址，如果未配置则回退到登录用户名
	fromEmail := global.Config.Mail.From
	if fromEmail == "" {
		fromEmail = global.Config.Mail.User
	}
	m.SetHeader("From", global.Config.Mail.Addresser+" "+"<"+fromEmail+">")   //添加别名
	m.SetHeader("To", email)                                                  //发送给多个用户
	m.SetHeader("Subject", subject)                                           //设置邮件主题
	m.SetHeader("MIME-Version", "1.0")                                        //MIME版本
	m.SetHeader("Content-Type", "text/html; charset=UTF-8")                   //内容类型为HTML
	m.SetBody(body)                                                           //设置邮件正文

	m.SetDialer(global.Config.Mail.Host, global.Config.Mail.Port, global.Config.Mail.User, global.Config.Mail.Pass)

	err := m.DialAndSend(fromEmail, email)
	return err
}
