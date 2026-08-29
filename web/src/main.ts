import { createApp } from 'vue'
import App from './App.vue'
// 自托管 Archivo Variable(拉丁/数字);中文字符按 unicode-range 自动回落系统字族
import '@fontsource-variable/archivo'
import './style.css'

createApp(App).mount('#app')

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    void navigator.serviceWorker.register('/sw.js')
  })
}
