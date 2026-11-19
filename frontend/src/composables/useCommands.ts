import { ref } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'

// Shared so changes made in the settings dialog reach the open review page. null until first loaded.
export const commands = ref<model.CommandDTO[] | null>(null)
export const defaultCommandName = ref('')
