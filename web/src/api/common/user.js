export function getAdminUserInfoApi() {
  return useGet('/v1/admin/user')
}
export function getAdminUsersApi(params) {
  return useGet('/v1/admin/users',params)
}
export function createAdminUserApi(params) {
  return usePost('/v1/admin/user',params)
}
export function updateAdminUserApi(params) {
  return usePut('/v1/admin/user',params)
}
export function deleteAdminUserApi(params) {
  return useDelete('/v1/admin/user',params)
}

