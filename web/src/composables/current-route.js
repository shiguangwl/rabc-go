import router from '@/router'

export function useCurrentRoute() {
  const currentRoute = router.currentRoute
  return {
    currentRoute,
  }
}
