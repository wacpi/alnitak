import { ref } from 'vue';
import { useMessage } from 'naive-ui';
import { sendEmailCodeAPI } from '@/api/code';
import { statusCode } from '@/utils/status-code';

export default function useSendCode() {

  const disabledSend = ref(false);//禁用发送按钮
  const sendBtnText = ref('发送验证码');//发送按钮文字
  const message = useMessage();//通知

  const startCountdown = (seconds: number) => {
    let remaining = seconds;
    disabledSend.value = true;
    sendBtnText.value = `${remaining}秒`;
    let tag = setInterval(() => {
      if (--remaining <= 0) {
        clearInterval(tag);
        disabledSend.value = false;
        sendBtnText.value = '发送验证码';
        return;
      }
      sendBtnText.value = `${remaining}秒`;
    }, 1000);
  }

  const sendEmailCodeAsync = async (email: string, captchaId: string) => {
    //禁用发送按钮
    disabledSend.value = true;
    try {
      const res = await sendEmailCodeAPI({ email, captchaId })
      if (res.data.code === statusCode.OK) {
        message.success(res.data.msg || '发送成功');
        //开启倒计时，使用后端返回的冷却时间
        startCountdown(res.data.data?.countdown || 60);
      } else if (res.data.code === statusCode.FAIL) {
        //如果后端返回了冷却时间（发送过于频繁），开启倒计时
        if (res.data.data?.countdown) {
          startCountdown(res.data.data.countdown);
          message.error(res.data.msg);
        } else {
          disabledSend.value = false;
        }
      } else {
        disabledSend.value = false;
      }
      return res.data;
    } catch {
      disabledSend.value = false;
      sendBtnText.value = '发送验证码';
      message.error('发送失败');
    }
  }

  return {
    disabledSend,
    sendBtnText,
    sendEmailCodeAsync
  }
}

