import { usePost } from '~/utils/request'

export function loginApi(params) {
  return usePost('/v1/login', params, {
    // 登录接口不能携带旧 token，避免过期凭证影响登录请求。
    token: false,
    customDev: false,
    loading: true,
  })
}

// logout 只靠 refreshToken 定位会话，不能自动注入 Authorization。
export function logoutApi(refreshToken) {
  return usePost('/v1/auth/logout', { refreshToken }, {
    token: false,
    loading: false,
  })
}
