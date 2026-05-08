import {getMenusApi} from '~@/api/common/menu'
import {getAdminUserInfoApi} from '~@/api/common/user'
import {rootRoute} from '~@/router/constant'
import {generateFlatRoutes, generateRoutes, generateTreeRoutes} from '~@/router/generate-route'
import {DYNAMIC_LOAD_WAY, DynamicLoadEnum} from '~@/utils/constant'

export const useUserStore = defineStore('user', () => {
    const routerData = shallowRef()
    const menuData = shallowRef([])
    const userInfo = shallowRef()
    const token = useAuthorization()
    const avatar = computed(() => userInfo.value?.avatar)
    const nickname = computed(() => userInfo.value?.nickname !== "" ? userInfo.value?.nickname : userInfo.value?.username)
    const roles = computed(() => userInfo.value?.roles)
    const getMenuRoutes = async () => {
        const {data} = await getMenusApi()
        return generateTreeRoutes(data.list ?? [])
    }
    const generateDynamicRoutes = async () => {
        const dynamicLoadWay = DYNAMIC_LOAD_WAY === DynamicLoadEnum.BACKEND ? getMenuRoutes : generateRoutes
        const {menuData: treeMenuData, routeData} = await dynamicLoadWay()
        menuData.value = treeMenuData
        routerData.value = {
            ...rootRoute,
            children: generateFlatRoutes(routeData),
        }
        return routerData.value
    }
    const getUserInfo = async () => {
        const {data} = await getAdminUserInfoApi()
        userInfo.value = data
    }
    const logout = async () => {
        token.value = null
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
