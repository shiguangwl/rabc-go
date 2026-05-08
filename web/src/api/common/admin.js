export function getRolesApi(params) {
  return useGet('/v1/admin/roles',params)
}
export function createRoleApi(params) {
  return usePost('/v1/admin/role',params)
}
export function updateRoleApi(params) {
  return usePut('/v1/admin/role',params)
}
export function deleteRoleApi(params) {
  return useDelete('/v1/admin/role',params)
}
export function getUserPermissionsApi(params) {
  return useGet('/v1/admin/user/permissions',params)
}
export function getRolePermissionsApi(params) {
  return useGet('/v1/admin/role/permissions',params)
}
export function updateRolePermissionsApi(params) {
  return usePut('/v1/admin/role/permission',params)
}

export function getAdminApiApi(params) {
  return useGet('/v1/admin/apis',params)
}
export function createAdminApiApi(params) {
  return usePost('/v1/admin/api',params)
}
export function updateAdminApiApi(params) {
  return usePut('/v1/admin/api',params)
}
export function deleteAdminApiApi(params) {
  return useDelete('/v1/admin/api',params)
}