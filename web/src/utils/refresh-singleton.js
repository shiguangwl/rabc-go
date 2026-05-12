// refresh token 轮换必须跨 Tab 串行化，避免多个 Tab 同时使用同一个旧 refresh。
import axios from 'axios'
import { ContentTypeEnum } from '~#/http-enum'
import {
  clearAuthTokens,
  setAuthTokens,
  useAccessToken,
  useRefreshToken,
} from '~/composables/authorization'

let pending = null
const REFRESH_LOCK_KEY = 'AuthRefreshLock'
const REFRESH_LOCK_TTL = 8000
const REFRESH_WAIT_TIMEOUT = 10000
const REFRESH_WAIT_INTERVAL = 120
const tabID = `${Date.now()}-${Math.random().toString(36).slice(2)}`

// refresh 请求必须绕过统一拦截器，避免 401 处理递归。
const rawAxios = axios.create({
  baseURL: import.meta.env.VITE_APP_BASE_API ?? '/',
  timeout: 6e4,
  headers: { 'Content-Type': ContentTypeEnum.JSON },
})

async function doRefresh() {
  const refresh = useRefreshToken()
  const rt = refresh.value
  if (!rt)
    throw new Error('no refresh token')
  const resp = await rawAxios.post('/v1/auth/refresh', { refreshToken: rt })
  const payload = resp?.data?.data
  if (!payload?.accessToken || !payload?.refreshToken)
    throw new Error('invalid refresh response')
  setAuthTokens({
    accessToken: payload.accessToken,
    refreshToken: payload.refreshToken,
  })
  return payload.accessToken
}

function readLock() {
  try {
    return JSON.parse(localStorage.getItem(REFRESH_LOCK_KEY) || 'null')
  }
  catch (_) {
    return null
  }
}

function tryAcquireLock() {
  const now = Date.now()
  const lock = readLock()
  if (lock?.expiresAt && lock.expiresAt > now && lock.owner !== tabID)
    return false

  localStorage.setItem(REFRESH_LOCK_KEY, JSON.stringify({
    owner: tabID,
    expiresAt: now + REFRESH_LOCK_TTL,
  }))
  return readLock()?.owner === tabID
}

function releaseLock() {
  if (readLock()?.owner === tabID)
    localStorage.removeItem(REFRESH_LOCK_KEY)
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

async function waitForOtherTabRefresh(oldRT) {
  const start = Date.now()
  const access = useAccessToken()
  const refresh = useRefreshToken()
  while (Date.now() - start < REFRESH_WAIT_TIMEOUT) {
    if (refresh.value && refresh.value !== oldRT && access.value)
      return access.value
    if (!readLock())
      return null
    await sleep(REFRESH_WAIT_INTERVAL)
  }
  return null
}

async function crossTabRefresh() {
  const oldRT = useRefreshToken().value
  if (tryAcquireLock()) {
    try {
      return await doRefresh()
    }
    finally {
      releaseLock()
    }
  }

  const refreshedAccess = await waitForOtherTabRefresh(oldRT)
  if (refreshedAccess)
    return refreshedAccess

  // 锁失效后重试获取锁，覆盖持锁 Tab 崩溃或超时场景。
  return crossTabRefresh()
}

// singleFlightRefresh 合并同 Tab 并发刷新；失败时必须清空本地凭证。
export function singleFlightRefresh() {
  if (pending)
    return pending
  pending = crossTabRefresh()
    .catch((err) => {
      clearAuthTokens()
      throw err
    })
    .finally(() => {
      pending = null
    })
  return pending
}
