// 路由守卫以 refresh token 判断登录态；access 缺失或过期由请求拦截器静默刷新。
import router from '~/router'
import { useMetaTitle } from '~/composables/meta-title'
import { clearAuthTokens, useRefreshToken } from '~/composables/authorization'
import { setRouteEmitter } from '~@/utils/route-listener'

const allowList = ['/login', '/error', '/401', '/404', '/403']
const loginPath = '/login'
router.beforeEach(async (to, _, next) => {
  setRouteEmitter(to)
  const userStore = useUserStore()
  const refresh = useRefreshToken()
  if (!refresh.value) {
    if (!allowList.includes(to.path) && !to.path.startsWith('/redirect')) {
      next({
        path: loginPath,
        query: {
          redirect: encodeURIComponent(to.fullPath),
        },
      })
      return
    }
  }
  else {
    if (!userStore.userInfo && !allowList.includes(to.path) && !to.path.startsWith('/redirect')) {
      try {
        // getUserInfo 走统一请求实例，access 过期时由响应拦截器刷新。
        await userStore.getUserInfo()
        const currentRoute = await userStore.generateDynamicRoutes()
        router.addRoute(currentRoute)
        next({
          ...to,
          replace: true,
        })
        return
      }
      catch (e) {
        if (!refresh.value || e?.response?.status === 401) {
          clearAuthTokens()
          next({
            path: loginPath,
            query: {
              redirect: encodeURIComponent(to.fullPath),
            },
          })
          return
        }
        // 非认证异常交给目标页面处理，避免路由层吞掉页面级错误展示。
      }
    }
    else {
      if (to.path === loginPath) {
        next({
          path: '/',
        })
        return
      }
    }
  }
  next()
})
router.afterEach((to) => {
  useMetaTitle(to)
  useLoadingCheck()
  useScrollToTop()
})
