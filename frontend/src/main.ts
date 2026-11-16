import {createApp} from 'vue'
import App from './App.vue'
import './style.css';
import 'vue-sonner/style.css'
import { applyAppearance, loadAppearance } from './composables/useAppearance'
import { loadToggles } from './composables/useToggles'

// main.go asks for this where the webview crashes on view transitions; every caller falls back when the method is missing.
if (new URLSearchParams(location.search).has('no-view-transitions')) delete (Document.prototype as Partial<Document>).startViewTransition

applyAppearance()
// Mount after the saved fonts and toggles are known so the UI doesn't re-flow and toggles start from their saved values.
Promise.allSettled([loadAppearance(), loadToggles()]).finally(() => createApp(App).mount('#app'))
