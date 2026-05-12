// access 放 sessionStorage，refresh 放 localStorage；静默刷新需要跨 Tab 共享 refresh。

const STORAGE_ACCESS_KEY = 'Authorization'
const STORAGE_REFRESH_KEY = 'RefreshToken'

// 第三个参数必须显式传 sessionStorage：@vueuse/core useStorage 默认走 localStorage。
export const useAccessToken = createGlobalState(() =>
  useStorage(STORAGE_ACCESS_KEY, null, sessionStorage),
)

export const useRefreshToken = createGlobalState(() =>
  useStorage(STORAGE_REFRESH_KEY, null, localStorage),
)

// 保留旧导出名，避免既有调用方误读 refresh token。
export const STORAGE_AUTHORIZE_KEY = STORAGE_ACCESS_KEY
export const useAuthorization = useAccessToken

export function clearAuthTokens() {
  useAccessToken().value = null
  useRefreshToken().value = null
}

export function setAuthTokens({ accessToken, refreshToken }) {
  if (accessToken !== undefined)
    useAccessToken().value = accessToken
  if (refreshToken !== undefined)
    useRefreshToken().value = refreshToken
}
