import axios from 'axios'
import { AxiosLoading } from './loading.js'
import { singleFlightRefresh } from './refresh-singleton.js'
import {
  STORAGE_AUTHORIZE_KEY,
  clearAuthTokens,
  useAccessToken,
  useRefreshToken,
} from '~/composables/authorization'
import { ContentTypeEnum, RequestEnum } from '~#/http-enum'
import router from '~/router'

const instance = axios.create({
  baseURL: import.meta.env.VITE_APP_BASE_API ?? '/',
  timeout: 6e4,
  headers: { 'Content-Type': ContentTypeEnum.JSON },
})
const axiosLoading = new AxiosLoading()

// refresh/logout 不参与 401 静默刷新，避免认证流程递归。
const REFRESH_BYPASS_PATHS = ['/v1/auth/refresh', '/v1/auth/logout']

async function requestHandler(config) {
  if (import.meta.env.DEV && import.meta.env.VITE_APP_BASE_API_DEV && import.meta.env.VITE_APP_BASE_URL_DEV && config.customDev)
    config.baseURL = import.meta.env.VITE_APP_BASE_API_DEV

  const token = useAccessToken()
  if (token.value && config.token !== false)
    config.headers.set(STORAGE_AUTHORIZE_KEY, token.value)
  const { locale } = useI18nLocale()
  config.headers.set('Accept-Language', locale.value ?? 'zh-CN')
  if (config.loading)
    axiosLoading.addLoading()
  return config
}

function responseHandler(response) {
  return response.data
}

// 进入登录页前必须清空双 Token，避免无效 refresh 反复触发重试。
function gotoLogin(notify, data, statusText) {
  clearAuthTokens()
  const current = router.currentRoute.value.fullPath
  notify?.error({
    message: '401',
    description: data?.message || statusText,
    duration: 3,
  })
  router.push({
    path: '/login',
    query: current && current !== '/login' ? { redirect: current } : undefined,
  }).catch(() => {})
}

async function errorHandler(error) {
  const notification = useNotification()
  if (!error.response)
    return Promise.reject(error)

  const { data, status, statusText, config } = error.response
  const requestUrl = config?.url || ''
  const isBypass = REFRESH_BYPASS_PATHS.some(p => requestUrl.endsWith(p))

  // 仅对普通接口做一次静默刷新，避免同一请求无限重放。
  if (status === 401 && !isBypass && !config._retried) {
    const refresh = useRefreshToken()
    if (refresh.value) {
      config._retried = true
      try {
        const newAccess = await singleFlightRefresh()
        if (config.headers?.set)
          config.headers.set(STORAGE_AUTHORIZE_KEY, newAccess)
        else
          config.headers = { ...config.headers, [STORAGE_AUTHORIZE_KEY]: newAccess }
        return instance.request(config)
      }
      catch (refreshErr) {
        gotoLogin(notification, data, statusText)
        return Promise.reject(refreshErr)
      }
    }
    gotoLogin(notification, data, statusText)
  }
  else if (status === 401) {
    // 认证接口或重试后仍 401 时，当前会话不可恢复。
    gotoLogin(notification, data, statusText)
  }
  else if (status === 403) {
    notification?.error({
      message: '403',
      description: data?.message || statusText,
      duration: 3,
    })
  }
  else if (status === 500) {
    notification?.error({
      message: '500',
      description: data?.message || statusText,
      duration: 3,
    })
  }
  else {
    notification?.error({
      message: '服务错误',
      description: data?.message || statusText,
      duration: 3,
    })
  }
  return Promise.reject(error)
}
instance.interceptors.request.use(requestHandler)
instance.interceptors.response.use(responseHandler, errorHandler)
export default instance
function instancePromise(options) {
  const { loading } = options
  return new Promise((resolve, reject) => {
    instance.request(options).then((res) => {
      resolve(res)
    }).catch((e) => {
      reject(e)
    }).finally(() => {
      if (loading)
        axiosLoading.closeLoading()
    })
  })
}
export function useGet(url, params, config) {
  const options = {
    url,
    params,
    method: RequestEnum.GET,
    ...config,
  }
  return instancePromise(options)
}
export function usePost(url, data, config) {
  const options = {
    url,
    data,
    method: RequestEnum.POST,
    ...config,
  }
  return instancePromise(options)
}
export function usePut(url, data, config) {
  const options = {
    url,
    data,
    method: RequestEnum.PUT,
    ...config,
  }
  return instancePromise(options)
}
export function useDelete(url, data, config) {
  const options = {
    url,
    params: data,
    method: RequestEnum.DELETE,
    ...config,
  }
  return instancePromise(options)
}
