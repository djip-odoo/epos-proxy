import './app.css'
import App from './App.vue'
import { createApp } from 'vue'
import { useDocs } from './modal/use-docs'

useDocs()

const app = createApp(App)
app.mount('#app')