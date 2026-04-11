export type ToastType = 'success' | 'error' | 'info'

export function toast(type: ToastType, message: string) {
  window.dispatchEvent(
    new CustomEvent('pgc-studio-toast', {
      detail: { type, message },
    }),
  )
}

