import {createApp} from 'vue'
import App from './App.vue'
import './style.css';
import 'vue-sonner/style.css'
import { applyAppearance, loadAppearance } from './composables/useAppearance'

applyAppearance()
// Mount after the saved fonts are known so the UI doesn't re-flow on start.
loadAppearance().finally(() => createApp(App).mount('#app'))
