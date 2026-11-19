import { ref } from 'vue'
import { GetCommands, GetDefaultCommandName } from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'

// Shared so changes made in the settings dialog reach the open review page. null until first loaded.
export const commands = ref<model.CommandDTO[] | null>(null)
export const defaultCommandName = ref('')

export async function loadCommands(): Promise<void> {
  try {
    commands.value = await GetCommands()
  } catch {
    // Non-fatal: no run buttons are shown
  }
  try {
    defaultCommandName.value = await GetDefaultCommandName()
  } catch {
    // Non-fatal
  }
}
