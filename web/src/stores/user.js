import { getMenusApi } from '~@/api/common/menu'
import { getAdminUserInfoApi } from '~@/api/common/user'
import { logoutApi } from '~@/api/common/login'
import { rootRoute } from '~@/router/constant'
import { generateFlatRoutes, generateRoutes, generateTreeRoutes } from '~@/router/generate-route'
import { DYNAMIC_LOAD_WAY, DynamicLoadEnum } from '~@/utils/constant'
import { clearAuthTokens, useRefreshToken } from '~/composables/authorization'

export const useUserStore = defineStore('user', () => {
  const routerData = shallowRef()
  const menuData = shallowRef([])
  const userInfo = shallowRef()
  const avatar = computed(() => userInfo.value?.avatar)
  const nickname = computed(() => userInfo.value?.nickname !== '' ? userInfo.value?.nickname : userInfo.value?.username)
  const roles = computed(() => userInfo.value?.roles)
  const getMenuRoutes = async () => {
    const { data } = await getMenusApi()
    return generateTreeRoutes(data.list ?? [])
  }
  const generateDynamicRoutes = async () => {
    const dynamicLoadWay = DYNAMIC_LOAD_WAY === DynamicLoadEnum.BACKEND ? getMenuRoutes : generateRoutes
    const { menuData: treeMenuData, routeData } = await dynamicLoadWay()
    menuData.value = treeMenuData
    routerData.value = {
      ...rootRoute,
      children: generateFlatRoutes(routeData),
    }
    return routerData.value
  }
  const getUserInfo = async () => {
    const { data } = await getAdminUserInfoApi()
    userInfo.value = data
  }
  // 登出以清理前端态为准；后端会话吊销失败不阻断用户退出。
  const logout = async () => {
    const refresh = useRefreshToken()
    const rt = refresh.value
    if (rt) {
      try {
        await logoutApi(rt)
      }
      catch (_) {
        // 后端吊销失败不影响本地退出流程。
      }
    }
    clearAuthTokens()
    userInfo.value = void 0
    routerData.value = void 0
    menuData.value = []
    window.location.href = '/login'
  }
  return {
    userInfo,
    roles,
    getUserInfo,
    logout,
    routerData,
    menuData,
    generateDynamicRoutes,
    avatar,
    nickname,
  }
})
