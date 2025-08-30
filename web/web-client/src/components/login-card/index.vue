<template>
  <div class="login-card">
    <div class="card-close" v-show="props.close" @click="closeClick">
      <close></close>
    </div>
    <div class="card-left">
      <div class="illustrations">
        <login-illustration></login-illustration>
      </div>
    </div>
    <div class="card-right">
      <login-form v-if="isLogin" @change-form="isLogin = false" @success="success"></login-form>
      <register-form v-else @change-form="isLogin = true" @success="success"></register-form>
    </div>
    <p class="protocol-container">
      <span>注册或登录即代表同意 </span>
      <span class="protocol">《用户协议》</span>
      <span>和</span>
      <span class="protocol">《隐私政策》</span>
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { Close } from '@icon-park/vue-next';
import LoginIllustration from "./components/LoginIllustration.vue";
import LoginForm from './components/LoginForm.vue'
import RegisterForm from './components/RegisterForm.vue'

const emits = defineEmits(["close", "success"]);
const props = withDefaults(defineProps<{
  close?: boolean,
}>(), {
  close: false
});

// 是否为登录
const isLogin = ref(true);

const closeClick = () => {
  emits("close");
}

const success = () => {
  emits("success");
}
</script>

<style lang="scss" scoped>
.login-card {
  position: absolute;
  display: grid;
  grid-template-columns: 330px auto;
  grid-template-rows: auto 60px;
  top: 50%;
  left: 50%;
  margin: -200px 0 0 -400px;
  width: 800px;
  height: 380px;
  overflow: hidden;
  /* LoginCard：卡片主体采用提层背景与主题阴影，适配深浅色 */
  background-color: var(--bg-elev-1);
  border-radius: 20px;
  box-shadow: 16px 16px 50px -12px var(--shadow-weak);
  color: var(--font-primary-1);

  .card-close {
    right: 26px;
    top: 16px;
    cursor: pointer;
    position: absolute;
    width: 30px;
    height: 30px;

    &:hover {
      color: var(--font-primary-3);
    }
  }

  // 插画部分
  .card-left {
    width: 330px;
    grid-row: 1 / span 2;
    /* LoginCard：左侧插画区使用主题主色 */
    background-color: var(--primary-color);
    -webkit-clip-path: polygon(98% 17%, 100% 34%, 98% 51%, 100% 68%, 98% 84%, 100% 100%, 0 100%, 0 0, 100% 0);
    clip-path: polygon(98% 17%, 100% 34%, 98% 51%, 100% 68%, 98% 84%, 100% 100%, 0 100%, 0 0, 100% 0);

    .illustrations {
      height: 100%;
      padding: 100px 30px 0 30px;
    }
  }

  // 表单部分
  .card-right {
    padding: 40px 30px 0 0;
  }

  //用户协议
  .protocol-container {
    grid-column: 2;
    color: var(--font-primary-3);
    text-align: center;
    font-size: 12px;

    .protocol {
      cursor: pointer;
      color: var(--primary-color);
      
      &:hover {
        color: var(--primary-hover-color);
      }
    }
  }
}


.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.fade-enter-to,
.fade-leave-from {
  opacity: 1;
}

.fade-enter-active,
.fade-leave-active {
  transition: all .5s ease;
}
</style>